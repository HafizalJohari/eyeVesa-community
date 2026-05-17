package delegation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/database"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/identity"
)

type mockQuerier struct {
	execFn     func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error)
	queryRowFn func(ctx context.Context, sql string, args ...interface{}) database.Row
	queryFn    func(ctx context.Context, sql string, args ...interface{}) (database.Rows, error)
}

func (m *mockQuerier) Exec(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
	if m.execFn != nil {
		return m.execFn(ctx, sql, args...)
	}
	return database.CommandTag{}, nil
}

func (m *mockQuerier) QueryRow(ctx context.Context, sql string, args ...interface{}) database.Row {
	if m.queryRowFn != nil {
		return m.queryRowFn(ctx, sql, args...)
	}
	return &mockRow{}
}

func (m *mockQuerier) Query(ctx context.Context, sql string, args ...interface{}) (database.Rows, error) {
	if m.queryFn != nil {
		return m.queryFn(ctx, sql, args...)
	}
	return &mockRows{}, nil
}

type mockRow struct {
	scanErr error
	scanFn  func(dest ...interface{}) error
}

func (r *mockRow) Scan(dest ...interface{}) error {
	if r.scanFn != nil {
		return r.scanFn(dest...)
	}
	return r.scanErr
}

type mockRows struct {
	items []func(dest ...interface{}) error
	idx   int
}

func (r *mockRows) Next() bool {
	if r.idx < len(r.items) {
		r.idx++
		return true
	}
	return false
}

func (r *mockRows) Scan(dest ...interface{}) error {
	if r.idx == 0 || r.idx > len(r.items) {
		return errors.New("scan out of bounds")
	}
	return r.items[r.idx-1](dest...)
}

func (r *mockRows) Close() {}

type mockIdentityProvider struct {
	svid    *identity.SVID
	svidErr error
}

func (m *mockIdentityProvider) FetchSVID(ctx context.Context) (*identity.SVID, error) {
	if m.svidErr != nil {
		return nil, m.svidErr
	}
	return m.svid, nil
}

func (m *mockIdentityProvider) WriteCerts(certPath, keyPath string) error {
	return nil
}

func makeSVID() *identity.SVID {
	return &identity.SVID{
		TrustDomain: "agentid.dev",
		SpiffeID:    "spiffe://agentid.dev/gateway",
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	}
}

func makeValidUUIDStr() string {
	return uuid.New().String()
}

func TestNewDelegationTracker(t *testing.T) {
	tracker := NewDelegationTracker(nil, nil)
	if tracker == nil {
		t.Fatal("NewDelegationTracker returned nil")
	}
}

func TestNewDelegationTrackerWithQuerier(t *testing.T) {
	q := &mockQuerier{}
	provider := &mockIdentityProvider{svid: makeSVID()}
	tracker := NewDelegationTrackerWithQuerier(q, provider)
	if tracker == nil {
		t.Fatal("NewDelegationTrackerWithQuerier returned nil")
	}
	if tracker.q != q {
		t.Fatal("expected querier to be set")
	}
	if tracker.provider != provider {
		t.Fatal("expected provider to be set")
	}
}

func TestDelegate_Success(t *testing.T) {
	parentUUID := uuid.New()
	childUUID := uuid.New()
	svid := makeSVID()

	callCount := 0
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			callCount++
			switch callCount {
			case 1:
				return &mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*int)) = 0
					return nil
				}}
			case 2:
				return &mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*string)) = "owner-1"
					return nil
				}}
			case 3:
				return &mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*string)) = "owner-2"
					return nil
				}}
			default:
				return &mockRow{scanErr: errors.New("unexpected queryRow call")}
			}
		},
	}

	provider := &mockIdentityProvider{svid: svid}
	tracker := NewDelegationTrackerWithQuerier(q, provider)

	req := DelegateRequest{
		ParentAgentID: parentUUID.String(),
		ChildAgentID:  childUUID.String(),
		Scope:         []string{"read", "write"},
		MaxDepth:      2,
		Duration:      30 * time.Minute,
	}

	result, err := tracker.Delegate(context.Background(), req)
	if err != nil {
		t.Fatalf("Delegate returned error: %v", err)
	}
	if result == nil {
		t.Fatal("Delegate returned nil result")
	}
	if result.ParentAgentID != parentUUID {
		t.Fatalf("expected parent agent ID %v, got %v", parentUUID, result.ParentAgentID)
	}
	if result.ChildAgentID != childUUID {
		t.Fatalf("expected child agent ID %v, got %v", childUUID, result.ChildAgentID)
	}
	if len(result.Scope) != 2 || result.Scope[0] != "read" || result.Scope[1] != "write" {
		t.Fatalf("expected scope [read, write], got %v", result.Scope)
	}
	if result.MaxDepth != 2 {
		t.Fatalf("expected max depth 2, got %d", result.MaxDepth)
	}
	if result.SVID != svid {
		t.Fatal("expected SVID to be set")
	}
}

