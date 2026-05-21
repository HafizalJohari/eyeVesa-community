package tenant

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/database"
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
	items   []func(dest ...interface{}) error
	idx     int
	closed  bool
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

func (r *mockRows) Close() {
	r.closed = true
}

func now() time.Time {
	return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
}

func newTenantRow(t Tenant) func(dest ...interface{}) error {
	return func(dest ...interface{}) error {
		if len(dest) != 11 {
			return fmt.Errorf("expected 11 scan destinations, got %d", len(dest))
		}
		*(dest[0].(*string)) = t.TenantID
		*(dest[1].(*string)) = t.Name
		*(dest[2].(*string)) = t.Slug
		*(dest[3].(*string)) = t.Plan
		*(dest[4].(*int)) = t.MaxAgents
		*(dest[5].(*int)) = t.MaxResources
		*(dest[6].(*bool)) = t.SSOEnabled
		*(dest[7].(*string)) = t.SSOProvider
		*(dest[8].(*string)) = t.SSOConfig
		*(dest[9].(*time.Time)) = t.CreatedAt
		*(dest[10].(*time.Time)) = t.UpdatedAt
		return nil
	}
}

func newApproverRow(a Approver) func(dest ...interface{}) error {
	return func(dest ...interface{}) error {
		if len(dest) != 10 {
			return fmt.Errorf("expected 10 scan destinations, got %d", len(dest))
		}
		*(dest[0].(*string)) = a.ApproverID
		*(dest[1].(*string)) = a.TenantID
		*(dest[2].(*string)) = a.Email
		*(dest[3].(*string)) = a.Name
		*(dest[4].(*string)) = a.Role
		*(dest[5].(*string)) = a.SSOSubject
		*(dest[6].(*string)) = a.NotificationChannel
		*(dest[7].(*string)) = a.NotificationTarget
		*(dest[8].(*bool)) = a.IsActive
		*(dest[9].(*time.Time)) = a.CreatedAt
		return nil
	}
}

func newApproverSSORow(a Approver) func(dest ...interface{}) error {
	return func(dest ...interface{}) error {
		if len(dest) != 9 {
			return fmt.Errorf("expected 9 scan destinations, got %d", len(dest))
		}
		*(dest[0].(*string)) = a.ApproverID
		*(dest[1].(*string)) = a.TenantID
		*(dest[2].(*string)) = a.Email
		*(dest[3].(*string)) = a.Name
		*(dest[4].(*string)) = a.Role
		*(dest[5].(*string)) = a.SSOSubject
		*(dest[6].(*string)) = a.NotificationChannel
		*(dest[7].(*string)) = a.NotificationTarget
		*(dest[8].(*bool)) = a.IsActive
		return nil
	}
}

func TestNewTenantService_NilDB(t *testing.T) {
	svc := NewTenantService(nil)
	if svc == nil {
		t.Fatal("NewTenantService returned nil")
	}
	if svc.q != nil {
		t.Fatal("expected nil querier when db is nil")
	}
}

func TestNewTenantServiceWithQuerier(t *testing.T) {
	q := &mockQuerier{}
	svc := NewTenantServiceWithQuerier(q)
	if svc == nil {
		t.Fatal("NewTenantServiceWithQuerier returned nil")
	}
	if svc.q != q {
		t.Fatal("expected querier to be set")
	}
}

