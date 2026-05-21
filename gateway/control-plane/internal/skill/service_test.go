package skill

import (
	"context"
	"errors"
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
	items  []func(dest ...interface{}) error
	idx    int
	closed bool
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

func ts() time.Time {
	return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
}

func TestNewSkillService_NilDB(t *testing.T) {
	svc := NewSkillService(nil)
	if svc == nil {
		t.Fatal("NewSkillService returned nil")
	}
	if svc.q != nil {
		t.Fatal("expected nil querier when db is nil")
	}
}

func TestNewSkillServiceWithQuerier(t *testing.T) {
	q := &mockQuerier{}
	svc := NewSkillServiceWithQuerier(q)
	if svc == nil {
		t.Fatal("NewSkillServiceWithQuerier returned nil")
	}
	if svc.q != q {
		t.Fatal("expected querier to be set")
	}
}

func TestCreateSkill_Success(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanFn: func(dest ...interface{}) error {
				*(dest[0].(*time.Time)) = ts()
				return nil
			}}
		},
	}
	svc := NewSkillServiceWithQuerier(q)

	sk, err := svc.CreateSkill(context.Background(), "kubernetes", "K8s management", "deployment", "high", 0.7, 3)
	if err != nil {
		t.Fatalf("CreateSkill returned error: %v", err)
	}
	if sk.Name != "kubernetes" {
		t.Fatalf("expected name 'kubernetes', got %q", sk.Name)
	}
	if sk.Category != "deployment" {
		t.Fatalf("expected category 'deployment', got %q", sk.Category)
	}
	if sk.RiskLevel != "high" {
		t.Fatalf("expected risk_level 'high', got %q", sk.RiskLevel)
	}
	if sk.RequiredTrustMin != 0.7 {
		t.Fatalf("expected required_trust_min 0.7, got %f", sk.RequiredTrustMin)
	}
	if sk.RequiredProficiency != 3 {
		t.Fatalf("expected required_proficiency 3, got %d", sk.RequiredProficiency)
	}
	if len(sk.SkillID) != 36 {
		t.Fatalf("expected UUID format skill_id, got %q", sk.SkillID)
	}
}

func TestCreateSkill_EmptyName(t *testing.T) {
	q := &mockQuerier{}
	svc := NewSkillServiceWithQuerier(q)

	_, err := svc.CreateSkill(context.Background(), "", "desc", "general", "low", 0.5, 1)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestCreateSkill_Defaults(t *testing.T) {
	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			return database.CommandTag{RowsAffected: 1}, nil
		},
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanFn: func(dest ...interface{}) error {
				*(dest[0].(*time.Time)) = ts()
				return nil
			}}
		},
	}
	svc := NewSkillServiceWithQuerier(q)

	sk, err := svc.CreateSkill(context.Background(), "database", "DB ops", "", "", 0, 0)
	if err != nil {
		t.Fatalf("CreateSkill returned error: %v", err)
	}
	if sk.Category != "general" {
		t.Fatalf("expected default category 'general', got %q", sk.Category)
	}
	if sk.RiskLevel != "medium" {
		t.Fatalf("expected default risk_level 'medium', got %q", sk.RiskLevel)
	}
	if sk.RequiredTrustMin != 0.5 {
		t.Fatalf("expected default required_trust_min 0.5, got %f", sk.RequiredTrustMin)
	}
	if sk.RequiredProficiency != 1 {
		t.Fatalf("expected default required_proficiency 1, got %d", sk.RequiredProficiency)
	}
}

func TestCreateSkill_DBError(t *testing.T) {
	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			return database.CommandTag{}, nil
		},
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanErr: errors.New("unique constraint violation")}
		},
	}
	svc := NewSkillServiceWithQuerier(q)

	_, err := svc.CreateSkill(context.Background(), "dup", "", "", "", 0, 0)
	if err == nil {
		t.Fatal("expected error from DB")
	}
}