func TestDelegate_DefaultScope(t *testing.T) {
	parentUUID := uuid.New()
	childUUID := uuid.New()

	callCount := 0
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			callCount++
			switch callCount {
			case 1:
				return &mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*int)) = 0
					return nil
				}}
			case 2:
				return &mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*string)) = "owner-1"
					return nil
				}}
			case 3:
				return &mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*string)) = "owner-2"
					return nil
				}}
			default:
				return &mockRow{scanErr: errors.New("unexpected queryRow call")}
			}
		},
	}

	provider := &mockIdentityProvider{svid: makeSVID()}
	tracker := NewDelegationTrackerWithQuerier(q, provider)

	req := DelegateRequest{
		ParentAgentID: parentUUID.String(),
		ChildAgentID:  childUUID.String(),
		Scope:         nil,
		MaxDepth:      1,
		Duration:      0,
	}

	result, err := tracker.Delegate(context.Background(), req)
	if err != nil {
		t.Fatalf("Delegate returned error: %v", err)
	}
	if result.Scope == nil {
		t.Fatal("expected scope to be non-nil")
	}
	if len(result.Scope) != 0 {
		t.Fatalf("expected empty scope, got %v", result.Scope)
	}
}

func TestDelegate_DefaultDuration(t *testing.T) {
	parentUUID := uuid.New()
	childUUID := uuid.New()

	callCount := 0
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			callCount++
			switch callCount {
			case 1:
				return &mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*int)) = 0
					return nil
				}}
			case 2:
				return &mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*string)) = "owner-1"
					return nil
				}}
			case 3:
				return &mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*string)) = "owner-2"
					return nil
				}}
			default:
				return &mockRow{scanErr: errors.New("unexpected queryRow call")}
			}
		},
	}

	provider := &mockIdentityProvider{svid: makeSVID()}
	tracker := NewDelegationTrackerWithQuerier(q, provider)

	before := time.Now()
	req := DelegateRequest{
		ParentAgentID: parentUUID.String(),
		ChildAgentID:  childUUID.String(),
		Scope:         []string{"read"},
		Duration:      0,
	}
	result, err := tracker.Delegate(context.Background(), req)
	if err != nil {
		t.Fatalf("Delegate returned error: %v", err)
	}
	after := time.Now()

	expectedMin := before.Add(1 * time.Hour)
	expectedMax := after.Add(1 * time.Hour)
	if result.ExpiresAt.Before(expectedMin) || result.ExpiresAt.After(expectedMax) {
		t.Fatalf("expected expires_at around %v, got %v", expectedMin, result.ExpiresAt)
	}
}

func TestDelegate_ChainTooDeep(t *testing.T) {
	parentUUID := uuid.New()
	childUUID := uuid.New()

	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanFn: func(dest ...interface{}) error {
				*(dest[0].(*int)) = 3
				return nil
			}}
		},
	}

	provider := &mockIdentityProvider{svid: makeSVID()}
	tracker := NewDelegationTrackerWithQuerier(q, provider)

	req := DelegateRequest{
		ParentAgentID: parentUUID.String(),
		ChildAgentID:  childUUID.String(),
		Scope:         []string{"read"},
		Duration:      30 * time.Minute,
	}

	result, err := tracker.Delegate(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for deep chain, got nil")
	}
	if result != nil {
		t.Fatal("expected nil result for deep chain")
	}
}

