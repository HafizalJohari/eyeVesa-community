package main

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "github.com/hafizaljohari/eyeVesa/proto/agentid"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/cmd/api/handlers"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/audit"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/auth"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/behavior"
	gwcrypto "github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/crypto"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/database"
	grpcserver "github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/grpcserver"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/delegation"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/health"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/hitl"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/identity"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/license"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/llm"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/metrics"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/migrate"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/ptv"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/policy"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/ratelimit"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/tenant"
)

func main() {
	var draining atomic.Bool

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := database.Connect(ctx)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	slog.Info("connected to database")

	licInfo := license.Load()
	slog.Info("license", "tier", licInfo.Tier, "max_agents", licInfo.MaxAgents, "max_resources", licInfo.MaxResources)

	migrationsDir := os.Getenv("MIGRATIONS_DIR")
	if migrationsDir == "" {
		exePath, _ := os.Executable()
		migrationsDir = filepath.Join(filepath.Dir(exePath), "..", "registry", "migrations")
		if _, err := os.Stat(migrationsDir); err != nil {
			migrationsDir = "registry/migrations"
		}
	}
	if err := migrate.RunMigrations(ctx, db.Pool, migrationsDir); err != nil {
		slog.Error("failed to run migrations", "dir", migrationsDir, "error", err)
		os.Exit(1)
	}

	var pubKey ed25519.PublicKey
	var privKey ed25519.PrivateKey

	keyPath := os.Getenv("GATEWAY_KEY_PATH")
	if keyPath == "" {
		keyPath = "/tmp/agentid-gateway-ed25519.key"
	}

	pubKey, privKey, err = gwcrypto.LoadOrGenerateKeys(keyPath)
	if err != nil {
		slog.Error("failed to load/generate gateway keys", "error", err)
		os.Exit(1)
	}
	slog.Info("gateway key loaded", "public_key", fmt.Sprintf("%x", pubKey))

	auditLogger := audit.NewAuditLogger(db)

	identityProvider := identity.NewIdentityProvider()

	svid, err := identityProvider.FetchSVID(ctx)
	if err != nil {
		slog.Warn("could not fetch SVID", "error", err)
	} else {
		slog.Info("gateway identity", "spiffe_id", svid.SpiffeID, "trust_domain", svid.TrustDomain)
	}

	if err := identityProvider.WriteCerts("/tmp/agentid-gateway.crt", "/tmp/agentid-gateway.key"); err != nil {
		slog.Warn("could not write certs", "error", err)
	}

	delegationTracker := delegation.NewDelegationTracker(db, identityProvider)
	ptvService := ptv.NewPTVService(db.Pool)
	hitlService := hitl.NewHITLService(db.Pool)
	escalationService := hitl.NewEscalationService(db.Pool)
	llmService := llm.NewLLMService(nil)
	embeddingService := behavior.NewEmbeddingService(db.Pool, llmService)
	tenantService := tenant.NewTenantService(db)
	pushService := hitl.NewPushService(db.Pool)
	spireService := identity.NewSpireService(db.Pool)

	webhookNotifier := hitl.NewWebhookNotifier()
	escalationService.RegisterNotifier(hitl.ChannelWebhook, webhookNotifier)

	slackWebhook := os.Getenv("SLACK_WEBHOOK_URL")
	if slackWebhook != "" {
		slackNotifier := hitl.NewSlackNotifier(slackWebhook)
		escalationService.RegisterNotifier(hitl.ChannelSlack, slackNotifier)
	}

	pagerdutyKey := os.Getenv("PAGERDUTY_INTEGRATION_KEY")
	if pagerdutyKey != "" {
		pdNotifier := hitl.NewPagerDutyNotifier(pagerdutyKey)
		escalationService.RegisterNotifier(hitl.ChannelPagerduty, pdNotifier)
	}

	telegramBotToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	telegramChatID := os.Getenv("TELEGRAM_CHAT_ID")
	if telegramBotToken != "" {
		telegramNotifier := hitl.NewTelegramNotifier(telegramBotToken, telegramChatID)
		escalationService.RegisterNotifier(hitl.ChannelTelegram, telegramNotifier)
		slog.Info("Telegram notifier enabled")
	}

	discordWebhook := os.Getenv("DISCORD_WEBHOOK_URL")
	if discordWebhook != "" {
		discordNotifier := hitl.NewDiscordNotifier(discordWebhook)
		escalationService.RegisterNotifier(hitl.ChannelDiscord, discordNotifier)
		slog.Info("Discord notifier enabled")
	}

	pushNotifier := hitl.NewPushNotifier()
	escalationService.RegisterNotifier("push", pushNotifier)

	authEnabled := os.Getenv("AUTH_ENABLED") != "false"
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = string(auth.GenerateJWTSecret())
	}

	var authMiddleware *auth.AuthMiddleware
	if authEnabled {
		authMiddleware = auth.NewAuthMiddleware(db.Pool, jwtSecret)
		slog.Info("authentication middleware enabled")
	}

	go func() {
		if sp, ok := identityProvider.(*identity.SpireProvider); ok {
			slog.Info("starting SPIRE SVID watcher for cert rotation")
			ch, err := sp.WatchX509SVID(ctx)
			if err != nil {
				slog.Warn("SPIRE watch failed", "error", err)
				return
			}
			for svid := range ch {
				slog.Info("SVID updated", "spiffe_id", svid.SpiffeID, "expires_at", svid.ExpiresAt.Format(time.RFC3339))
				if err := identityProvider.WriteCerts("/tmp/agentid-gateway.crt", "/tmp/agentid-gateway.key"); err != nil {
					slog.Warn("cert rotation write failed", "error", err)
				} else {
					slog.Info("rotated gateway certificates from SPIRE SVID update")
				}
			}
			slog.Info("SPIRE SVID watcher stopped")
		}
	}()

	go escalationService.RunEscalationTicker(ctx)

	bundleRefreshInterval := 5 * time.Minute
	if v := os.Getenv("SPIRE_BUNDLE_REFRESH_SECS"); v != "" {
		if secs, err := strconv.ParseInt(v, 10, 64); err == nil && secs > 0 {
			bundleRefreshInterval = time.Duration(secs) * time.Second
		}
	}
	go spireService.RunBundleRefresh(ctx, bundleRefreshInterval)

	opaEndpoint := os.Getenv("OPA_ENDPOINT")
	policyDir := os.Getenv("POLICY_DIR")
	if policyDir == "" {
		exePath, _ := os.Executable()
		policyDir = filepath.Join(filepath.Dir(exePath), "policies")
		if _, err := os.Stat(policyDir); err != nil {
			policyDir = "policies"
		}
	}
	policyEngine := policy.NewPolicyEngine(policyDir, opaEndpoint)

	handlers.SetDB(db)
	handlers.SetAuditLogger(auditLogger)
	handlers.SetGatewayKeys(privKey)
	handlers.SetDelegationTracker(delegationTracker)
	handlers.SetPTVService(ptvService)
	handlers.SetHITLService(hitlService)
	handlers.SetPolicyEngine(policyEngine)
	handlers.SetEscalationService(escalationService)
	handlers.SetLLMService(llmService)
	handlers.SetEmbeddingService(embeddingService)
	handlers.SetTenantService(tenantService)
	handlers.SetPushService(pushService)
	handlers.SetSpireService(spireService)
	handlers.SetIdentityProvider(identityProvider)

	grpcSrv := grpcserver.NewGatewayServer(db, auditLogger, privKey, policyEngine)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(metrics.Middleware)
	r.Use(license.Middleware)

	if authEnabled && authMiddleware != nil {
		r.Use(authMiddleware.Middleware)
	}

	globalRPS := 100.0
	if v := os.Getenv("RATE_LIMIT_RPS"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			globalRPS = f
		}
	}
	rateLimiter := ratelimit.NewRateLimiter(globalRPS*10, globalRPS)
	r.Use(rateLimiter.Middleware)

	healthChecker := health.NewChecker(db, policyEngine, &draining)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		report := healthChecker.Check(r.Context())
		statusCode := http.StatusOK
		if report.Status == health.StatusUnhealthy {
			statusCode = http.StatusServiceUnavailable
		} else if report.Status == health.StatusDegraded {
			statusCode = http.StatusServiceUnavailable
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(report)
	})

	r.Handle("/metrics", metrics.Handler())

	r.Get("/identity", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"spiffe_id":   svid.SpiffeID,
			"trust_domain": svid.TrustDomain,
			"expires_at":  svid.ExpiresAt.Format(time.RFC3339),
		})
	})

	r.Route("/v1", func(r chi.Router) {
		r.Post("/agents/register", handlers.RegisterAgent)
		r.Get("/agents", handlers.ListAgents)
		r.Get("/agents/{agentID}", handlers.GetAgent)

		r.Post("/resources/register", handlers.RegisterResource)
		r.Get("/resources", handlers.ListResources)
		r.Get("/resources/{resourceID}", handlers.GetResource)

		r.Post("/mcp", handlers.HandleMCP)

		r.Post("/authorize", handlers.Authorize)
		r.Post("/verify-signature", handlers.VerifySignature)

		r.Post("/delegate", handlers.DelegateAgent)
		r.Get("/delegations/{agentID}", handlers.GetDelegationChain)
		r.Get("/delegations/validate", handlers.ValidateDelegation)
		r.Delete("/delegations/{delegationID}", handlers.RevokeDelegation)

		r.Post("/hitl/request", handlers.RequestApproval)
		r.Get("/hitl/pending", handlers.ListPendingApprovals)
		r.Get("/hitl/{approvalID}", handlers.GetApprovalStatus)
		r.Post("/hitl/{approvalID}/decide", handlers.DecideApproval)

		// Phase 3: Multi-layer HITL escalation
		r.Post("/hitl/escalate", license.Require(license.FeatureMultiLayerHITL, handlers.RequestEscalatedApproval))
		r.Post("/hitl/{approvalID}/chain", license.Require(license.FeatureMultiLayerHITL, handlers.ProcessChainDecision))
		r.Get("/hitl/{approvalID}/chain", license.Require(license.FeatureMultiLayerHITL, handlers.GetApprovalChain))
		r.Get("/hitl/{approvalID}/notifications", license.Require(license.FeatureMultiLayerHITL, handlers.GetNotifications))

		// Phase 3: LLM integration
		r.Post("/llm/hitl-summary/{approvalID}", license.Require(license.FeatureLLM, handlers.GenerateHITLSummary))
		r.Post("/llm/audit-narrative", license.Require(license.FeatureLLM, handlers.GenerateAuditNarrative))
		r.Post("/llm/policy-translate", license.Require(license.FeatureLLM, handlers.TranslatePolicy))

		// Phase 3: Behavioral embeddings
		r.Post("/behavior/{agentID}/embedding", license.Require(license.FeatureAnomalyDetect, handlers.UpdateBehaviorEmbedding))
		r.Get("/behavior/{agentID}/anomalies", license.Require(license.FeatureAnomalyDetect, handlers.DetectBehavioralAnomalies))
		r.Get("/behavior/{agentID}/similar", license.Require(license.FeatureAnomalyDetect, handlers.GetSimilarAgents))

		// Phase 3: Multi-tenant
		r.Post("/tenants", license.Require(license.FeatureMultiTenant, handlers.CreateTenant))
		r.Get("/tenants", license.Require(license.FeatureMultiTenant, handlers.ListTenants))
		r.Get("/tenants/{tenantID}", license.Require(license.FeatureMultiTenant, handlers.GetTenant))

		// Phase 3: Budget metering
		r.Get("/budget/check", license.Require(license.FeatureBudget, handlers.CheckBudget))
		r.Post("/budget/spend", license.Require(license.FeatureBudget, handlers.RecordSpend))

		// Phase 3: Push notification tokens
		r.Post("/push/register", license.Require(license.FeaturePushNotify, handlers.RegisterPushToken))
		r.Get("/push/tokens", license.Require(license.FeaturePushNotify, handlers.GetPushTokens))
		r.Delete("/push/tokens/{tokenID}", license.Require(license.FeaturePushNotify, handlers.DeactivatePushToken))
		
		// Phase 3: Audit log retrieval
		r.Get("/audit", handlers.GetAuditLog)

		r.Post("/ptv/attest", handlers.AttestIdentity)
		r.Post("/ptv/bind", handlers.BindIdentity)
		r.Get("/ptv/verify/{bindingID}", handlers.VerifyIdentity)

		// Phase 5: SPIRE trust bundles & federation
		r.Post("/spire/bundles", handlers.CreateTrustBundle)
		r.Get("/spire/bundles", handlers.ListTrustBundles)
		r.Get("/spire/bundles/{trustDomain}", handlers.GetTrustBundle)
		r.Put("/spire/bundles/{trustDomain}", handlers.UpdateTrustBundle)
		r.Post("/spire/bundles/{trustDomain}/verify", handlers.VerifyTrustBundle)
		r.Delete("/spire/bundles/{trustDomain}", handlers.DeleteTrustBundle)
		r.Post("/spire/bundles/fetch", handlers.FetchBundleFromEndpoint)

		// Phase 5: SPIRE workload registrations
		r.Post("/spire/workloads", handlers.RegisterWorkload)
		r.Get("/spire/workloads", handlers.ListWorkloads)
		r.Get("/spire/workloads/{spiffeID}", handlers.GetWorkload)
		r.Post("/spire/workloads/{spiffeID}/attest", handlers.AttestWorkload)
		r.Delete("/spire/workloads/{spiffeID}", handlers.DeleteWorkload)

		// Phase 5: SPIRE status
		r.Get("/spire/status", handlers.GetSpireStatus)

		// Phase 6: Skills
		r.Post("/skills", handlers.CreateSkill)
		r.Get("/skills", handlers.ListSkills)
		r.Get("/skills/search", handlers.SearchSkills)
		r.Get("/skills/{skillID}", handlers.GetSkill)
		r.Put("/skills/{skillID}", handlers.UpdateSkill)
		r.Delete("/skills/{skillID}", handlers.DeleteSkill)

		r.Post("/agents/{agentID}/skills", handlers.AssignSkill)
		r.Get("/agents/{agentID}/skills", handlers.ListAgentSkills)
		r.Delete("/agents/{agentID}/skills/{skillID}", handlers.RemoveSkill)
		r.Post("/agents/{agentID}/skills/{skillID}/verify", handlers.VerifySkill)
		r.Post("/agents/{agentID}/skills/{skillID}/endorse", handlers.EndorseSkill)
		r.Get("/agents/{agentID}/skills/{skillID}/endorsements", handlers.ListEndorsements)

		r.Get("/agents/{agentID}/skill-trust", handlers.GetSkillTrust)
		r.Post("/agents/{agentID}/skill-trust/{skillID}", handlers.AdjustSkillTrust)

		r.Post("/agents/{agentID}/skill-authz", handlers.CheckSkillAuthz)
		r.Post("/agents/{agentID}/missing-skills", handlers.FindMissingSkills)
	})

	var httpSrv *http.Server
	go func() {
		httpAddr := os.Getenv("HTTP_ADDR")
		if httpAddr == "" {
			httpAddr = ":8080"
		}

		backendTLSCert := os.Getenv("BACKEND_TLS_CERT_PATH")
		backendTLSKey := os.Getenv("BACKEND_TLS_KEY_PATH")

		if backendTLSCert != "" && backendTLSKey != "" {
			cfg := &tls.Config{
				MinVersion: tls.VersionTLS12,
			}
			httpSrv = &http.Server{
				Addr:         httpAddr,
				Handler:      r,
				TLSConfig:    cfg,
				ReadTimeout:  30 * time.Second,
				WriteTimeout: 30 * time.Second,
			}
			slog.Info("HTTPS server starting", "addr", httpAddr)
			if err := httpSrv.ListenAndServeTLS(backendTLSCert, backendTLSKey); err != nil && err != http.ErrServerClosed {
				slog.Error("HTTPS server failed", "error", err)
				os.Exit(1)
			}
		} else {
			httpSrv = &http.Server{
				Addr:         httpAddr,
				Handler:      r,
				ReadTimeout:  30 * time.Second,
				WriteTimeout: 30 * time.Second,
			}
			slog.Info("HTTP server starting", "addr", httpAddr)
			if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("HTTP server failed", "error", err)
				os.Exit(1)
			}
		}
	}()

	grpcAddr := os.Getenv("GRPC_ADDR")
	if grpcAddr == "" {
		grpcAddr = ":9090"
	}
	grpcListener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		slog.Error("failed to listen for gRPC", "addr", grpcAddr, "error", err)
		os.Exit(1)
	}

	var grpcServer *grpc.Server
	backendGRPCTLSCert := os.Getenv("BACKEND_GRPC_TLS_CERT_PATH")
	backendGRPCTLSKey := os.Getenv("BACKEND_GRPC_TLS_KEY_PATH")

	if backendGRPCTLSCert != "" && backendGRPCTLSKey != "" {
		creds, err := credentials.NewServerTLSFromFile(backendGRPCTLSCert, backendGRPCTLSKey)
		if err != nil {
			slog.Error("failed to load gRPC TLS credentials", "error", err)
			os.Exit(1)
		}
		grpcServer = grpc.NewServer(grpc.Creds(creds))
		slog.Info("gRPC server starting with TLS", "addr", grpcAddr)
	} else {
		grpcServer = grpc.NewServer()
		slog.Info("gRPC server starting (plaintext)", "addr", grpcAddr)
	}
	pb.RegisterGatewayServiceServer(grpcServer, grpcSrv)

	go func() {
		if err := grpcServer.Serve(grpcListener); err != nil {
			slog.Error("gRPC server failed", "error", err)
			os.Exit(1)
		}
	}()

	r.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		if draining.Load() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "draining"})
			return
		}

		report := healthChecker.Check(r.Context())
		if report.Status != health.StatusHealthy {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(report)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	})

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)

	for {
		select {
		case <-quit:
			draining.Store(true)
			slog.Info("shutting down servers...")

			cancel()

			grpcServer.GracefulStop()

			if httpSrv != nil {
				shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer shutdownCancel()
				if err := httpSrv.Shutdown(shutdownCtx); err != nil {
					slog.Error("HTTP server shutdown error", "error", err)
				}
			}

			slog.Info("shutdown complete")
			return

		case <-sighup:
			slog.Info("received SIGHUP, reloading configuration...")

			if newRPS := os.Getenv("RATE_LIMIT_RPS"); newRPS != "" {
				if f, err := strconv.ParseFloat(newRPS, 64); err == nil {
					rateLimiter.Reload(f*10, f)
					slog.Info("rate limit RPS reloaded", "rps", f)
				}
			}

			reloadPolicyDir := os.Getenv("POLICY_DIR")
			if reloadPolicyDir == "" {
				reloadPolicyDir = policyDir
			}
			if reloadPolicyDir != "" {
				if _, err := os.Stat(reloadPolicyDir); err == nil {
					if reloadErr := policyEngine.Reload(reloadPolicyDir); reloadErr != nil {
						slog.Error("policy reload failed", "error", reloadErr)
					} else {
						slog.Info("policy reloaded", "path", reloadPolicyDir)
					}
				}
			}

			slog.Info("configuration reloaded")
		}
	}
}