func TestGetSkill_Success(t *testing.T) {
	expected := Skill{
		SkillID:            "skill-1",
		Name:               "kubernetes",
		Description:        "K8s management",
		Category:            "deployment",
		RiskLevel:           "high",
		RequiredTrustMin:    0.7,
		RequiredProficiency: 3,
		CreatedAt:           ts(),
		UpdatedAt:           ts(),
	}

	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanFn: func(dest ...interface{}) error {
				*(dest[0].(*string)) = expected.SkillID
				*(dest[1].(*string)) = expected.Name
				*(dest[2].(*string)) = expected.Description
				*(dest[3].(*string)) = expected.Category
				*(dest[4].(*string)) = expected.RiskLevel
				*(dest[5].(*float64)) = expected.RequiredTrustMin
				*(dest[6].(*int)) = expected.RequiredProficiency
				*(dest[7].(*time.Time)) = expected.CreatedAt
				*(dest[8].(*time.Time)) = expected.UpdatedAt
				return nil
			}}
		},
	}
	svc := NewSkillServiceWithQuerier(q)

	sk, err := svc.GetSkill(context.Background(), "skill-1")
	if err != nil {
		t.Fatalf("GetSkill returned error: %v", err)
	}
	if sk.Name != "kubernetes" {
		t.Fatalf("expected name 'kubernetes', got %q", sk.Name)
	}
}

func TestGetSkill_NotFound(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanErr: errors.New("no rows in result set")}
		},
	}
	svc := NewSkillServiceWithQuerier(q)

	_, err := svc.GetSkill(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent skill")
	}
}

func TestGetSkillByName_Success(t *testing.T) {
	expected := Skill{
		SkillID:            "skill-k8s",
		Name:               "kubernetes",
		Description:        "K8s management",
		Category:            "deployment",
		RiskLevel:           "high",
		RequiredTrustMin:    0.7,
		RequiredProficiency: 3,
		CreatedAt:           ts(),
		UpdatedAt:           ts(),
	}

	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanFn: func(dest ...interface{}) error {
				*(dest[0].(*string)) = expected.SkillID
				*(dest[1].(*string)) = expected.Name
				*(dest[2].(*string)) = expected.Description
				*(dest[3].(*string)) = expected.Category
				*(dest[4].(*string)) = expected.RiskLevel
				*(dest[5].(*float64)) = expected.RequiredTrustMin
				*(dest[6].(*int)) = expected.RequiredProficiency
				*(dest[7].(*time.Time)) = expected.CreatedAt
				*(dest[8].(*time.Time)) = expected.UpdatedAt
				return nil
			}}
		},
	}
	svc := NewSkillServiceWithQuerier(q)

	sk, err := svc.GetSkillByName(context.Background(), "kubernetes")
	if err != nil {
		t.Fatalf("GetSkillByName returned error: %v", err)
	}
	if sk.SkillID != "skill-k8s" {
		t.Fatalf("expected skill_id 'skill-k8s', got %q", sk.SkillID)
	}
}

func TestListSkills_Empty(t *testing.T) {
	q := &mockQuerier{
		queryFn: func(ctx context.Context, sql string, args ...interface{}) (database.Rows, error) {
			return &mockRows{items: nil}, nil
		},
	}
	svc := NewSkillServiceWithQuerier(q)

	skills, err := svc.ListSkills(context.Background(), "")
	if err != nil {
		t.Fatalf("ListSkills returned error: %v", err)
	}
	if skills != nil {
		t.Fatalf("expected nil for empty list, got %v", skills)
	}
}

func TestDeleteSkill_Success(t *testing.T) {
	var capturedSQL string
	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			capturedSQL = sql
			return database.CommandTag{RowsAffected: 1}, nil
		},
	}
	svc := NewSkillServiceWithQuerier(q)

	err := svc.DeleteSkill(context.Background(), "skill-1")
	if err != nil {
		t.Fatalf("DeleteSkill returned error: %v", err)
	}
	if capturedSQL == "" {
		t.Fatal("expected DELETE to be executed")
	}
}

func TestDeleteSkill_DBError(t *testing.T) {
	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			return database.CommandTag{}, errors.New("fk violation")
		},
	}
	svc := NewSkillServiceWithQuerier(q)

	err := svc.DeleteSkill(context.Background(), "skill-1")
	if err == nil {
		t.Fatal("expected error from DB")
	}
}