func TestDelegate_ChainDepthAtLimit(t *testing.T) {
	parentUUID := uuid.New()
	childUUID := uuid.New()

	callCount := 0
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			callCount++
			switch callCount {
			case 1:
				return &mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*int)) = 2
					return nil
				}}
			case 2:
				return &mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*string)) = "owner-1"
					return nil
				}}
			case 3:
				return &mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*string)) = "owner-2"
					return nil
				}}
			default:
				return &mockRow{scanErr: errors.New("unexpected queryRow call")}
			}
		},
	}

	provider := &mockIdentityProvider{svid: makeSVID()}
	tracker := NewDelegationTrackerWithQuerier(q, provider)

	req := DelegateRequest{
		ParentAgentID: parentUUID.String(),
		ChildAgentID:  childUUID.String(),
		Scope:         []string{"read"},
		Duration:      30 * time.Minute,
	}

	result, err := tracker.Delegate(context.Background(), req)
	if err != nil {
		t.Fatalf("expected success at chain depth 2, got error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestDelegate_InvalidParentID(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanFn: func(dest ...interface{}) error {
				*(dest[0].(*int)) = 0
				return nil
			}}
		},
	}

	provider := &mockIdentityProvider{svid: makeSVID()}
	tracker := NewDelegationTrackerWithQuerier(q, provider)

	req := DelegateRequest{
		ParentAgentID: "not-a-uuid",
		ChildAgentID:  uuid.New().String(),
		Scope:         []string{"read"},
		Duration:      30 * time.Minute,
	}

	result, err := tracker.Delegate(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for invalid parent ID, got nil")
	}
	if result != nil {
		t.Fatal("expected nil result for invalid parent ID")
	}
}

func TestDelegate_InvalidChildID(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanFn: func(dest ...interface{}) error {
				*(dest[0].(*int)) = 0
				return nil
			}}
		},
	}

	provider := &mockIdentityProvider{svid: makeSVID()}
	tracker := NewDelegationTrackerWithQuerier(q, provider)

	req := DelegateRequest{
		ParentAgentID: uuid.New().String(),
		ChildAgentID:  "not-a-uuid",
		Scope:         []string{"read"},
		Duration:      30 * time.Minute,
	}

	result, err := tracker.Delegate(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for invalid child ID, got nil")
	}
	if result != nil {
		t.Fatal("expected nil result for invalid child ID")
	}
}

func TestDelegate_ParentNotFound(t *testing.T) {
	callCount := 0
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			callCount++
			switch callCount {
			case 1:
				return &mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*int)) = 0
					return nil
				}}
			case 2:
				return &mockRow{scanErr: errors.New("not found")}
			default:
				return &mockRow{scanErr: errors.New("unexpected call")}
			}
		},
	}

	provider := &mockIdentityProvider{svid: makeSVID()}
	tracker := NewDelegationTrackerWithQuerier(q, provider)

	req := DelegateRequest{
		ParentAgentID: uuid.New().String(),
		ChildAgentID:  uuid.New().String(),
		Scope:         []string{"read"},
		Duration:      30 * time.Minute,
	}

	result, err := tracker.Delegate(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when parent not found, got nil")
	}
	if result != nil {
		t.Fatal("expected nil result when parent not found")
	}
}

func TestDelegate_ChildNotFound(t *testing.T) {
	callCount := 0
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			callCount++
			switch callCount {
			case 1:
				return &mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*int)) = 0
					return nil
				}}
			case 2:
				return &mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*string)) = "owner-1"
					return nil
				}}
			case 3:
				return &mockRow{scanErr: errors.New("not found")}
			default:
				return &mockRow{scanErr: errors.New("unexpected call")}
			}
		},
	}

	provider := &mockIdentityProvider{svid: makeSVID()}
	tracker := NewDelegationTrackerWithQuerier(q, provider)

	req := DelegateRequest{
		ParentAgentID: uuid.New().String(),
		ChildAgentID:  uuid.New().String(),
		Scope:         []string{"read"},
		Duration:      30 * time.Minute,
	}

	result, err := tracker.Delegate(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when child not found, got nil")
	}
	if result != nil {
		t.Fatal("expected nil result when child not found")
	}
}

func TestDelegate_SVIDFetchFails(t *testing.T) {
	callCount := 0
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			callCount++
			switch callCount {
			case 1:
				return &mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*int)) = 0
					return nil
				}}
			case 2:
				return &mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*string)) = "owner-1"
					return nil
				}}
			case 3:
				return &mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*string)) = "owner-2"
					return nil
				}}
			default:
				return &mockRow{scanErr: errors.New("unexpected call")}
			}
		},
	}

	provider := &mockIdentityProvider{svidErr: errors.New("SVID unavailable")}
	tracker := NewDelegationTrackerWithQuerier(q, provider)

	req := DelegateRequest{
		ParentAgentID: uuid.New().String(),
		ChildAgentID:  uuid.New().String(),
		Scope:         []string{"read"},
		Duration:      30 * time.Minute,
	}

	result, err := tracker.Delegate(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when SVID fetch fails, got nil")
	}
	if result != nil {
		t.Fatal("expected nil result when SVID fetch fails")
	}
}