func TestCreateTenant_Success(t *testing.T) {
	var capturedArgs []interface{}
	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			capturedArgs = args
			return database.CommandTag{RowsAffected: 1}, nil
		},
	}
	svc := NewTenantServiceWithQuerier(q)

	tenant, err := svc.CreateTenant(context.Background(), "Acme Corp", "acme", "enterprise", 100, 500)
	if err != nil {
		t.Fatalf("CreateTenant returned error: %v", err)
	}

	if tenant.Name != "Acme Corp" {
		t.Fatalf("expected name 'Acme Corp', got %q", tenant.Name)
	}
	if tenant.Slug != "acme" {
		t.Fatalf("expected slug 'acme', got %q", tenant.Slug)
	}
	if tenant.Plan != "enterprise" {
		t.Fatalf("expected plan 'enterprise', got %q", tenant.Plan)
	}
	if tenant.MaxAgents != 100 {
		t.Fatalf("expected max_agents 100, got %d", tenant.MaxAgents)
	}
	if tenant.MaxResources != 500 {
		t.Fatalf("expected max_resources 500, got %d", tenant.MaxResources)
	}
	if tenant.TenantID == "" {
		t.Fatal("expected auto-generated tenant_id")
	}
	if len(tenant.TenantID) != 36 {
		t.Fatalf("expected UUID format tenant_id, got %q", tenant.TenantID)
	}
	if tenant.CreatedAt.IsZero() {
		t.Fatal("expected non-zero created_at")
	}
	if tenant.UpdatedAt.IsZero() {
		t.Fatal("expected non-zero updated_at")
	}

	if len(capturedArgs) != 6 {
		t.Fatalf("expected 6 args, got %d", len(capturedArgs))
	}
	if capturedArgs[1].(string) != "Acme Corp" {
		t.Fatalf("expected name arg 'Acme Corp', got %v", capturedArgs[1])
	}
	if capturedArgs[2].(string) != "acme" {
		t.Fatalf("expected slug arg 'acme', got %v", capturedArgs[2])
	}
}

func TestCreateTenant_DBError(t *testing.T) {
	dbErr := errors.New("unique constraint violation")
	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			return database.CommandTag{}, dbErr
		},
	}
	svc := NewTenantServiceWithQuerier(q)

	tenant, err := svc.CreateTenant(context.Background(), "Acme", "acme", "free", 10, 50)
	if err == nil {
		t.Fatal("expected error from DB, got nil")
	}
	if tenant != nil {
		t.Fatalf("expected nil tenant on error, got %v", tenant)
	}
	if err.Error() != "create tenant: unique constraint violation" {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

func TestCreateTenant_DuplicateSlug(t *testing.T) {
	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			return database.CommandTag{}, errors.New("duplicate key value violates unique constraint")
		},
	}
	svc := NewTenantServiceWithQuerier(q)

	_, err := svc.CreateTenant(context.Background(), "Acme", "acme", "free", 10, 50)
	if err == nil {
		t.Fatal("expected error for duplicate slug")
	}
}

func TestGetTenant_Success(t *testing.T) {
	ts := now()
	expected := Tenant{
		TenantID:     "550e8400-e29b-41d4-a716-446655440000",
		Name:         "Acme Corp",
		Slug:         "acme",
		Plan:         "enterprise",
		MaxAgents:    100,
		MaxResources: 500,
		SSOEnabled:   true,
		SSOProvider:  "okta",
		SSOConfig:    `{"domain":"example.okta.com"}`,
		CreatedAt:    ts,
		UpdatedAt:    ts,
	}

	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanFn: newTenantRow(expected)}
		},
	}
	svc := NewTenantServiceWithQuerier(q)

	tenant, err := svc.GetTenant(context.Background(), expected.TenantID)
	if err != nil {
		t.Fatalf("GetTenant returned error: %v", err)
	}
	if tenant.TenantID != expected.TenantID {
		t.Fatalf("expected tenant_id %q, got %q", expected.TenantID, tenant.TenantID)
	}
	if tenant.Name != expected.Name {
		t.Fatalf("expected name %q, got %q", expected.Name, tenant.Name)
	}
	if tenant.SSOEnabled != expected.SSOEnabled {
		t.Fatalf("expected sso_enabled %v, got %v", expected.SSOEnabled, tenant.SSOEnabled)
	}
	if tenant.SSOProvider != expected.SSOProvider {
		t.Fatalf("expected sso_provider %q, got %q", expected.SSOProvider, tenant.SSOProvider)
	}
}

