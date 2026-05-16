package delegation

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/identity"
)

type DelegationTracker struct {
	db     *pgxpool.Pool
	provider identity.IdentityProvider
}

func NewDelegationTracker(db *pgxpool.Pool, provider identity.IdentityProvider) *DelegationTracker {
	return &DelegationTracker{
		db:       db,
		provider: provider,
	}
}

type DelegationChain struct {
	DelegationID uuid.UUID
	ParentAgentID uuid.UUID
	ChildAgentID  uuid.UUID
	Scope         []string
	MaxDepth      int
	ExpiresAt     time.Time
	ApprovedBy    *uuid.UUID
	SVID          *identity.SVID
}

type DelegateRequest struct {
	ParentAgentID string
	ChildAgentID  string
	Scope         []string
	MaxDepth      int
	Duration      time.Duration
}

func (dt *DelegationTracker) Delegate(ctx context.Context, req DelegateRequest) (*DelegationChain, error) {
	var chainDepth int
	err := dt.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM delegations WHERE child_agent_id = $1`,
		req.ChildAgentID,
	).Scan(&chainDepth)
	if err != nil {
		chainDepth = 0
	}

	if chainDepth >= 3 {
		return nil, fmt.Errorf("delegation chain too deep: agent has %d parent delegations (max 3)", chainDepth)
	}

	parentID, err := uuid.Parse(req.ParentAgentID)
	if err != nil {
		return nil, fmt.Errorf("invalid parent agent ID: %w", err)
	}
	childID, err := uuid.Parse(req.ChildAgentID)
	if err != nil {
		return nil, fmt.Errorf("invalid child agent ID: %w", err)
	}

	var parentOwner string
	err = dt.db.QueryRow(ctx,
		`SELECT owner FROM agents WHERE agent_id = $1 AND status = 'active'`,
		req.ParentAgentID,
	).Scan(&parentOwner)
	if err != nil {
		return nil, fmt.Errorf("parent agent not found or inactive: %w", err)
	}

	var childOwner string
	err = dt.db.QueryRow(ctx,
		`SELECT owner FROM agents WHERE agent_id = $1 AND status = 'active'`,
		req.ChildAgentID,
	).Scan(&childOwner)
	if err != nil {
		return nil, fmt.Errorf("child agent not found or inactive: %w", err)
	}

	svid, err := dt.provider.FetchSVID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch SVID for delegation: %w", err)
	}

	delegationID := uuid.New()
	expiresAt := time.Now().Add(req.Duration)
	if req.Duration == 0 {
		expiresAt = time.Now().Add(1 * time.Hour)
	}

	effectiveScope := req.Scope
	if effectiveScope == nil {
		effectiveScope = []string{}
	}

	_, err = dt.db.Exec(ctx,
		`INSERT INTO delegations (delegation_id, parent_agent_id, child_agent_id, scope, max_depth, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		delegationID, parentID, childID, effectiveScope, req.MaxDepth, expiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to record delegation: %w", err)
	}

	return &DelegationChain{
		DelegationID: delegationID,
		ParentAgentID: parentID,
		ChildAgentID:  childID,
		Scope:         effectiveScope,
		MaxDepth:      req.MaxDepth,
		ExpiresAt:     expiresAt,
		SVID:          svid,
	}, nil
}

func (dt *DelegationTracker) ValidateDelegation(ctx context.Context, parentAgentID, childAgentID string) (bool, error) {
	var count int
	err := dt.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM delegations
		 WHERE parent_agent_id = $1 AND child_agent_id = $2 AND expires_at > NOW()`,
		parentAgentID, childAgentID,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (dt *DelegationTracker) GetDelegationChain(ctx context.Context, agentID string) ([]DelegationChain, error) {
	rows, err := dt.db.Query(ctx,
		`SELECT delegation_id, parent_agent_id, child_agent_id, scope, max_depth, expires_at
		 FROM delegations
		 WHERE parent_agent_id = $1 OR child_agent_id = $1
		 ORDER BY created_at`,
		agentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chains []DelegationChain
	for rows.Next() {
		var c DelegationChain
		if err := rows.Scan(&c.DelegationID, &c.ParentAgentID, &c.ChildAgentID, &c.Scope, &c.MaxDepth, &c.ExpiresAt); err != nil {
			continue
		}
		chains = append(chains, c)
	}
	return chains, nil
}

func (dt *DelegationTracker) Revoke(ctx context.Context, delegationID string) error {
	_, err := dt.db.Exec(ctx,
		`DELETE FROM delegations WHERE delegation_id = $1`,
		delegationID,
	)
	return err
}