func TestDelegate_InsertFails(t *testing.T) {
	callCount := 0
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			callCount++
			switch callCount {
			case 1:
				return &mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*int)) = 0
					return nil
				}}
			case 2:
				return &mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*string)) = "owner-1"
					return nil
				}}
			case 3:
				return &mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*string)) = "owner-2"
					return nil
				}}
			default:
				return &mockRow{scanErr: errors.New("unexpected call")}
			}
		},
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			return database.CommandTag{}, errors.New("insert failed")
		},
	}

	provider := &mockIdentityProvider{svid: makeSVID()}
	tracker := NewDelegationTrackerWithQuerier(q, provider)

	req := DelegateRequest{
		ParentAgentID: uuid.New().String(),
		ChildAgentID:  uuid.New().String(),
		Scope:         []string{"read"},
		Duration:      30 * time.Minute,
	}

	result, err := tracker.Delegate(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when insert fails, got nil")
	}
	if result != nil {
		t.Fatal("expected nil result when insert fails")
	}
}

func TestDelegate_ChainDepthQueryFails(t *testing.T) {
	callCount := 0
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			callCount++
			switch callCount {
			case 1:
				return &mockRow{scanErr: errors.New("db error")}
			case 2:
				return &mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*string)) = "owner-1"
					return nil
				}}
			case 3:
				return &mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*string)) = "owner-2"
					return nil
				}}
			default:
				return &mockRow{scanErr: errors.New("unexpected call")}
			}
		},
	}

	provider := &mockIdentityProvider{svid: makeSVID()}
	tracker := NewDelegationTrackerWithQuerier(q, provider)

	req := DelegateRequest{
		ParentAgentID: uuid.New().String(),
		ChildAgentID:  uuid.New().String(),
		Scope:         []string{"read"},
		Duration:      30 * time.Minute,
	}

	result, err := tracker.Delegate(context.Background(), req)
	if err != nil {
		t.Fatalf("expected success when chain depth query fails (defaults to 0), got error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestValidateDelegation_Valid(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanFn: func(dest ...interface{}) error {
				*(dest[0].(*int)) = 1
				return nil
			}}
		},
	}

	provider := &mockIdentityProvider{svid: makeSVID()}
	tracker := NewDelegationTrackerWithQuerier(q, provider)

	valid, err := tracker.ValidateDelegation(context.Background(), "parent-1", "child-1")
	if err != nil {
		t.Fatalf("ValidateDelegation returned error: %v", err)
	}
	if !valid {
		t.Fatal("expected delegation to be valid")
	}
}

func TestValidateDelegation_Invalid(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanFn: func(dest ...interface{}) error {
				*(dest[0].(*int)) = 0
				return nil
			}}
		},
	}

	provider := &mockIdentityProvider{svid: makeSVID()}
	tracker := NewDelegationTrackerWithQuerier(q, provider)

	valid, err := tracker.ValidateDelegation(context.Background(), "parent-1", "child-1")
	if err != nil {
		t.Fatalf("ValidateDelegation returned error: %v", err)
	}
	if valid {
		t.Fatal("expected delegation to be invalid")
	}
}

func TestValidateDelegation_QueryError(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanErr: errors.New("db error")}
		},
	}

	provider := &mockIdentityProvider{svid: makeSVID()}
	tracker := NewDelegationTrackerWithQuerier(q, provider)

	valid, err := tracker.ValidateDelegation(context.Background(), "parent-1", "child-1")
	if err == nil {
		t.Fatal("expected error from query failure, got nil")
	}
	if valid {
		t.Fatal("expected valid=false on error")
	}
}