func TestAssignSkill_Success(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanFn: func(dest ...interface{}) error {
				*(dest[0].(*time.Time)) = ts()
				return nil
			}}
		},
	}
	svc := NewSkillServiceWithQuerier(q)

	as, err := svc.AssignSkill(context.Background(), "agent-1", "skill-1", 3)
	if err != nil {
		t.Fatalf("AssignSkill returned error: %v", err)
	}
	if as.AgentID != "agent-1" {
		t.Fatalf("expected agent_id 'agent-1', got %q", as.AgentID)
	}
	if as.SkillID != "skill-1" {
		t.Fatalf("expected skill_id 'skill-1', got %q", as.SkillID)
	}
	if as.Proficiency != 3 {
		t.Fatalf("expected proficiency 3, got %d", as.Proficiency)
	}
}

func TestAssignSkill_ClampsProficiency(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanFn: func(dest ...interface{}) error {
				*(dest[0].(*time.Time)) = ts()
				return nil
			}}
		},
	}
	svc := NewSkillServiceWithQuerier(q)

	as, err := svc.AssignSkill(context.Background(), "agent-1", "skill-1", 10)
	if err != nil {
		t.Fatalf("AssignSkill returned error: %v", err)
	}
	if as.Proficiency != 5 {
		t.Fatalf("expected proficiency clamped to 5, got %d", as.Proficiency)
	}

	as0, err := svc.AssignSkill(context.Background(), "agent-1", "skill-1", 0)
	if err != nil {
		t.Fatalf("AssignSkill returned error: %v", err)
	}
	if as0.Proficiency != 1 {
		t.Fatalf("expected proficiency clamped to 1, got %d", as0.Proficiency)
	}
}

func TestRemoveSkill_Success(t *testing.T) {
	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			return database.CommandTag{RowsAffected: 1}, nil
		},
	}
	svc := NewSkillServiceWithQuerier(q)

	err := svc.RemoveSkill(context.Background(), "agent-1", "skill-1")
	if err != nil {
		t.Fatalf("RemoveSkill returned error: %v", err)
	}
}

func TestVerifySkill_Success(t *testing.T) {
	skillScanFns := []func(dest ...interface{}) error{
		func(dest ...interface{}) error {
			*(dest[0].(*string)) = "agent-1"
			*(dest[1].(*string)) = "skill-1"
			*(dest[2].(*string)) = "kubernetes"
			*(dest[3].(*int)) = 3
			*(dest[4].(*bool)) = true
			*(dest[5].(*string)) = "admin@example.com"
			verifiedAt := ts()
			*(dest[6].(**time.Time)) = &verifiedAt
			*(dest[7].(*int)) = 2
			*(dest[8].(*time.Time)) = ts()
			return nil
		},
	}

	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			return database.CommandTag{RowsAffected: 1}, nil
		},
		queryFn: func(ctx context.Context, sql string, args ...interface{}) (database.Rows, error) {
			return &mockRows{items: skillScanFns}, nil
		},
	}
	svc := NewSkillServiceWithQuerier(q)

	as, err := svc.VerifySkill(context.Background(), "agent-1", "skill-1", "admin@example.com")
	if err != nil {
		t.Fatalf("VerifySkill returned error: %v", err)
	}
	if !as.Verified {
		t.Fatal("expected verified=true")
	}
	if as.VerifiedBy != "admin@example.com" {
		t.Fatalf("expected verified_by 'admin@example.com', got %q", as.VerifiedBy)
	}
}

func TestEndorseSkill_InvalidType(t *testing.T) {
	q := &mockQuerier{}
	svc := NewSkillServiceWithQuerier(q)

	_, err := svc.EndorseSkill(context.Background(), "agent-1", "skill-1", "invalid", "user-1", "comment")
	if err == nil {
		t.Fatal("expected error for invalid endorser_type")
	}
}

func TestEndorseSkill_Success(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanFn: func(dest ...interface{}) error {
				*(dest[0].(*time.Time)) = ts()
				return nil
			}}
		},
	}
	svc := NewSkillServiceWithQuerier(q)

	e, err := svc.EndorseSkill(context.Background(), "agent-1", "skill-1", "human", "user-1", "great work")
	if err != nil {
		t.Fatalf("EndorseSkill returned error: %v", err)
	}
	if e.EndorserType != "human" {
		t.Fatalf("expected endorser_type 'human', got %q", e.EndorserType)
	}
	if e.EndorserID != "user-1" {
		t.Fatalf("expected endorser_id 'user-1', got %q", e.EndorserID)
	}
	if e.Comment != "great work" {
		t.Fatalf("expected comment 'great work', got %q", e.Comment)
	}
	if len(e.EndorsementID) != 36 {
		t.Fatalf("expected UUID format endorsement_id, got %q", e.EndorsementID)
	}
}

