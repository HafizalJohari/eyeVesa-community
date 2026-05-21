package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/auth"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/crypto"
)

type AirportHeartbeat struct {
	AgentID  string `json:"agent_id"`
	Status   string `json:"status"`
	Metadata []byte `json:"metadata,omitempty"`
}

type AirportProfileUpdate struct {
	Description     string          `json:"description"`
	ServicesOffered json.RawMessage `json:"services_offered"`
	Endpoints       json.RawMessage `json:"endpoints"`
	Tags            []string        `json:"tags"`
	Listed          *bool           `json:"listed"`
}

type AirportSearchRequest struct {
	Capability     string  `json:"capability"`
	Skill          string  `json:"skill"`
	MinTrust       float64 `json:"min_trust"`
	MinProficiency int     `json:"min_proficiency"`
	Verified       *bool   `json:"verified"`
	Status         string  `json:"status"`
	Tag            string  `json:"tag"`
	Owner          string  `json:"owner"`
	Limit          int     `json:"limit"`
	Offset         int     `json:"offset"`
}

type AirportConnection struct {
	ConnectionID     string  `json:"connection_id"`
	RequesterID      string  `json:"requester_id"`
	ResponderID      string  `json:"responder_id"`
	Action           string  `json:"action"`
	Outcome          string  `json:"outcome"`
	TrustScoreAtTime float64 `json:"trust_score_at_time"`
	CreatedAt        string  `json:"created_at"`
}

type AirportAgent struct {
	AgentID         string          `json:"agent_id"`
	Name            string          `json:"name"`
	Owner           string          `json:"owner"`
	TrustScore      float64         `json:"trust_score"`
	Status          string          `json:"status"`
	Description     string          `json:"description"`
	ServicesOffered json.RawMessage `json:"services_offered"`
	Endpoints       json.RawMessage `json:"endpoints"`
	Tags            []string        `json:"tags"`
	TotalActions    int             `json:"total_actions"`
	ApprovalRate    float64         `json:"approval_rate"`
	LastSeen        string          `json:"last_seen"`
}

func AirportHeartbeatHandler(w http.ResponseWriter, r *http.Request) {
	var hb AirportHeartbeat
	if err := json.NewDecoder(r.Body).Decode(&hb); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	agentID, err := uuid.Parse(hb.AgentID)
	if err != nil {
		http.Error(w, "invalid agent_id", http.StatusBadRequest)
		return
	}

	if !canAccessAirportAgent(r.Context(), agentID) {
		http.Error(w, "agent not found", http.StatusNotFound)
		return
	}

	validStatuses := map[string]bool{"online": true, "offline": true, "busy": true, "idle": true}
	if !validStatuses[hb.Status] {
		hb.Status = "online"
	}

	metadata := hb.Metadata
	if metadata == nil {
		metadata = json.RawMessage(`{}`)
	}

	_, err = querier.Exec(r.Context(), `
		INSERT INTO agent_heartbeats (agent_id, last_heartbeat, status, metadata, updated_at)
		VALUES ($1, NOW(), $2, $3, NOW())
		ON CONFLICT (agent_id) DO UPDATE SET
			last_heartbeat = NOW(),
			status = $2,
			metadata = $3,
			updated_at = NOW()
	`, agentID, hb.Status, metadata)

	if err != nil {
		slog.Error("airport heartbeat failed", "error", err)
		http.Error(w, "heartbeat failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agent_id": agentID.String(),
		"status":   hb.Status,
		"ok":       true,
	})
}

