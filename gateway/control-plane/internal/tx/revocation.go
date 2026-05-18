package tx

import (
	"context"
	"fmt"
	"time"

	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/database"
)

type RevocationStore struct {
	q database.Querier
}

func NewRevocationStore(q database.Querier) *RevocationStore {
	return &RevocationStore{q: q}
}

func (r *RevocationStore) RevokeToken(ctx context.Context, tokenID, reason string) error {
	if tokenID == "" {
		return fmt.Errorf("token_id is required")
	}
	_, err := r.q.Exec(ctx,
		`INSERT INTO revoked_tokens (token_id, reason, revoked_at) VALUES ($1, $2, NOW())
		 ON CONFLICT (token_id) DO NOTHING`,
		tokenID, reason,
	)
	if err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}
	return nil
}

func (r *RevocationStore) IsRevoked(ctx context.Context, tokenID string) (bool, error) {
	if tokenID == "" {
		return false, nil
	}
	var exists bool
	err := r.q.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM revoked_tokens WHERE token_id = $1)`,
		tokenID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check revocation: %w", err)
	}
	return exists, nil
}

func (r *RevocationStore) ListRevokedTokens(ctx context.Context, limit int) ([]RevokedToken, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.q.Query(ctx,
		`SELECT token_id, reason, revoked_at FROM revoked_tokens ORDER BY revoked_at DESC LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []RevokedToken
	for rows.Next() {
		var rt RevokedToken
		if err := rows.Scan(&rt.TokenID, &rt.Reason, &rt.RevokedAt); err != nil {
			continue
		}
		tokens = append(tokens, rt)
	}
	return tokens, nil
}

func (r *RevocationStore) CleanupExpired(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.q.Exec(ctx,
		`DELETE FROM revoked_tokens WHERE revoked_at < $1`,
		before,
	)
	if err != nil {
		return 0, fmt.Errorf("cleanup revoked tokens: %w", err)
	}
	return tag.RowsAffected, nil
}

type RevokedToken struct {
	TokenID   string    `json:"token_id"`
	Reason    string    `json:"reason"`
	RevokedAt time.Time `json:"revoked_at"`
}