func TestGetTenant_NotFound(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanErr: errors.New("no rows in result set")}
		},
	}
	svc := NewTenantServiceWithQuerier(q)

	tenant, err := svc.GetTenant(context.Background(), "nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent tenant")
	}
	if tenant != nil {
		t.Fatalf("expected nil tenant, got %v", tenant)
	}
	if err.Error() != "tenant not found: no rows in result set" {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

func TestGetTenantBySlug_Success(t *testing.T) {
	ts := now()
	expected := Tenant{
		TenantID:     "660e8400-e29b-41d4-a716-446655440001",
		Name:         "Globex",
		Slug:         "globex",
		Plan:         "pro",
		MaxAgents:    50,
		MaxResources: 200,
		SSOEnabled:   false,
		SSOProvider:  "",
		SSOConfig:    "",
		CreatedAt:    ts,
		UpdatedAt:    ts,
	}

	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanFn: newTenantRow(expected)}
		},
	}
	svc := NewTenantServiceWithQuerier(q)

	tenant, err := svc.GetTenantBySlug(context.Background(), "globex")
	if err != nil {
		t.Fatalf("GetTenantBySlug returned error: %v", err)
	}
	if tenant.Slug != "globex" {
		t.Fatalf("expected slug 'globex', got %q", tenant.Slug)
	}
	if tenant.Name != "Globex" {
		t.Fatalf("expected name 'Globex', got %q", tenant.Name)
	}
}

func TestGetTenantBySlug_NotFound(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanErr: errors.New("no rows in result set")}
		},
	}
	svc := NewTenantServiceWithQuerier(q)

	tenant, err := svc.GetTenantBySlug(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent slug")
	}
	if tenant != nil {
		t.Fatalf("expected nil tenant, got %v", tenant)
	}
}

func TestListTenants_Success(t *testing.T) {
	ts := now()
	tenants := []Tenant{
		{
			TenantID: "id-1", Name: "First", Slug: "first", Plan: "free",
			MaxAgents: 10, MaxResources: 50, SSOEnabled: false,
			SSOProvider: "", SSOConfig: "", CreatedAt: ts, UpdatedAt: ts,
		},
		{
			TenantID: "id-2", Name: "Second", Slug: "second", Plan: "pro",
			MaxAgents: 50, MaxResources: 200, SSOEnabled: true,
			SSOProvider: "okta", SSOConfig: "{}", CreatedAt: ts, UpdatedAt: ts,
		},
	}

	scanFns := make([]func(dest ...interface{}) error, len(tenants))
	for i, t := range tenants {
		scanFns[i] = newTenantRow(t)
	}

	q := &mockQuerier{
		queryFn: func(ctx context.Context, sql string, args ...interface{}) (database.Rows, error) {
			return &mockRows{items: scanFns}, nil
		},
	}
	svc := NewTenantServiceWithQuerier(q)

	result, err := svc.ListTenants(context.Background())
	if err != nil {
		t.Fatalf("ListTenants returned error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 tenants, got %d", len(result))
	}
	if result[0].TenantID != "id-1" {
		t.Fatalf("expected first tenant id-1, got %q", result[0].TenantID)
	}
	if result[1].Name != "Second" {
		t.Fatalf("expected second tenant 'Second', got %q", result[1].Name)
	}
}

func TestListTenants_Empty(t *testing.T) {
	q := &mockQuerier{
		queryFn: func(ctx context.Context, sql string, args ...interface{}) (database.Rows, error) {
			return &mockRows{items: nil}, nil
		},
	}
	svc := NewTenantServiceWithQuerier(q)

	result, err := svc.ListTenants(context.Background())
	if err != nil {
		t.Fatalf("ListTenants returned error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil for empty list, got %v", result)
	}
}

func TestListTenants_QueryError(t *testing.T) {
	q := &mockQuerier{
		queryFn: func(ctx context.Context, sql string, args ...interface{}) (database.Rows, error) {
			return nil, errors.New("connection refused")
		},
	}
	svc := NewTenantServiceWithQuerier(q)

	result, err := svc.ListTenants(context.Background())
	if err == nil {
		t.Fatal("expected error from DB query")
	}
	if result != nil {
		t.Fatalf("expected nil result, got %v", result)
	}
}

