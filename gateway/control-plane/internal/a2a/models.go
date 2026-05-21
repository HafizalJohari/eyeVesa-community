package a2a

import "time"

type AgentCard struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Owner        string   `json:"owner"`
	Status       string   `json:"status"`
	TrustScore   float64  `json:"trust_score"`
	Capabilities []string `json:"capabilities,omitempty"`
}

type TaskCreateRequest struct {
	FromAgentID string                 `json:"from_agent_id"`
	ToAgentID   string                 `json:"to_agent_id"`
	Action      string                 `json:"action"`
	Input       map[string]interface{} `json:"input,omitempty"`
	Scope       []string               `json:"scope,omitempty"`
	Duration    string                 `json:"duration,omitempty"`
}

type TaskStatus string

const (
	TaskStatusSubmitted TaskStatus = "submitted"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
)

type Task struct {
	TaskID       string                 `json:"task_id"`
	FromAgentID  string                 `json:"from_agent_id"`
	ToAgentID    string                 `json:"to_agent_id"`
	Action       string                 `json:"action"`
	Input        map[string]interface{} `json:"input,omitempty"`
	Scope        []string               `json:"scope,omitempty"`
	Status       TaskStatus             `json:"status"`
	DelegationID string                 `json:"delegation_id,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}