func TestGetDelegationChain_Success(t *testing.T) {
	delegationID1 := uuid.New()
	parentID1 := uuid.New()
	childID1 := uuid.New()
	delegationID2 := uuid.New()
	parentID2 := uuid.New()
	childID2 := uuid.New()

	rowIdx := 0
	q := &mockQuerier{
		queryFn: func(ctx context.Context, sql string, args ...interface{}) (database.Rows, error) {
			return &mockRows{
				items: []func(dest ...interface{}) error{
					func(dest ...interface{}) error {
						*(dest[0].(*uuid.UUID)) = delegationID1
						*(dest[1].(*uuid.UUID)) = parentID1
						*(dest[2].(*uuid.UUID)) = childID1
						*(dest[3].(*[]string)) = []string{"read"}
						*(dest[4].(*int)) = 1
						*(dest[5].(*time.Time)) = time.Now().Add(1 * time.Hour)
						rowIdx++
						return nil
					},
					func(dest ...interface{}) error {
						*(dest[0].(*uuid.UUID)) = delegationID2
						*(dest[1].(*uuid.UUID)) = parentID2
						*(dest[2].(*uuid.UUID)) = childID2
						*(dest[3].(*[]string)) = []string{"write"}
						*(dest[4].(*int)) = 2
						*(dest[5].(*time.Time)) = time.Now().Add(2 * time.Hour)
						rowIdx++
						return nil
					},
				},
			}, nil
		},
	}

	provider := &mockIdentityProvider{svid: makeSVID()}
	tracker := NewDelegationTrackerWithQuerier(q, provider)

	chains, err := tracker.GetDelegationChain(context.Background(), "agent-1")
	if err != nil {
		t.Fatalf("GetDelegationChain returned error: %v", err)
	}
	if len(chains) != 2 {
		t.Fatalf("expected 2 chains, got %d", len(chains))
	}
	if chains[0].DelegationID != delegationID1 {
		t.Fatalf("expected delegation ID %v, got %v", delegationID1, chains[0].DelegationID)
	}
	if chains[1].DelegationID != delegationID2 {
		t.Fatalf("expected delegation ID %v, got %v", delegationID2, chains[1].DelegationID)
	}
}

func TestGetDelegationChain_EmptyResult(t *testing.T) {
	q := &mockQuerier{
		queryFn: func(ctx context.Context, sql string, args ...interface{}) (database.Rows, error) {
			return &mockRows{items: nil}, nil
		},
	}

	provider := &mockIdentityProvider{svid: makeSVID()}
	tracker := NewDelegationTrackerWithQuerier(q, provider)

	chains, err := tracker.GetDelegationChain(context.Background(), "agent-1")
	if err != nil {
		t.Fatalf("GetDelegationChain returned error: %v", err)
	}
	if chains != nil {
		t.Fatalf("expected nil chains for empty result, got %v", chains)
	}
}

func TestGetDelegationChain_QueryError(t *testing.T) {
	q := &mockQuerier{
		queryFn: func(ctx context.Context, sql string, args ...interface{}) (database.Rows, error) {
			return nil, errors.New("db error")
		},
	}

	provider := &mockIdentityProvider{svid: makeSVID()}
	tracker := NewDelegationTrackerWithQuerier(q, provider)

	chains, err := tracker.GetDelegationChain(context.Background(), "agent-1")
	if err == nil {
		t.Fatal("expected error from query failure, got nil")
	}
	if chains != nil {
		t.Fatal("expected nil chains on error")
	}
}

func TestGetDelegationChain_ScanErrorSkips(t *testing.T) {
	delegationID1 := uuid.New()
	parentID1 := uuid.New()
	childID1 := uuid.New()

	q := &mockQuerier{
		queryFn: func(ctx context.Context, sql string, args ...interface{}) (database.Rows, error) {
			return &mockRows{
				items: []func(dest ...interface{}) error{
					func(dest ...interface{}) error {
						return errors.New("scan error")
					},
					func(dest ...interface{}) error {
						*(dest[0].(*uuid.UUID)) = delegationID1
						*(dest[1].(*uuid.UUID)) = parentID1
						*(dest[2].(*uuid.UUID)) = childID1
						*(dest[3].(*[]string)) = []string{"read"}
						*(dest[4].(*int)) = 1
						*(dest[5].(*time.Time)) = time.Now().Add(1 * time.Hour)
						return nil
					},
				},
			}, nil
		},
	}

	provider := &mockIdentityProvider{svid: makeSVID()}
	tracker := NewDelegationTrackerWithQuerier(q, provider)

	chains, err := tracker.GetDelegationChain(context.Background(), "agent-1")
	if err != nil {
		t.Fatalf("GetDelegationChain returned error: %v", err)
	}
	if len(chains) != 1 {
		t.Fatalf("expected 1 chain (scan error row skipped), got %d", len(chains))
	}
	if chains[0].DelegationID != delegationID1 {
		t.Fatalf("expected delegation ID %v, got %v", delegationID1, chains[0].DelegationID)
	}
}

