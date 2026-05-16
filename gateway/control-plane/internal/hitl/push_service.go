package hitl

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PushToken struct {
	TokenID     string `json:"token_id"`
	ApproverID  string `json:"approver_id"`
	DeviceToken string `json:"device_token"`
	Platform    string `json:"platform"`
	BundleID    string `json:"bundle_id,omitempty"`
	IsActive    bool   `json:"is_active"`
	CreatedAt   string `json:"created_at"`
}

type PushService struct {
	db *pgxpool.Pool
}

func NewPushService(db *pgxpool.Pool) *PushService {
	return &PushService{db: db}
}

func (s *PushService) RegisterToken(ctx context.Context, approverID, deviceToken, platform, bundleID string) (*PushToken, error) {
	tokenID := uuid.New()

	var isActive bool = true
	err := s.db.QueryRow(ctx,
		`INSERT INTO push_tokens (token_id, approver_id, device_token, platform, bundle_id, is_active)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (approver_id, device_token) DO UPDATE SET is_active = TRUE, updated_at = NOW()
		 RETURNING token_id, is_active`,
		tokenID, approverID, deviceToken, platform, bundleID, isActive,
	).Scan(&tokenID, &isActive)

	if err != nil {
		return nil, fmt.Errorf("failed to register push token: %w", err)
	}

	return &PushToken{
		TokenID:     tokenID.String(),
		ApproverID:  approverID,
		DeviceToken: deviceToken,
		Platform:    platform,
		BundleID:    bundleID,
		IsActive:    true,
	}, nil
}

func (s *PushService) GetTokensForApprover(ctx context.Context, approverID string) ([]PushToken, error) {
	rows, err := s.db.Query(ctx,
		`SELECT token_id, approver_id, device_token, platform, COALESCE(bundle_id, ''), is_active, created_at::text
		 FROM push_tokens WHERE approver_id = $1 AND is_active = TRUE`,
		approverID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []PushToken
	for rows.Next() {
		var t PushToken
		if err := rows.Scan(&t.TokenID, &t.ApproverID, &t.DeviceToken, &t.Platform, &t.BundleID, &t.IsActive, &t.CreatedAt); err != nil {
			continue
		}
		tokens = append(tokens, t)
	}
	return tokens, nil
}

func (s *PushService) DeactivateToken(ctx context.Context, tokenID string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE push_tokens SET is_active = FALSE, updated_at = NOW() WHERE token_id = $1`,
		tokenID,
	)
	return err
}