func TestGetSkillTrust_NotFound(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanErr: errors.New("no rows in result set")}
		},
	}
	svc := NewSkillServiceWithQuerier(q)

	_, err := svc.GetSkillTrust(context.Background(), "agent-1", "skill-1")
	if err == nil {
		t.Fatal("expected error for missing trust score")
	}
}

func TestUpdateSkillTrust_Success(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanFn: func(dest ...interface{}) error {
				*(dest[0].(*float64)) = 0.8
				return nil
			}}
		},
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			return database.CommandTag{RowsAffected: 1}, nil
		},
	}
	svc := NewSkillServiceWithQuerier(q)

	newScore, err := svc.UpdateSkillTrust(context.Background(), "agent-1", "skill-1", 0.05)
	if err != nil {
		t.Fatalf("UpdateSkillTrust returned error: %v", err)
	}
	if newScore < 0 || newScore > 1 {
		t.Fatalf("expected clamped score, got %f", newScore)
	}
}

func TestFindMissingSkills_Empty(t *testing.T) {
	q := &mockQuerier{
		queryFn: func(ctx context.Context, sql string, args ...interface{}) (database.Rows, error) {
			return &mockRows{items: nil}, nil
		},
	}
	svc := NewSkillServiceWithQuerier(q)

	missing, err := svc.FindMissingSkills(context.Background(), "agent-1", nil)
	if err != nil {
		t.Fatalf("FindMissingSkills returned error: %v", err)
	}
	if missing != nil {
		t.Fatalf("expected nil for empty required, got %v", missing)
	}
}

func TestFindMissingSkills_WithMissingSkills(t *testing.T) {
	scanFns := []func(dest ...interface{}) error{
		func(dest ...interface{}) error {
			*(dest[0].(*string)) = "kubernetes"
			return nil
		},
	}

	q := &mockQuerier{
		queryFn: func(ctx context.Context, sql string, args ...interface{}) (database.Rows, error) {
			return &mockRows{items: scanFns}, nil
		},
	}
	svc := NewSkillServiceWithQuerier(q)

	missing, err := svc.FindMissingSkills(context.Background(), "agent-1", []string{"kubernetes", "database"})
	if err != nil {
		t.Fatalf("FindMissingSkills returned error: %v", err)
	}
	if len(missing) != 1 {
		t.Fatalf("expected 1 missing skill, got %d", len(missing))
	}
	if missing[0] != "kubernetes" {
		t.Fatalf("expected missing skill 'kubernetes', got %q", missing[0])
	}
}

func TestTrustAdjustment_ClampTrust(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{1.5, 1.0},
		{-0.5, 0.0},
		{0.5, 0.5},
		{0.0, 0.0},
		{1.0, 1.0},
	}
	for _, tt := range tests {
		result := ClampTrust(tt.input)
		if result != tt.expected {
			t.Fatalf("ClampTrust(%f) = %f, expected %f", tt.input, result, tt.expected)
		}
	}
}

func TestCheckSkillAuthorization_NoSkillRow(t *testing.T) {
	q := &mockQuerier{
		queryFn: func(ctx context.Context, sql string, args ...interface{}) (database.Rows, error) {
			return &mockRows{items: nil}, nil
		},
	}
	svc := NewSkillServiceWithQuerier(q)

	allowed, reasons, trust, err := svc.CheckSkillAuthorization(context.Background(), "agent-1", "unknown_action")
	if err != nil {
		t.Fatalf("CheckSkillAuthorization returned error: %v", err)
	}
	if !allowed {
		t.Fatal("expected allowed=true when no skill row exists (no skill gating)")
	}
	if reasons != nil {
		t.Fatalf("expected no reasons, got %v", reasons)
	}
	if trust != 1.0 {
		t.Fatalf("expected default trust 1.0, got %f", trust)
	}
}