func TestListTenants_ScanErrorSkipsRow(t *testing.T) {
	ts := now()
	scanFns := []func(dest ...interface{}) error{
		func(dest ...interface{}) error { return errors.New("scan failed") },
		newTenantRow(Tenant{
			TenantID: "id-ok", Name: "OK", Slug: "ok", Plan: "free",
			MaxAgents: 10, MaxResources: 50, SSOEnabled: false,
			SSOProvider: "", SSOConfig: "", CreatedAt: ts, UpdatedAt: ts,
		}),
	}

	q := &mockQuerier{
		queryFn: func(ctx context.Context, sql string, args ...interface{}) (database.Rows, error) {
			return &mockRows{items: scanFns}, nil
		},
	}
	svc := NewTenantServiceWithQuerier(q)

	result, err := svc.ListTenants(context.Background())
	if err != nil {
		t.Fatalf("ListTenants returned error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 tenant (scan error skipped), got %d", len(result))
	}
	if result[0].TenantID != "id-ok" {
		t.Fatalf("expected tenant id-ok, got %q", result[0].TenantID)
	}
}

func TestUpdatePlan_Success(t *testing.T) {
	var capturedArgs []interface{}
	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			capturedArgs = args
			return database.CommandTag{RowsAffected: 1}, nil
		},
	}
	svc := NewTenantServiceWithQuerier(q)

	err := svc.UpdatePlan(context.Background(), "tenant-1", "enterprise", 200, 1000)
	if err != nil {
		t.Fatalf("UpdatePlan returned error: %v", err)
	}
	if capturedArgs[0].(string) != "enterprise" {
		t.Fatalf("expected plan 'enterprise', got %v", capturedArgs[0])
	}
	if capturedArgs[1].(int) != 200 {
		t.Fatalf("expected max_agents 200, got %v", capturedArgs[1])
	}
	if capturedArgs[2].(int) != 1000 {
		t.Fatalf("expected max_resources 1000, got %v", capturedArgs[2])
	}
	if capturedArgs[3].(string) != "tenant-1" {
		t.Fatalf("expected tenant_id 'tenant-1', got %v", capturedArgs[3])
	}
}

func TestUpdatePlan_DBError(t *testing.T) {
	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			return database.CommandTag{}, errors.New("update failed")
		},
	}
	svc := NewTenantServiceWithQuerier(q)

	err := svc.UpdatePlan(context.Background(), "tenant-1", "pro", 50, 200)
	if err == nil {
		t.Fatal("expected error from DB")
	}
}

func TestEnableSSO_Success(t *testing.T) {
	var capturedArgs []interface{}
	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			capturedArgs = args
			return database.CommandTag{RowsAffected: 1}, nil
		},
	}
	svc := NewTenantServiceWithQuerier(q)

	err := svc.EnableSSO(context.Background(), "tenant-1", "okta", `{"domain":"acme.okta.com"}`)
	if err != nil {
		t.Fatalf("EnableSSO returned error: %v", err)
	}
	if capturedArgs[0].(string) != "okta" {
		t.Fatalf("expected provider 'okta', got %v", capturedArgs[0])
	}
	if capturedArgs[1].(string) != `{"domain":"acme.okta.com"}` {
		t.Fatalf("expected config JSON, got %v", capturedArgs[1])
	}
	if capturedArgs[2].(string) != "tenant-1" {
		t.Fatalf("expected tenant_id 'tenant-1', got %v", capturedArgs[2])
	}
}

func TestEnableSSO_DBError(t *testing.T) {
	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			return database.CommandTag{}, errors.New("update failed")
		},
	}
	svc := NewTenantServiceWithQuerier(q)

	err := svc.EnableSSO(context.Background(), "tenant-1", "okta", "{}")
	if err == nil {
		t.Fatal("expected error from DB")
	}
}

func TestCheckAgentLimit_UnderLimit(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanFn: func(dest ...interface{}) error {
				*(dest[0].(*int)) = 10
				*(dest[1].(*int)) = 3
				return nil
			}}
		},
	}
	svc := NewTenantServiceWithQuerier(q)

	ok, current, max, err := svc.CheckAgentLimit(context.Background(), "tenant-1")
	if err != nil {
		t.Fatalf("CheckAgentLimit returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected limit OK (3 < 10)")
	}
	if current != 3 {
		t.Fatalf("expected current 3, got %d", current)
	}
	if max != 10 {
		t.Fatalf("expected max 10, got %d", max)
	}
}