func AirportGetProfileHandler(w http.ResponseWriter, r *http.Request) {
	agentIDStr := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
	agentID, err := uuid.Parse(agentIDStr)
	if err != nil {
		http.Error(w, "invalid agent_id", http.StatusBadRequest)
		return
	}

	agent, err := airportGetAgent(r.Context(), agentID)
	if err != nil {
		http.Error(w, "agent not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agent)
}

func AirportUpdateProfileHandler(w http.ResponseWriter, r *http.Request) {
	agentIDStr := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
	agentID, err := uuid.Parse(agentIDStr)
	if err != nil {
		http.Error(w, "invalid agent_id", http.StatusBadRequest)
		return
	}

	if !canAccessAirportAgent(r.Context(), agentID) {
		http.Error(w, "agent not found", http.StatusNotFound)
		return
	}

	var update AirportProfileUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	services := update.ServicesOffered
	if services == nil {
		services = json.RawMessage(`[]`)
	}
	endpoints := update.Endpoints
	if endpoints == nil {
		endpoints = json.RawMessage(`{}`)
	}
	listed := true
	if update.Listed != nil {
		listed = *update.Listed
	}

	tags := update.Tags
	if tags == nil {
		tags = []string{}
	}

	_, err = querier.Exec(r.Context(), `
		INSERT INTO agent_profiles (agent_id, description, services_offered, endpoints, tags, listed, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (agent_id) DO UPDATE SET
			description = $2,
			services_offered = $3,
			endpoints = $4,
			tags = $5,
			listed = $6,
			updated_at = NOW()
	`, agentID, update.Description, services, endpoints, tags, listed)

	if err != nil {
		slog.Error("airport profile update failed", "error", err)
		http.Error(w, "profile update failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agent_id": agentID.String(),
		"listed":   listed,
		"ok":       true,
	})
}

func canAccessAirportAgent(ctx context.Context, agentID uuid.UUID) bool {
	tenantID := auth.GetTenantID(ctx)
	if tenantID == "" {
		return true
	}

	var exists int
	err := querier.QueryRow(ctx, `
		SELECT 1 FROM agents
		WHERE agent_id = $1
		AND (tenant_id::text = $2 OR owner = $2)
	`, agentID, tenantID).Scan(&exists)
	return err == nil && exists == 1
}

func AirportSearchHandler(w http.ResponseWriter, r *http.Request) {
	var req AirportSearchRequest
	req.Capability = r.URL.Query().Get("capability")
	req.Skill = r.URL.Query().Get("skill")
	req.Status = r.URL.Query().Get("status")
	req.Tag = r.URL.Query().Get("tag")
	req.Owner = r.URL.Query().Get("owner")

	if v := r.URL.Query().Get("min_trust"); v != "" {
		if f, err := parseFloat(v); err == nil {
			req.MinTrust = f
		}
	}
	if v := r.URL.Query().Get("min_proficiency"); v != "" {
		if i, err := parseInt(v); err == nil {
			req.MinProficiency = i
		}
	}
	if v := r.URL.Query().Get("verified"); v != "" {
		b := v == "true"
		req.Verified = &b
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if i, err := parseInt(v); err == nil && i > 0 {
			req.Limit = i
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if i, err := parseInt(v); err == nil && i >= 0 {
			req.Offset = i
		}
	}

	if req.Limit <= 0 {
		req.Limit = 50
	}
	if req.Limit > 200 {
		req.Limit = 200
	}

	args := []interface{}{}
	argIdx := 1
	conditions := []string{"listed = true"}
	joinSkills := false

	if req.MinTrust > 0 {
		conditions = append(conditions, "trust_score >= $1")
		args = append(args, req.MinTrust)
		argIdx++
	}

	if req.Owner != "" {
		conditions = append(conditions, "owner = $"+itoa(argIdx))
		args = append(args, req.Owner)
		argIdx++
	}

	if req.Status != "" {
		conditions = append(conditions, "status = $"+itoa(argIdx))
		args = append(args, req.Status)
		argIdx++
	}

	if req.Tag != "" {
		conditions = append(conditions, "$"+itoa(argIdx)+" = ANY(tags)")
		args = append(args, req.Tag)
		argIdx++
	}

	if req.Capability != "" {
		conditions = append(conditions, "$"+itoa(argIdx)+" = ANY(allowed_tools)")
		args = append(args, req.Capability)
		argIdx++
	}

	if req.Skill != "" {
		joinSkills = true
		conditions = append(conditions, "sk.name = $"+itoa(argIdx))
		args = append(args, req.Skill)
		argIdx++
		if req.MinProficiency > 0 {
			conditions = append(conditions, "askl.proficiency >= $"+itoa(argIdx))
			args = append(args, req.MinProficiency)
			argIdx++
		}
		if req.Verified != nil && *req.Verified {
			conditions = append(conditions, "askl.verified = true")
		}
	}

	where := strings.Join(conditions, " AND ")

	skillJoin := ""
	if joinSkills {
		skillJoin = " JOIN agent_skills askl ON askl.agent_id = a.agent_id JOIN skills sk ON sk.skill_id = askl.skill_id "
	}

	query := `SELECT a.agent_id, a.name, a.owner, a.trust_score,
		COALESCE(ap.description, '') as description,
		COALESCE(ap.services_offered, '[]'::jsonb) as services_offered,
		COALESCE(ap.endpoints, '{}'::jsonb) as endpoints,
		COALESCE(array_to_json(ap.tags)::text, '[]') as tags,
		COALESCE(ap.total_actions, 0) as total_actions,
		COALESCE(ap.approval_rate, 1.0) as approval_rate,
		COALESCE(ah.last_heartbeat::text, '') as last_seen,
		COALESCE(ah.status, 'offline') as status
		FROM agents a
		LEFT JOIN agent_profiles ap ON ap.agent_id = a.agent_id
		LEFT JOIN agent_heartbeats ah ON ah.agent_id = a.agent_id
	` + skillJoin +
		" WHERE " + strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(where, "listed", "ap.listed"), "trust_score", "a.trust_score"), "owner", "a.owner"), "status", "ah.status"), "allowed_tools", "a.allowed_tools"), "tags", "ap.tags") +
		" ORDER BY a.trust_score DESC " +
		" LIMIT $" + itoa(argIdx) + " OFFSET $" + itoa(argIdx+1)

	if !joinSkills {
		query = `WITH airport_agents AS (
			SELECT a.agent_id::text AS agent_id, a.name, a.owner, a.trust_score,
				COALESCE(ap.description, '') AS description,
				COALESCE(ap.services_offered, '[]'::jsonb) AS services_offered,
				COALESCE(ap.endpoints, '{}'::jsonb) AS endpoints,
				COALESCE(ap.tags, '{}') AS tags,
				COALESCE(ap.total_actions, 0) AS total_actions,
				COALESCE(ap.approval_rate, 1.0) AS approval_rate,
				COALESCE(ah.last_heartbeat::text, '') AS last_seen,
				COALESCE(ah.status, 'offline') AS status,
				a.allowed_tools,
				COALESCE(ap.listed, true) AS listed
			FROM agents a
			LEFT JOIN agent_profiles ap ON ap.agent_id = a.agent_id
			LEFT JOIN agent_heartbeats ah ON ah.agent_id = a.agent_id
			UNION ALL
			SELECT fa.agent_id::text AS agent_id, fa.name, fa.owner, fa.trust_score,
				COALESCE(fp.description, '') AS description,
				COALESCE(fp.services_offered, '[]'::jsonb) AS services_offered,
				COALESCE(fp.endpoints, '{}'::jsonb) AS endpoints,
				COALESCE(fp.tags, '{}') AS tags,
				0 AS total_actions,
				1.0 AS approval_rate,
				COALESCE(fh.last_heartbeat::text, '') AS last_seen,
				COALESCE(fh.status, 'offline') AS status,
				fa.allowed_tools,
				COALESCE(fp.listed, true) AS listed
			FROM federated_agents fa
			LEFT JOIN federated_profiles fp ON fp.agent_id = fa.agent_id
			LEFT JOIN federated_heartbeats fh ON fh.agent_id = fa.agent_id
			WHERE fa.status = 'active'
		)
		SELECT agent_id, name, owner, trust_score, description, services_offered, endpoints,
			COALESCE(array_to_json(tags)::text, '[]') AS tags,
			total_actions, approval_rate, last_seen, status
		FROM airport_agents
		WHERE ` + where +
			" ORDER BY trust_score DESC " +
			" LIMIT $" + itoa(argIdx) + " OFFSET $" + itoa(argIdx+1)
	}

	args = append(args, req.Limit, req.Offset)

	rows, err := querier.Query(r.Context(), query, args...)
	if err != nil {
		slog.Error("airport search failed", "error", err)
		http.Error(w, "search failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	agents := []AirportAgent{}
	for rows.Next() {
		var a AirportAgent
		var lastSeen, status, tagsArr *string
		var trustScore, approvalRate *float64
		var totalActions *int

		err := rows.Scan(
			&a.AgentID, &a.Name, &a.Owner, &trustScore,
			&a.Description, &a.ServicesOffered, &a.Endpoints,
			&tagsArr, &totalActions, &approvalRate,
			&lastSeen, &status,
		)
		if err != nil {
			continue
		}

		if trustScore != nil {
			a.TrustScore = *trustScore
		}
		if approvalRate != nil {
			a.ApprovalRate = *approvalRate
		}
		if totalActions != nil {
			a.TotalActions = *totalActions
		}
		if lastSeen != nil {
			a.LastSeen = *lastSeen
		}
		if status != nil {
			a.Status = *status
		}
		if tagsArr != nil {
			if err := json.Unmarshal([]byte(*tagsArr), &a.Tags); err != nil {
				tagsStr := strings.Trim(*tagsArr, "{}\"")
				if tagsStr != "" {
					a.Tags = strings.Split(tagsStr, ",")
				} else {
					a.Tags = []string{}
				}
			}
		}

		agents = append(agents, a)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agents": agents,
		"count":  len(agents),
		"limit":  req.Limit,
		"offset": req.Offset,
	})
}

func AirportListOnlineHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := querier.Query(r.Context(), `
		SELECT a.agent_id, a.name, a.owner, a.trust_score,
			COALESCE(ap.description, '') as description,
			COALESCE(ap.services_offered, '[]'::jsonb) as services_offered,
			COALESCE(ap.endpoints, '{}'::jsonb) as endpoints,
			COALESCE(array_to_json(ap.tags)::text, '[]') as tags,
			COALESCE(ap.total_actions, 0) as total_actions,
			COALESCE(ap.approval_rate, 1.0) as approval_rate,
			ah.last_heartbeat::text as last_seen,
			ah.status
		FROM agents a
		JOIN agent_heartbeats ah ON ah.agent_id = a.agent_id
		LEFT JOIN agent_profiles ap ON ap.agent_id = a.agent_id
		WHERE ah.status = 'online' AND ah.last_heartbeat > NOW() - INTERVAL '2 minutes'
		AND COALESCE(ap.listed, true) = true
		UNION ALL
		SELECT fa.agent_id, fa.name, fa.owner, fa.trust_score,
			COALESCE(fp.description, '') as description,
			COALESCE(fp.services_offered, '[]'::jsonb) as services_offered,
			COALESCE(fp.endpoints, '{}'::jsonb) as endpoints,
			COALESCE(array_to_json(fp.tags)::text, '[]') as tags,
			0 as total_actions,
			1.0 as approval_rate,
			fh.last_heartbeat::text as last_seen,
			fh.status
		FROM federated_agents fa
		JOIN federated_heartbeats fh ON fh.agent_id = fa.agent_id
		LEFT JOIN federated_profiles fp ON fp.agent_id = fa.agent_id
		WHERE fa.status = 'active'
		AND fh.status = 'online' AND fh.last_heartbeat > NOW() - INTERVAL '5 minutes'
		AND COALESCE(fp.listed, true) = true
		ORDER BY trust_score DESC
		LIMIT 100
	`)
	if err != nil {
		slog.Error("airport online list failed", "error", err)
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	agents := []AirportAgent{}
	for rows.Next() {
		var a AirportAgent
		var lastSeen, status, tagsArr *string
		var trustScore, approvalRate *float64
		var totalActions *int

		err := rows.Scan(
			&a.AgentID, &a.Name, &a.Owner, &trustScore,
			&a.Description, &a.ServicesOffered, &a.Endpoints,
			&tagsArr, &totalActions, &approvalRate,
			&lastSeen, &status,
		)
		if err != nil {
			continue
		}

		if trustScore != nil {
			a.TrustScore = *trustScore
		}
		if approvalRate != nil {
			a.ApprovalRate = *approvalRate
		}
		if totalActions != nil {
			a.TotalActions = *totalActions
		}
		if lastSeen != nil {
			a.LastSeen = *lastSeen
		}
		if status != nil {
			a.Status = *status
		}
		if tagsArr != nil {
			if err := json.Unmarshal([]byte(*tagsArr), &a.Tags); err != nil {
				tagsStr := strings.Trim(*tagsArr, "{}\"")
				if tagsStr != "" {
					a.Tags = strings.Split(tagsStr, ",")
				} else {
					a.Tags = []string{}
				}
			}
		}

		agents = append(agents, a)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agents": agents,
		"count":  len(agents),
	})
}

