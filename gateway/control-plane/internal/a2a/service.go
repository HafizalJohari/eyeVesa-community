package a2a

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

type DelegationCreator interface {
	Create(fromAgentID, toAgentID string, scope []string, duration time.Duration) (string, error)
}

type Service struct {
	mu         sync.RWMutex
	tasks      map[string]Task
	delegation DelegationCreator
}

func NewService(delegation DelegationCreator) *Service {
	return &Service{
		tasks:      make(map[string]Task),
		delegation: delegation,
	}
}

func (s *Service) CreateTask(req TaskCreateRequest) (Task, error) {
	if req.FromAgentID == "" || req.ToAgentID == "" || req.Action == "" {
		return Task{}, errors.New("from_agent_id, to_agent_id, and action are required")
	}

	now := time.Now().UTC()
	taskID := uuid.NewString()
	task := Task{
		TaskID:      taskID,
		FromAgentID: req.FromAgentID,
		ToAgentID:   req.ToAgentID,
		Action:      req.Action,
		Input:       req.Input,
		Scope:       req.Scope,
		Status:      TaskStatusSubmitted,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if s.delegation != nil {
		dur := time.Hour
		if req.Duration != "" {
			if parsed, err := time.ParseDuration(req.Duration); err == nil {
				dur = parsed
			}
		}
		delegationID, err := s.delegation.Create(req.FromAgentID, req.ToAgentID, req.Scope, dur)
		if err != nil {
			return Task{}, err
		}
		task.DelegationID = delegationID
	}

	s.mu.Lock()
	s.tasks[taskID] = task
	s.mu.Unlock()

	return task, nil
}

func (s *Service) GetTask(taskID string) (Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task, ok := s.tasks[taskID]
	return task, ok
}
