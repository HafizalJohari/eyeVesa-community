package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
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
	ConnectionID   string  `json:"connection_id"`
	RequesterID    string  `json:"requester_id"`
	ResponderID    string  `json:"responder_id"`
	Action         string  `json:"action"`
	Outcome        string  `json:"outcome"`
	TrustScoreAtTime float64 `json:"trust_score_at_time"`
	CreatedAt      string  `json:"created_at"`
}

type AirportAgent struct {
	AgentID      string  `json:"agent_id"`
	Name         string  `json:"name"`
	Owner        string  `json:"owner"`
	TrustScore   float64 `json:"trust_score"`
	Status       string  `json:"status"`
	Description  string  `json:"description"`
	ServicesOffered json.RawMessage `json:"services_offered"`
	Endpoints    json.RawMessage `json:"endpoints"`
	Tags         []string `json:"tags"`
	TotalActions int     `json:"total_actions"`
	ApprovalRate float64 `json:"approval_rate"`
	LastSeen     string  `json:"last_seen"`
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
		"ok":      true,
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
		"ok":      true,
	})
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
	conditions := []string{"ap.listed = true"}
	joinSkills := false

	if req.MinTrust > 0 {
		conditions = append(conditions, "a.trust_score >= $1")
		args = append(args, req.MinTrust)
		argIdx++
	}

	if req.Owner != "" {
		conditions = append(conditions, "a.owner = $"+itoa(argIdx))
		args = append(args, req.Owner)
		argIdx++
	}

	if req.Status != "" {
		conditions = append(conditions, "ah.status = $"+itoa(argIdx))
		args = append(args, req.Status)
		argIdx++
	}

	if req.Tag != "" {
		conditions = append(conditions, "$"+itoa(argIdx)+" = ANY(ap.tags)")
		args = append(args, req.Tag)
		argIdx++
	}

	if req.Capability != "" {
		conditions = append(conditions, "$"+itoa(argIdx)+" = ANY(a.allowed_tools)")
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
		COALESCE(ap.tags, '{}') as tags,
		COALESCE(ap.total_actions, 0) as total_actions,
		COALESCE(ap.approval_rate, 1.0) as approval_rate,
		COALESCE(ah.last_heartbeat::text, '') as last_seen,
		COALESCE(ah.status, 'offline') as status
		FROM agents a
		LEFT JOIN agent_profiles ap ON ap.agent_id = a.agent_id
		LEFT JOIN agent_heartbeats ah ON ah.agent_id = a.agent_id
	` + skillJoin +
		" WHERE " + where +
		" ORDER BY a.trust_score DESC " +
		" LIMIT $" + itoa(argIdx) + " OFFSET $" + itoa(argIdx+1)

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
			a.Tags = strings.Split(*tagsArr, ",")
			if len(a.Tags) == 1 && a.Tags[0] == "" {
				a.Tags = []string{}
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
			COALESCE(ap.tags, '{}') as tags,
			COALESCE(ap.total_actions, 0) as total_actions,
			COALESCE(ap.approval_rate, 1.0) as approval_rate,
			ah.last_heartbeat::text as last_seen,
			ah.status
		FROM agents a
		JOIN agent_heartbeats ah ON ah.agent_id = a.agent_id
		LEFT JOIN agent_profiles ap ON ap.agent_id = a.agent_id
		WHERE ah.status = 'online' AND ah.last_heartbeat > NOW() - INTERVAL '2 minutes'
		AND COALESCE(ap.listed, true) = true
		ORDER BY a.trust_score DESC
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
			a.Tags = strings.Split(*tagsArr, ",")
			if len(a.Tags) == 1 && a.Tags[0] == "" {
				a.Tags = []string{}
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
			COALESCE(ap.tags, '{}') as tags,
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
		a.Tags = strings.Split(*tagsArr, ",")
		if len(a.Tags) == 1 && a.Tags[0] == "" {
			a.Tags = []string{}
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