func TestCheckAgentLimit_AtLimit(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanFn: func(dest ...interface{}) error {
				*(dest[0].(*int)) = 10
				*(dest[1].(*int)) = 10
				return nil
			}}
		},
	}
	svc := NewTenantServiceWithQuerier(q)

	ok, current, max, err := svc.CheckAgentLimit(context.Background(), "tenant-1")
	if err != nil {
		t.Fatalf("CheckAgentLimit returned error: %v", err)
	}
	if ok {
		t.Fatal("expected limit NOT OK (10 == 10)")
	}
	if current != 10 {
		t.Fatalf("expected current 10, got %d", current)
	}
	if max != 10 {
		t.Fatalf("expected max 10, got %d", max)
	}
}

func TestCheckAgentLimit_OverLimit(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanFn: func(dest ...interface{}) error {
				*(dest[0].(*int)) = 5
				*(dest[1].(*int)) = 8
				return nil
			}}
		},
	}
	svc := NewTenantServiceWithQuerier(q)

	ok, _, _, err := svc.CheckAgentLimit(context.Background(), "tenant-1")
	if err != nil {
		t.Fatalf("CheckAgentLimit returned error: %v", err)
	}
	if ok {
		t.Fatal("expected limit NOT OK (8 > 5)")
	}
}

func TestCheckAgentLimit_DBError(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanErr: errors.New("tenant not found")}
		},
	}
	svc := NewTenantServiceWithQuerier(q)

	ok, current, max, err := svc.CheckAgentLimit(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("CheckAgentLimit should not return error on DB miss, got %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true on DB error (permissive default)")
	}
	if current != 0 {
		t.Fatalf("expected current 0, got %d", current)
	}
	if max != 0 {
		t.Fatalf("expected max 0, got %d", max)
	}
}

func TestCheckResourceLimit_UnderLimit(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanFn: func(dest ...interface{}) error {
				*(dest[0].(*int)) = 500
				*(dest[1].(*int)) = 100
				return nil
			}}
		},
	}
	svc := NewTenantServiceWithQuerier(q)

	ok, current, max, err := svc.CheckResourceLimit(context.Background(), "tenant-1")
	if err != nil {
		t.Fatalf("CheckResourceLimit returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected limit OK (100 < 500)")
	}
	if current != 100 {
		t.Fatalf("expected current 100, got %d", current)
	}
	if max != 500 {
		t.Fatalf("expected max 500, got %d", max)
	}
}

func TestCheckResourceLimit_AtLimit(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanFn: func(dest ...interface{}) error {
				*(dest[0].(*int)) = 500
				*(dest[1].(*int)) = 500
				return nil
			}}
		},
	}
	svc := NewTenantServiceWithQuerier(q)

	ok, _, _, err := svc.CheckResourceLimit(context.Background(), "tenant-1")
	if err != nil {
		t.Fatalf("CheckResourceLimit returned error: %v", err)
	}
	if ok {
		t.Fatal("expected limit NOT OK (500 == 500)")
	}
}

func TestCheckResourceLimit_DBError(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanErr: errors.New("not found")}
		},
	}
	svc := NewTenantServiceWithQuerier(q)

	ok, current, max, err := svc.CheckResourceLimit(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("CheckResourceLimit should not return error on DB miss, got %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true on DB error (permissive default)")
	}
	if current != 0 || max != 0 {
		t.Fatalf("expected 0,0 got %d,%d", current, max)
	}
}

