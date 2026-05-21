package health

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/database"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/policy"
)

type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusDegraded  Status = "degraded"
	StatusUnhealthy Status = "unhealthy"
)

type ComponentStatus struct {
	Name    string `json:"name"`
	Status  Status `json:"status"`
	Latency string `json:"latency,omitempty"`
	Error   string `json:"error,omitempty"`
}

type HealthReport struct {
	Status     Status            `json:"status"`
	Components []ComponentStatus `json:"components"`
	Timestamp  string            `json:"timestamp"`
}

type Checker struct {
	db            *database.DB
	policyEngine  *policy.PolicyEngine
	draining      *atomic.Bool
	checkTimeout  time.Duration
}

func NewChecker(db *database.DB, policyEngine *policy.PolicyEngine, draining *atomic.Bool) *Checker {
	return &Checker{
		db:           db,
		policyEngine: policyEngine,
		draining:     draining,
		checkTimeout: 5 * time.Second,
	}
}

func (c *Checker) Check(ctx context.Context) HealthReport {
	report := HealthReport{
		Timestamp: time.Now().Format(time.RFC3339),
	}

	if c.draining != nil && c.draining.Load() {
		report.Status = StatusUnhealthy
		report.Components = append(report.Components, ComponentStatus{
			Name:   "server",
			Status: StatusUnhealthy,
			Error:  "draining connections",
		})
		return report
	}

	allHealthy := true

	dbStatus := c.checkDB(ctx)
	report.Components = append(report.Components, dbStatus)
	if dbStatus.Status != StatusHealthy {
		allHealthy = false
	}

	opaStatus := c.checkOPA(ctx)
	report.Components = append(report.Components, opaStatus)
	if opaStatus.Status != StatusHealthy {
		allHealthy = false
	}

	if allHealthy {
		report.Status = StatusHealthy
	} else {
		report.Status = StatusDegraded
	}

	return report
}

func (c *Checker) checkDB(ctx context.Context) ComponentStatus {
	status := ComponentStatus{Name: "postgresql"}

	if c.db == nil || c.db.Pool == nil {
		status.Status = StatusUnhealthy
		status.Error = "database not configured"
		return status
	}

	checkCtx, cancel := context.WithTimeout(ctx, c.checkTimeout)
	defer cancel()

	start := time.Now()
	err := c.db.Pool.Ping(checkCtx)
	latency := time.Since(start)

	if err != nil {
		status.Status = StatusUnhealthy
		status.Error = err.Error()
		slog.Error("health check: database unhealthy", "error", err)
		return status
	}

	status.Status = StatusHealthy
	status.Latency = latency.Round(time.Millisecond).String()
	return status
}

func (c *Checker) checkOPA(ctx context.Context) ComponentStatus {
	status := ComponentStatus{Name: "opa_policy"}

	checkCtx, cancel := context.WithTimeout(ctx, c.checkTimeout)
	defer cancel()

	start := time.Now()

	testInput := policy.PolicyInput{}
	testInput.Agent.ID = "health-check"
	testInput.Agent.TrustScore = 1.0
	testInput.Agent.AllowedTools = []string{"health"}
	testInput.Action.Tool = "health"

	decision := c.policyEngine.Evaluate(checkCtx, testInput)
	latency := time.Since(start)

	if decision == nil {
		status.Status = StatusUnhealthy
		status.Error = "policy engine returned nil"
		slog.Error("health check: OPA policy engine unhealthy", "error", "nil decision")
		return status
	}

	status.Status = StatusHealthy
	status.Latency = latency.Round(time.Millisecond).String()
	return status
}