func TestRevoke_Success(t *testing.T) {
	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			return database.CommandTag{RowsAffected: 1}, nil
		},
	}

	provider := &mockIdentityProvider{svid: makeSVID()}
	tracker := NewDelegationTrackerWithQuerier(q, provider)

	err := tracker.Revoke(context.Background(), uuid.New().String())
	if err != nil {
		t.Fatalf("Revoke returned error: %v", err)
	}
}

func TestRevoke_Error(t *testing.T) {
	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			return database.CommandTag{}, errors.New("delete failed")
		},
	}

	provider := &mockIdentityProvider{svid: makeSVID()}
	tracker := NewDelegationTrackerWithQuerier(q, provider)

	err := tracker.Revoke(context.Background(), uuid.New().String())
	if err == nil {
		t.Fatal("expected error from delete failure, got nil")
	}
}

func TestValidateDelegation_MultipleValid(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanFn: func(dest ...interface{}) error {
				*(dest[0].(*int)) = 5
				return nil
			}}
		},
	}

	provider := &mockIdentityProvider{svid: makeSVID()}
	tracker := NewDelegationTrackerWithQuerier(q, provider)

	valid, err := tracker.ValidateDelegation(context.Background(), "parent-1", "child-1")
	if err != nil {
		t.Fatalf("ValidateDelegation returned error: %v", err)
	}
	if !valid {
		t.Fatal("expected delegation to be valid with count > 0")
	}
}

func TestDelegate_CustomDuration(t *testing.T) {
	parentUUID := uuid.New()
	childUUID := uuid.New()

	callCount := 0
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			callCount++
			switch callCount {
			case 1:
				return &mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*int)) = 0
					return nil
				}}
			case 2:
				return &mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*string)) = "owner-1"
					return nil
				}}
			case 3:
				return &mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*string)) = "owner-2"
					return nil
				}}
			default:
				return &mockRow{scanErr: errors.New("unexpected queryRow call")}
			}
		},
	}

	provider := &mockIdentityProvider{svid: makeSVID()}
	tracker := NewDelegationTrackerWithQuerier(q, provider)

	duration := 2 * time.Hour
	req := DelegateRequest{
		ParentAgentID: parentUUID.String(),
		ChildAgentID:  childUUID.String(),
		Scope:         []string{"admin"},
		MaxDepth:      5,
		Duration:      duration,
	}

	before := time.Now()
	result, err := tracker.Delegate(context.Background(), req)
	after := time.Now()

	if err != nil {
		t.Fatalf("Delegate returned error: %v", err)
	}

	expectedMin := before.Add(duration)
	expectedMax := after.Add(duration)
	if result.ExpiresAt.Before(expectedMin) || result.ExpiresAt.After(expectedMax) {
		t.Fatalf("expected expires_at around %v, got %v", expectedMin, result.ExpiresAt)
	}
}

func TestDelegate_ExecArgsCaptured(t *testing.T) {
	parentUUID := uuid.New()
	childUUID := uuid.New()
	svid := makeSVID()

	var capturedSQL string
	var capturedArgs []interface{}

	callCount := 0
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			callCount++
			switch callCount {
			case 1:
				return &mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*int)) = 0
					return nil
				}}
			case 2:
				return &mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*string)) = "owner-1"
					return nil
				}}
			case 3:
				return &mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*string)) = "owner-2"
					return nil
				}}
			default:
				return &mockRow{scanErr: errors.New("unexpected queryRow call")}
			}
		},
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			capturedSQL = sql
			capturedArgs = args
			return database.CommandTag{RowsAffected: 1}, nil
		},
	}

	provider := &mockIdentityProvider{svid: svid}
	tracker := NewDelegationTrackerWithQuerier(q, provider)

	req := DelegateRequest{
		ParentAgentID: parentUUID.String(),
		ChildAgentID:  childUUID.String(),
		Scope:         []string{"read", "write"},
		MaxDepth:      3,
		Duration:      1 * time.Hour,
	}

	result, err := tracker.Delegate(context.Background(), req)
	if err != nil {
		t.Fatalf("Delegate returned error: %v", err)
	}

	if capturedSQL == "" {
		t.Fatal("expected exec SQL to be captured")
	}
	if len(capturedArgs) != 6 {
		t.Fatalf("expected 6 exec args, got %d", len(capturedArgs))
	}
	if capturedArgs[0] != result.DelegationID {
		t.Fatalf("expected first arg to be delegation ID %v, got %v", result.DelegationID, capturedArgs[0])
	}
	if capturedArgs[1] != result.ParentAgentID {
		t.Fatalf("expected second arg to be parent ID %v, got %v", result.ParentAgentID, capturedArgs[1])
	}
	if capturedArgs[2] != result.ChildAgentID {
		t.Fatalf("expected third arg to be child ID %v, got %v", result.ChildAgentID, capturedArgs[2])
	}
}