func AirportConnectionsHandler(w http.ResponseWriter, r *http.Request) {
	agentID := r.URL.Query().Get("agent_id")
	if agentID == "" {
		http.Error(w, "agent_id required", http.StatusBadRequest)
		return
	}

	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if i, err := parseInt(v); err == nil && i > 0 {
			limit = i
		}
	}

	rows, err := querier.Query(r.Context(), `
		SELECT connection_id, requester_id, responder_id, action, outcome, trust_score_at_time, created_at::text
		FROM airport_connections
		WHERE requester_id = $1 OR responder_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, agentID, limit)
	if err != nil {
		slog.Error("airport connections query failed", "error", err)
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	connections := []AirportConnection{}
	for rows.Next() {
		var c AirportConnection
		err := rows.Scan(&c.ConnectionID, &c.RequesterID, &c.ResponderID, &c.Action, &c.Outcome, &c.TrustScoreAtTime, &c.CreatedAt)
		if err != nil {
			continue
		}
		connections = append(connections, c)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"connections": connections,
		"count":       len(connections),
	})
}

func airportGetAgent(ctx context.Context, agentID uuid.UUID) (*AirportAgent, error) {
	var a AirportAgent
	var lastSeen, status, tagsArr *string
	var trustScore, approvalRate *float64
	var totalActions *int

	err := querier.QueryRow(ctx, `
		SELECT a.agent_id, a.name, a.owner, a.trust_score,
			COALESCE(ap.description, '') as description,
			COALESCE(ap.services_offered, '[]'::jsonb) as services_offered,
			COALESCE(ap.endpoints, '{}'::jsonb) as endpoints,
			COALESCE(array_to_json(ap.tags)::text, '[]') as tags,
			COALESCE(ap.total_actions, 0) as total_actions,
			COALESCE(ap.approval_rate, 1.0) as approval_rate,
			COALESCE(ah.last_heartbeat::text, '') as last_seen,
			COALESCE(ah.status, 'offline') as status
		FROM agents a
		LEFT JOIN agent_profiles ap ON ap.agent_id = a.agent_id
		LEFT JOIN agent_heartbeats ah ON ah.agent_id = a.agent_id
		WHERE a.agent_id = $1
	`, agentID).Scan(
		&a.AgentID, &a.Name, &a.Owner, &trustScore,
		&a.Description, &a.ServicesOffered, &a.Endpoints,
		&tagsArr, &totalActions, &approvalRate,
		&lastSeen, &status,
	)

	if err != nil {
		return nil, err
	}

	if trustScore != nil {
		a.TrustScore = *trustScore
	}
	if approvalRate != nil {
		a.ApprovalRate = *approvalRate
	}
	if totalActions != nil {
		a.TotalActions = *totalActions
	}
	if lastSeen != nil {
		a.LastSeen = *lastSeen
	}
	if status != nil {
		a.Status = *status
	}
	if tagsArr != nil {
		if err := json.Unmarshal([]byte(*tagsArr), &a.Tags); err != nil {
			tagsStr := strings.Trim(*tagsArr, "{}\"")
			if tagsStr != "" {
				a.Tags = strings.Split(tagsStr, ",")
			} else {
				a.Tags = []string{}
			}
		}
	}

	return &a, nil
}

func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

func parseInt(s string) (int, error) {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}

func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}

func logAirportConnection(ctx context.Context, requesterID, responderID, action, outcome string, trustScore float64) {
	connectionID := uuid.New()
	_, err := querier.Exec(ctx, `
		INSERT INTO airport_connections (connection_id, requester_id, responder_id, action, outcome, trust_score_at_time, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
	`, connectionID, requesterID, responderID, action, outcome, trustScore)
	if err != nil {
		slog.Error("airport connection log failed", "error", err)
	}
}

func autoCreateHeartbeat(ctx context.Context, agentID string) {
	_, err := querier.Exec(ctx, `
		INSERT INTO agent_heartbeats (agent_id, last_heartbeat, status, metadata, updated_at)
		VALUES ($1, NOW(), 'online', '{}', NOW())
		ON CONFLICT (agent_id) DO UPDATE SET
			last_heartbeat = NOW(),
			status = 'online',
			updated_at = NOW()
	`, agentID)
	if err != nil {
		slog.Error("auto heartbeat failed", "error", err)
	}
}

func autoCreateProfile(ctx context.Context, agentID, name, owner string) {
	_, err := querier.Exec(ctx, `
		INSERT INTO agent_profiles (agent_id, description, services_offered, endpoints, tags, listed, total_actions, approval_rate, updated_at)
		VALUES ($1, '', '[]'::jsonb, '{}'::jsonb, '{}', true, 0, 1.0, NOW())
		ON CONFLICT (agent_id) DO NOTHING
	`, agentID)
	if err != nil {
		slog.Error("auto profile creation failed", "error", err)
	}
}

func StartHeartbeatCleanup(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				result, err := querier.Exec(ctx, `
					UPDATE agent_heartbeats SET status = 'offline', updated_at = NOW()
					WHERE last_heartbeat < NOW() - INTERVAL '5 minutes' AND status != 'offline'
				`)
				if err != nil {
					slog.Error("heartbeat cleanup failed", "error", err)
				} else {
					rows := result.RowsAffected
					if rows > 0 {
						slog.Info("heartbeat cleanup", "marked_offline", rows)
					}
				}
			}
		}
	}()
}

// AirportHandshakeRequest represents an Ed25519-signed agent handshake
type AirportHandshakeRequest struct {
	AgentID   string `json:"agent_id"`
	PeerID    string `json:"peer_id"`
	Timestamp string `json:"timestamp"`
}

func AirportHandshakeHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Verify Ed25519 signature from headers
	agentID := r.Header.Get("X-Agent-Id")
	sigB64 := r.Header.Get("X-Signature")
	pubKeyB64 := r.Header.Get("X-Public-Key")

	if agentID == "" || sigB64 == "" {
		http.Error(w, "missing X-Agent-Id or X-Signature header", http.StatusBadRequest)
		return
	}

	// If public key provided in header, verify directly; otherwise fetch from DB
	var pubKeyBytes []byte
	if pubKeyB64 != "" {
		pubKeyBytes, err = crypto.DecodeBase64(pubKeyB64)
		if err != nil {
			http.Error(w, "invalid public key encoding", http.StatusBadRequest)
			return
		}
	} else {
		err = querier.QueryRow(r.Context(),
			`SELECT public_key FROM agents WHERE agent_id = $1`, agentID,
		).Scan(&pubKeyBytes)
		if err != nil {
			http.Error(w, "agent not found", http.StatusNotFound)
			return
		}
	}

	sig, err := crypto.DecodeBase64(sigB64)
	if err != nil {
		http.Error(w, "invalid signature encoding", http.StatusBadRequest)
		return
	}

	if !crypto.VerifySignature(pubKeyBytes, body, sig) {
		logAirportConnection(r.Context(), agentID, "", "handshake", "denied", 0)
		slog.Warn("handshake signature invalid", "agent", agentID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":       false,
			"error":    "signature verification failed",
			"agent_id": agentID,
		})
		return
	}

	// Parse handshake payload to extract peer
	var hb AirportHandshakeRequest
	if err := json.Unmarshal(body, &hb); err == nil && hb.PeerID != "" {
		autoCreateHeartbeat(r.Context(), agentID)

		// Look up peer details
		var peerName, peerOwner string
		var peerTrust float64
		querier.QueryRow(r.Context(),
			`SELECT name, owner, trust_score FROM agents WHERE agent_id = $1`, hb.PeerID,
		).Scan(&peerName, &peerOwner, &peerTrust)

		logAirportConnection(r.Context(), agentID, hb.PeerID, "handshake", "success", peerTrust)

		slog.Info("agent handshake", "agent", agentID, "peer", hb.PeerID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":        true,
			"agent_id":  agentID,
			"handshake": "completed",
			"peer": map[string]interface{}{
				"agent_id":    hb.PeerID,
				"name":        peerName,
				"owner":       peerOwner,
				"trust_score": peerTrust,
			},
			"timestamp": hb.Timestamp,
		})
		return
	}

	// No peer specified — just verify identity, log, heartbeat
	autoCreateHeartbeat(r.Context(), agentID)
	logAirportConnection(r.Context(), agentID, "", "handshake", "success", 0)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":        true,
		"agent_id":  agentID,
		"handshake": "verified",
	})
}

// AirportConnectRequest is the body for POST /v1/airport/connect
type AirportConnectRequest struct {
	AgentID string `json:"agent_id"`
	PeerID  string `json:"peer_id"`
	Action  string `json:"action"`
}

func AirportConnectHandler(w http.ResponseWriter, r *http.Request) {
	var req AirportConnectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.AgentID == "" || req.PeerID == "" {
		http.Error(w, "agent_id and peer_id are required", http.StatusBadRequest)
		return
	}

	if req.Action == "" {
		req.Action = "connect"
	}

	// Look up trust score for logging
	var trustScore float64
	err := querier.QueryRow(r.Context(),
		`SELECT trust_score FROM agents WHERE agent_id = $1`, req.AgentID,
	).Scan(&trustScore)
	if err != nil {
		trustScore = 0
	}

	// Verify peer exists in either local or federated airport tables.
	var peerName string
	peerErr := querier.QueryRow(r.Context(),
		`SELECT name FROM agents WHERE agent_id = $1`, req.PeerID,
	).Scan(&peerName)
	peerIsLocal := peerErr == nil
	if peerErr != nil {
		peerErr = querier.QueryRow(r.Context(),
			`SELECT name FROM federated_agents WHERE agent_id = $1`, req.PeerID,
		).Scan(&peerName)
	}

	outcome := "success"
	if peerErr != nil {
		outcome = "error"
	}

	logAirportConnection(r.Context(), req.AgentID, req.PeerID, req.Action, outcome, trustScore)

	// Heartbeat both sides
	autoCreateHeartbeat(r.Context(), req.AgentID)
	// Keep heartbeat writes scoped to local agents table.
	if peerErr == nil && peerIsLocal {
		autoCreateHeartbeat(r.Context(), req.PeerID)
	}

	slog.Info("agent connection", "agent", req.AgentID, "peer", req.PeerID, "action", req.Action)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":          outcome == "success",
		"connection":  req.Action,
		"agent_id":    req.AgentID,
		"peer_id":     req.PeerID,
		"peer_exists": peerErr == nil,
		"trust_score": trustScore,
	})
}

func AirportHealthHandler(w http.ResponseWriter, r *http.Request) {
	onlineCount := 0
	querier.QueryRow(r.Context(), `
		SELECT COUNT(*) FROM agent_heartbeats WHERE status = 'online' AND last_heartbeat > NOW() - INTERVAL '2 minutes'
	`).Scan(&onlineCount)

	totalProfiles := 0
	querier.QueryRow(r.Context(), `SELECT COUNT(*) FROM agent_profiles`).Scan(&totalProfiles)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         "healthy",
		"online_agents":  onlineCount,
		"total_profiles": totalProfiles,
	})
}

func AirportStatsHandler(w http.ResponseWriter, r *http.Request) {
	totalAgents := 0
	onlineAgents := 0
	federationAgents := 0
	recentActivity := 0
	gatewayCount := 0

	querier.QueryRow(r.Context(), `SELECT COUNT(*) FROM agents`).Scan(&totalAgents)
	querier.QueryRow(r.Context(), `
		SELECT COUNT(*) FROM agent_heartbeats
		WHERE status = 'online' AND last_heartbeat > NOW() - INTERVAL '2 minutes'
	`).Scan(&onlineAgents)
	querier.QueryRow(r.Context(), `SELECT COUNT(*) FROM federated_agents WHERE status = 'active'`).Scan(&federationAgents)

	federatedOnline := 0
	querier.QueryRow(r.Context(), `
		SELECT COUNT(*) FROM federated_heartbeats
		WHERE status = 'online' AND last_heartbeat > NOW() - INTERVAL '5 minutes'
	`).Scan(&federatedOnline)
	onlineAgents += federatedOnline

	querier.QueryRow(r.Context(), `
		SELECT COUNT(*) FROM airport_connections WHERE created_at > NOW() - INTERVAL '1 hour'
	`).Scan(&recentActivity)
	querier.QueryRow(r.Context(), `SELECT COUNT(*) FROM federation_peers WHERE status = 'active'`).Scan(&gatewayCount)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "healthy",
		"agents": map[string]int{
			"total":     totalAgents + federationAgents,
			"local":     totalAgents,
			"federated": federationAgents,
			"online":    onlineAgents,
		},
		"recent_activity_1h": recentActivity,
		"gateway": map[string]interface{}{
			"health":       "healthy",
			"active_peers": gatewayCount,
		},
	})
}