func TestCreateApprover_Success(t *testing.T) {
	var capturedArgs []interface{}
	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			capturedArgs = args
			return database.CommandTag{RowsAffected: 1}, nil
		},
	}
	svc := NewTenantServiceWithQuerier(q)

	approver, err := svc.CreateApprover(context.Background(), "tenant-1", "alice@example.com", "Alice", "admin")
	if err != nil {
		t.Fatalf("CreateApprover returned error: %v", err)
	}
	if approver.ApproverID == "" {
		t.Fatal("expected auto-generated approver_id")
	}
	if len(approver.ApproverID) != 36 {
		t.Fatalf("expected UUID format approver_id, got %q", approver.ApproverID)
	}
	if approver.TenantID != "tenant-1" {
		t.Fatalf("expected tenant_id 'tenant-1', got %q", approver.TenantID)
	}
	if approver.Email != "alice@example.com" {
		t.Fatalf("expected email 'alice@example.com', got %q", approver.Email)
	}
	if approver.Name != "Alice" {
		t.Fatalf("expected name 'Alice', got %q", approver.Name)
	}
	if approver.Role != "admin" {
		t.Fatalf("expected role 'admin', got %q", approver.Role)
	}
	if !approver.IsActive {
		t.Fatal("expected is_active true for new approver")
	}
	if approver.CreatedAt.IsZero() {
		t.Fatal("expected non-zero created_at")
	}

	if len(capturedArgs) != 5 {
		t.Fatalf("expected 5 args, got %d", len(capturedArgs))
	}
}

func TestCreateApprover_DBError(t *testing.T) {
	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			return database.CommandTag{}, errors.New("foreign key violation")
		},
	}
	svc := NewTenantServiceWithQuerier(q)

	approver, err := svc.CreateApprover(context.Background(), "nonexistent-tenant", "x@x.com", "X", "viewer")
	if err == nil {
		t.Fatal("expected error from DB")
	}
	if approver != nil {
		t.Fatalf("expected nil approver on error, got %v", approver)
	}
	if err.Error() != "create approver: foreign key violation" {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

func TestListApprovers_Success(t *testing.T) {
	ts := now()
	approvers := []Approver{
		{
			ApproverID: "appr-1", TenantID: "tenant-1", Email: "a@a.com",
			Name: "Alice", Role: "admin", SSOSubject: "",
			NotificationChannel: "email", NotificationTarget: "a@a.com",
			IsActive: true, CreatedAt: ts,
		},
		{
			ApproverID: "appr-2", TenantID: "tenant-1", Email: "b@b.com",
			Name: "Bob", Role: "viewer", SSOSubject: "bob-sso",
			NotificationChannel: "slack", NotificationTarget: "#channel",
			IsActive: true, CreatedAt: ts,
		},
	}

	scanFns := make([]func(dest ...interface{}) error, len(approvers))
	for i, a := range approvers {
		scanFns[i] = newApproverRow(a)
	}

	q := &mockQuerier{
		queryFn: func(ctx context.Context, sql string, args ...interface{}) (database.Rows, error) {
			return &mockRows{items: scanFns}, nil
		},
	}
	svc := NewTenantServiceWithQuerier(q)

	result, err := svc.ListApprovers(context.Background(), "tenant-1")
	if err != nil {
		t.Fatalf("ListApprovers returned error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 approvers, got %d", len(result))
	}
	if result[0].Name != "Alice" {
		t.Fatalf("expected first approver 'Alice', got %q", result[0].Name)
	}
	if result[1].NotificationChannel != "slack" {
		t.Fatalf("expected second approver channel 'slack', got %q", result[1].NotificationChannel)
	}
}

func TestListApprovers_Empty(t *testing.T) {
	q := &mockQuerier{
		queryFn: func(ctx context.Context, sql string, args ...interface{}) (database.Rows, error) {
			return &mockRows{items: nil}, nil
		},
	}
	svc := NewTenantServiceWithQuerier(q)

	result, err := svc.ListApprovers(context.Background(), "tenant-1")
	if err != nil {
		t.Fatalf("ListApprovers returned error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil for empty list, got %v", result)
	}
}

func TestListApprovers_QueryError(t *testing.T) {
	q := &mockQuerier{
		queryFn: func(ctx context.Context, sql string, args ...interface{}) (database.Rows, error) {
			return nil, errors.New("connection refused")
		},
	}
	svc := NewTenantServiceWithQuerier(q)

	result, err := svc.ListApprovers(context.Background(), "tenant-1")
	if err == nil {
		t.Fatal("expected error from DB query")
	}
	if result != nil {
		t.Fatalf("expected nil result, got %v", result)
	}
}

func TestListApprovers_ScanErrorSkipsRow(t *testing.T) {
	ts := now()
	scanFns := []func(dest ...interface{}) error{
		func(dest ...interface{}) error { return errors.New("scan failed") },
		newApproverRow(Approver{
			ApproverID: "appr-ok", TenantID: "t1", Email: "ok@ok.com",
			Name: "OK", Role: "admin", SSOSubject: "",
			NotificationChannel: "", NotificationTarget: "",
			IsActive: true, CreatedAt: ts,
		}),
	}

	q := &mockQuerier{
		queryFn: func(ctx context.Context, sql string, args ...interface{}) (database.Rows, error) {
			return &mockRows{items: scanFns}, nil
		},
	}
	svc := NewTenantServiceWithQuerier(q)

	result, err := svc.ListApprovers(context.Background(), "t1")
	if err != nil {
		t.Fatalf("ListApprovers returned error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 approver (scan error skipped), got %d", len(result))
	}
	if result[0].ApproverID != "appr-ok" {
		t.Fatalf("expected approver appr-ok, got %q", result[0].ApproverID)
	}
}

func TestFindApproverBySSO_Success(t *testing.T) {
	expected := Approver{
		ApproverID: "appr-sso", TenantID: "tenant-1", Email: "sso@ok.com",
		Name: "SSO User", Role: "approver", SSOSubject: "user-sso-sub",
		NotificationChannel: "email", NotificationTarget: "sso@ok.com",
		IsActive: true,
	}

	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanFn: newApproverSSORow(expected)}
		},
	}
	svc := NewTenantServiceWithQuerier(q)

	approver, err := svc.FindApproverBySSO(context.Background(), "tenant-1", "user-sso-sub")
	if err != nil {
		t.Fatalf("FindApproverBySSO returned error: %v", err)
	}
	if approver.ApproverID != "appr-sso" {
		t.Fatalf("expected approver_id 'appr-sso', got %q", approver.ApproverID)
	}
	if approver.SSOSubject != "user-sso-sub" {
		t.Fatalf("expected sso_subject 'user-sso-sub', got %q", approver.SSOSubject)
	}
}