func TestRevoke_ArgsCaptured(t *testing.T) {
	delegationID := uuid.New().String()
	var capturedSQL string
	var capturedArgs []interface{}

	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			capturedSQL = sql
			capturedArgs = args
			return database.CommandTag{RowsAffected: 1}, nil
		},
	}

	provider := &mockIdentityProvider{svid: makeSVID()}
	tracker := NewDelegationTrackerWithQuerier(q, provider)

	err := tracker.Revoke(context.Background(), delegationID)
	if err != nil {
		t.Fatalf("Revoke returned error: %v", err)
	}
	if capturedSQL == "" {
		t.Fatal("expected exec SQL to be captured")
	}
	if len(capturedArgs) != 1 {
		t.Fatalf("expected 1 exec arg, got %d", len(capturedArgs))
	}
	if capturedArgs[0] != delegationID {
		t.Fatalf("expected arg to be %s, got %v", delegationID, capturedArgs[0])
	}
}

func TestDelegate_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	callCount := 0
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			callCount++
			switch callCount {
			case 1:
				return &mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*int)) = 0
					return nil
				}}
			case 2:
				return &mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*string)) = "owner-1"
					return nil
				}}
			case 3:
				return &mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*string)) = "owner-2"
					return nil
				}}
			default:
				return &mockRow{scanErr: errors.New("unexpected call")}
			}
		},
	}

	provider := &mockIdentityProvider{svid: makeSVID()}
	tracker := NewDelegationTrackerWithQuerier(q, provider)

	req := DelegateRequest{
		ParentAgentID: uuid.New().String(),
		ChildAgentID:  uuid.New().String(),
		Scope:         []string{"read"},
		Duration:      30 * time.Minute,
	}

	_, _ = tracker.Delegate(ctx, req)
}

func TestGetDelegationChain_SingleChain(t *testing.T) {
	delegationID := uuid.New()
	parentID := uuid.New()
	childID := uuid.New()

	q := &mockQuerier{
		queryFn: func(ctx context.Context, sql string, args ...interface{}) (database.Rows, error) {
			return &mockRows{
				items: []func(dest ...interface{}) error{
					func(dest ...interface{}) error {
						*(dest[0].(*uuid.UUID)) = delegationID
						*(dest[1].(*uuid.UUID)) = parentID
						*(dest[2].(*uuid.UUID)) = childID
						*(dest[3].(*[]string)) = []string{"read", "write"}
						*(dest[4].(*int)) = 3
						*(dest[5].(*time.Time)) = time.Now().Add(1 * time.Hour)
						return nil
					},
				},
			}, nil
		},
	}

	provider := &mockIdentityProvider{svid: makeSVID()}
	tracker := NewDelegationTrackerWithQuerier(q, provider)

	chains, err := tracker.GetDelegationChain(context.Background(), "agent-1")
	if err != nil {
		t.Fatalf("GetDelegationChain returned error: %v", err)
	}
	if len(chains) != 1 {
		t.Fatalf("expected 1 chain, got %d", len(chains))
	}
	c := chains[0]
	if c.DelegationID != delegationID {
		t.Fatalf("expected delegation ID %v, got %v", delegationID, c.DelegationID)
	}
	if c.ParentAgentID != parentID {
		t.Fatalf("expected parent ID %v, got %v", parentID, c.ParentAgentID)
	}
	if c.ChildAgentID != childID {
		t.Fatalf("expected child ID %v, got %v", childID, c.ChildAgentID)
	}
	if len(c.Scope) != 2 {
		t.Fatalf("expected scope length 2, got %d", len(c.Scope))
	}
	if c.MaxDepth != 3 {
		t.Fatalf("expected max depth 3, got %d", c.MaxDepth)
	}
}