func TestFindApproverBySSO_NotFound(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanErr: errors.New("no rows in result set")}
		},
	}
	svc := NewTenantServiceWithQuerier(q)

	approver, err := svc.FindApproverBySSO(context.Background(), "tenant-1", "nonexistent-sub")
	if err == nil {
		t.Fatal("expected error for nonexistent SSO subject")
	}
	if approver != nil {
		t.Fatalf("expected nil approver, got %v", approver)
	}
	if err.Error() != "approver not found: no rows in result set" {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

func TestDeactivateApprover_Success(t *testing.T) {
	var capturedArgs []interface{}
	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			capturedArgs = args
			return database.CommandTag{RowsAffected: 1}, nil
		},
	}
	svc := NewTenantServiceWithQuerier(q)

	err := svc.DeactivateApprover(context.Background(), "appr-1")
	if err != nil {
		t.Fatalf("DeactivateApprover returned error: %v", err)
	}
	if capturedArgs[0].(string) != "appr-1" {
		t.Fatalf("expected approver_id 'appr-1', got %v", capturedArgs[0])
	}
}

func TestDeactivateApprover_DBError(t *testing.T) {
	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			return database.CommandTag{}, errors.New("update failed")
		},
	}
	svc := NewTenantServiceWithQuerier(q)

	err := svc.DeactivateApprover(context.Background(), "appr-1")
	if err == nil {
		t.Fatal("expected error from DB")
	}
}

func TestCancelledContext(t *testing.T) {
	ctxErr := context.Canceled
	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			return database.CommandTag{}, ctxErr
		},
	}
	svc := NewTenantServiceWithQuerier(q)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.CreateTenant(ctx, "Test", "test", "free", 1, 1)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}