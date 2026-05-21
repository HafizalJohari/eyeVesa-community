package audit

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/database"
)

type mockQuerier struct {
	execFn      func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error)
	queryRowFn  func(ctx context.Context, sql string, args ...interface{}) database.Row
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
	return nil, nil
}

type mockRow struct {
	scanErr error
}

func (r *mockRow) Scan(dest ...interface{}) error {
	return r.scanErr
}

func TestNewAuditLogger(t *testing.T) {
	logger := NewAuditLogger(nil)
	if logger == nil {
		t.Fatal("NewAuditLogger returned nil")
	}
	if logger.q != nil {
		t.Fatal("expected nil querier when db is nil")
	}
}

func TestNewAuditLoggerWithQuerier(t *testing.T) {
	q := &mockQuerier{}
	logger := NewAuditLoggerWithQuerier(q)
	if logger == nil {
		t.Fatal("NewAuditLoggerWithQuerier returned nil")
	}
	if logger.q != q {
		t.Fatal("expected querier to be set")
	}
}

func TestLog_GeneratesLogID(t *testing.T) {
	var capturedArgs []interface{}
	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			capturedArgs = args
			return database.CommandTag{}, nil
		},
	}
	logger := NewAuditLoggerWithQuerier(q)

	entry := AuditEntry{
		AgentID: "agent-1",
		Action:  "register",
		Method:  "POST",
		Status:  "success",
	}

	err := logger.Log(context.Background(), entry, nil)
	if err != nil {
		t.Fatalf("Log returned error: %v", err)
	}

	generatedID := capturedArgs[0].(string)
	if generatedID == "" {
		t.Fatal("expected auto-generated log_id but got empty string")
	}
	if len(generatedID) != 36 {
		t.Fatalf("expected UUID format, got %q", generatedID)
	}
}

func TestLog_PreservesExistingLogID(t *testing.T) {
	var capturedArgs []interface{}
	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			capturedArgs = args
			return database.CommandTag{}, nil
		},
	}
	logger := NewAuditLoggerWithQuerier(q)

	entry := AuditEntry{
		LogID:   "custom-log-id",
		AgentID: "agent-1",
		Action:  "register",
		Method:  "POST",
		Status:  "success",
	}

	err := logger.Log(context.Background(), entry, nil)
	if err != nil {
		t.Fatalf("Log returned error: %v", err)
	}

	if capturedArgs[0].(string) != "custom-log-id" {
		t.Fatalf("expected custom-log-id, got %v", capturedArgs[0])
	}
}

func TestLog_WithSigningKey(t *testing.T) {
	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	var capturedArgs []interface{}
	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			capturedArgs = args
			return database.CommandTag{}, nil
		},
	}
	logger := NewAuditLoggerWithQuerier(q)

	entry := AuditEntry{
		LogID:      "log-123",
		AgentID:    "agent-1",
		ResourceID: "res-1",
		Action:     "register",
		Method:     "POST",
		Status:     "success",
	}

	err = logger.Log(context.Background(), entry, privKey)
	if err != nil {
		t.Fatalf("Log returned error: %v", err)
	}

	signature := capturedArgs[11].([]byte)
	if len(signature) != ed25519.SignatureSize {
		t.Fatalf("expected %d byte signature, got %d", ed25519.SignatureSize, len(signature))
	}

	payload := fmt.Sprintf("%s:%s:%s:%s:%s:%s",
		entry.LogID, entry.AgentID, entry.ResourceID,
		entry.Action, entry.Method, entry.Status)
	hash := sha256.Sum256([]byte(payload))

	if !ed25519.Verify(pubKey, hash[:], signature) {
		t.Fatal("signature verification failed")
	}
}

func TestLog_WithNilSigningKey(t *testing.T) {
	var capturedArgs []interface{}
	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			capturedArgs = args
			return database.CommandTag{}, nil
		},
	}
	logger := NewAuditLoggerWithQuerier(q)

	entry := AuditEntry{
		LogID:   "log-456",
		AgentID: "agent-2",
		Action:  "delete",
		Method:  "DELETE",
		Status:  "denied",
	}

	err := logger.Log(context.Background(), entry, nil)
	if err != nil {
		t.Fatalf("Log returned error: %v", err)
	}

	sig := capturedArgs[11]
	if sig != nil {
		sigSlice, ok := sig.([]byte)
		if !ok || len(sigSlice) != 0 {
			t.Fatalf("expected nil/empty signature when no key provided, got %v", sig)
		}
	}
}

func TestLog_DBError(t *testing.T) {
	dbErr := errors.New("connection refused")
	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			return database.CommandTag{}, dbErr
		},
	}
	logger := NewAuditLoggerWithQuerier(q)

	entry := AuditEntry{
		AgentID: "agent-1",
		Action:  "register",
		Method:  "POST",
		Status:  "success",
	}

	err := logger.Log(context.Background(), entry, nil)
	if err == nil {
		t.Fatal("expected error from DB, got nil")
	}
	if err.Error() != "connection refused" {
		t.Fatalf("expected DB error, got %v", err)
	}
}

func TestLog_NilIfEmptyFields(t *testing.T) {
	var capturedArgs []interface{}
	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			capturedArgs = args
			return database.CommandTag{}, nil
		},
	}
	logger := NewAuditLoggerWithQuerier(q)

	entry := AuditEntry{
		LogID:      "log-789",
		AgentID:    "agent-1",
		Action:     "read",
		Method:     "GET",
		Status:     "success",
	}

	err := logger.Log(context.Background(), entry, nil)
	if err != nil {
		t.Fatalf("Log returned error: %v", err)
	}

	resourceID := capturedArgs[2]
	if resourceID != nil {
		t.Fatalf("expected nil for empty resource_id, got %v", resourceID)
	}

	sessionID := capturedArgs[10]
	if sessionID != nil {
		t.Fatalf("expected nil for empty session_id, got %v", sessionID)
	}
}

func TestLog_NonEmptyResourceAndSession(t *testing.T) {
	var capturedArgs []interface{}
	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			capturedArgs = args
			return database.CommandTag{}, nil
		},
	}
	logger := NewAuditLoggerWithQuerier(q)

	entry := AuditEntry{
		LogID:      "log-abc",
		AgentID:    "agent-1",
		ResourceID: "res-99",
		Action:     "update",
		Method:     "PUT",
		Status:     "success",
		SessionID:  "sess-42",
	}

	err := logger.Log(context.Background(), entry, nil)
	if err != nil {
		t.Fatalf("Log returned error: %v", err)
	}

	if capturedArgs[2].(string) != "res-99" {
		t.Fatalf("expected res-99, got %v", capturedArgs[2])
	}
	if capturedArgs[10].(string) != "sess-42" {
		t.Fatalf("expected sess-42, got %v", capturedArgs[10])
	}
}

func TestLog_ParamsAndResultJSON(t *testing.T) {
	var capturedArgs []interface{}
	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			capturedArgs = args
			return database.CommandTag{}, nil
		},
	}
	logger := NewAuditLoggerWithQuerier(q)

	entry := AuditEntry{
		LogID:   "log-json",
		AgentID: "agent-1",
		Action:  "register",
		Method:  "POST",
		Status:  "success",
		Params:  map[string]interface{}{"key": "value", "num": 42},
		Result:  map[string]interface{}{"ok": true},
	}

	err := logger.Log(context.Background(), entry, nil)
	if err != nil {
		t.Fatalf("Log returned error: %v", err)
	}

	paramsJSON := string(capturedArgs[5].([]byte))
	resultJSON := string(capturedArgs[6].([]byte))

	if paramsJSON == "" || paramsJSON == "null" {
		t.Fatalf("expected params JSON, got %q", paramsJSON)
	}
	if resultJSON == "" || resultJSON == "null" {
		t.Fatalf("expected result JSON, got %q", resultJSON)
	}
}

func TestLog_NilParamsAndResult(t *testing.T) {
	var capturedArgs []interface{}
	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			capturedArgs = args
			return database.CommandTag{}, nil
		},
	}
	logger := NewAuditLoggerWithQuerier(q)

	entry := AuditEntry{
		LogID:  "log-nil-params",
		Action: "read",
		Status: "success",
	}

	err := logger.Log(context.Background(), entry, nil)
	if err != nil {
		t.Fatalf("Log returned error: %v", err)
	}

	paramsJSON := string(capturedArgs[5].([]byte))
	resultJSON := string(capturedArgs[6].([]byte))

	if paramsJSON != "null" {
		t.Fatalf("expected null for nil params, got %q", paramsJSON)
	}
	if resultJSON != "null" {
		t.Fatalf("expected null for nil result, got %q", resultJSON)
	}
}

func TestLog_TrustScores(t *testing.T) {
	var capturedArgs []interface{}
	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			capturedArgs = args
			return database.CommandTag{}, nil
		},
	}
	logger := NewAuditLoggerWithQuerier(q)

	entry := AuditEntry{
		LogID:       "log-trust",
		AgentID:     "agent-1",
		Action:      "register",
		Method:      "POST",
		Status:      "success",
		TrustBefore: 0.75,
		TrustAfter:  0.80,
	}

	err := logger.Log(context.Background(), entry, nil)
	if err != nil {
		t.Fatalf("Log returned error: %v", err)
	}

	if capturedArgs[8].(float64) != 0.75 {
		t.Fatalf("expected trust_before 0.75, got %v", capturedArgs[8])
	}
	if capturedArgs[9].(float64) != 0.80 {
		t.Fatalf("expected trust_after 0.80, got %v", capturedArgs[9])
	}
}

func TestLog_CancelledContext(t *testing.T) {
	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			return database.CommandTag{}, ctx.Err()
		},
	}
	logger := NewAuditLoggerWithQuerier(q)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	entry := AuditEntry{LogID: "log-cancel", AgentID: "a", Action: "x", Method: "GET", Status: "ok"}

	err := logger.Log(ctx, entry, nil)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestComputeSignature(t *testing.T) {
	_, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	logger := &AuditLogger{}

	entry := AuditEntry{
		LogID:      "sig-test",
		AgentID:    "agent-1",
		ResourceID: "res-1",
		Action:     "register",
		Method:     "POST",
		Status:     "success",
	}

	sig, err := logger.computeSignature(entry, privKey)
	if err != nil {
		t.Fatalf("computeSignature returned error: %v", err)
	}
	if len(sig) != ed25519.SignatureSize {
		t.Fatalf("expected %d bytes, got %d", ed25519.SignatureSize, len(sig))
	}
}

func TestComputeSignature_Deterministic(t *testing.T) {
	_, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	logger := &AuditLogger{}

	entry := AuditEntry{
		LogID:      "sig-det",
		AgentID:    "agent-1",
		ResourceID: "res-1",
		Action:     "register",
		Method:     "POST",
		Status:     "success",
	}

	sig1, _ := logger.computeSignature(entry, privKey)
	sig2, _ := logger.computeSignature(entry, privKey)

	if string(sig1) != string(sig2) {
		t.Fatal("signatures for same entry should be identical")
	}
}

func TestComputeSignature_DifferentEntries(t *testing.T) {
	_, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	logger := &AuditLogger{}

	entry1 := AuditEntry{
		LogID: "sig-1", AgentID: "a1", ResourceID: "r1",
		Action: "register", Method: "POST", Status: "success",
	}
	entry2 := AuditEntry{
		LogID: "sig-2", AgentID: "a2", ResourceID: "r2",
		Action: "delete", Method: "DELETE", Status: "denied",
	}

	sig1, _ := logger.computeSignature(entry1, privKey)
	sig2, _ := logger.computeSignature(entry2, privKey)

	if string(sig1) == string(sig2) {
		t.Fatal("different entries should produce different signatures")
	}
}

func TestVerifyIntegrity_Valid(t *testing.T) {
	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	entry := AuditEntry{
		LogID:      "verify-valid",
		AgentID:    "agent-1",
		ResourceID: "res-1",
		Action:     "register",
		Method:     "POST",
		Status:     "success",
	}

	sig, _ := (&AuditLogger{}).computeSignature(entry, privKey)

	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &verifyRow{
				agentID:    entry.AgentID,
				resourceID: entry.ResourceID,
				action:     entry.Action,
				method:     entry.Method,
				status:     entry.Status,
				signature:  sig,
			}
		},
	}

	logger := NewAuditLoggerWithQuerier(q)
	valid, err := logger.VerifyIntegrity(context.Background(), entry.LogID, pubKey)
	if err != nil {
		t.Fatalf("VerifyIntegrity error: %v", err)
	}
	if !valid {
		t.Fatal("expected valid signature")
	}
}

func TestVerifyIntegrity_Invalid(t *testing.T) {
	_, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	otherPub, _, _ := ed25519.GenerateKey(nil)

	entry := AuditEntry{
		LogID:      "verify-invalid",
		AgentID:    "agent-1",
		ResourceID: "res-1",
		Action:     "register",
		Method:     "POST",
		Status:     "success",
	}

	sig, _ := (&AuditLogger{}).computeSignature(entry, privKey)

	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &verifyRow{
				agentID:    entry.AgentID,
				resourceID: entry.ResourceID,
				action:     entry.Action,
				method:     entry.Method,
				status:     entry.Status,
				signature:  sig,
			}
		},
	}

	logger := NewAuditLoggerWithQuerier(q)
	valid, err := logger.VerifyIntegrity(context.Background(), entry.LogID, otherPub)
	if err != nil {
		t.Fatalf("VerifyIntegrity error: %v", err)
	}
	if valid {
		t.Fatal("expected invalid signature with wrong public key")
	}
}

func TestVerifyIntegrity_DBError(t *testing.T) {
	pubKey, _, _ := ed25519.GenerateKey(nil)

	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanErr: errors.New("not found")}
		},
	}

	logger := NewAuditLoggerWithQuerier(q)
	valid, err := logger.VerifyIntegrity(context.Background(), "missing-id", pubKey)
	if err == nil {
		t.Fatal("expected error from DB")
	}
	if valid {
		t.Fatal("expected false on DB error")
	}
}

func TestVerifyIntegrity_TamperedData(t *testing.T) {
	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	origEntry := AuditEntry{
		LogID:      "verify-tampered",
		AgentID:    "agent-1",
		ResourceID: "res-1",
		Action:     "register",
		Method:     "POST",
		Status:     "success",
	}

	sig, _ := (&AuditLogger{}).computeSignature(origEntry, privKey)

	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &verifyRow{
				agentID:    "agent-TAMPERED",
				resourceID: origEntry.ResourceID,
				action:     origEntry.Action,
				method:     origEntry.Method,
				status:     origEntry.Status,
				signature:  sig,
			}
		},
	}

	logger := NewAuditLoggerWithQuerier(q)
	valid, err := logger.VerifyIntegrity(context.Background(), origEntry.LogID, pubKey)
	if err != nil {
		t.Fatalf("VerifyIntegrity error: %v", err)
	}
	if valid {
		t.Fatal("expected invalid for tampered data")
	}
}

type verifyRow struct {
	agentID    string
	resourceID string
	action     string
	method     string
	status     string
	signature  []byte
}

func (r *verifyRow) Scan(dest ...interface{}) error {
	if len(dest) != 6 {
		return fmt.Errorf("expected 6 scan destinations, got %d", len(dest))
	}
	*(dest[0].(*string)) = r.agentID
	*(dest[1].(*string)) = r.resourceID
	*(dest[2].(*string)) = r.action
	*(dest[3].(*string)) = r.method
	*(dest[4].(*string)) = r.status
	*(dest[5].(*[]byte)) = r.signature
	return nil
}

func TestNilIfEmpty(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{"", nil},
		{"some-value", "some-value"},
		{" ", " "},
	}

	for _, tc := range tests {
		result := nilIfEmpty(tc.input)
		if result != tc.expected {
			t.Errorf("nilIfEmpty(%q) = %v, want %v", tc.input, result, tc.expected)
		}
	}
}

func TestNow(t *testing.T) {
	ts := Now()
	if ts == "" {
		t.Fatal("Now() returned empty string")
	}

	_, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		t.Fatalf("Now() returned invalid RFC3339: %v", err)
	}
}

func TestComputeLogHash_Empty(t *testing.T) {
	h := ComputeLogHash(nil)
	if h == "" {
		t.Fatal("expected non-empty hash for empty input")
	}

	h2 := ComputeLogHash([]AuditEntry{})
	if h != h2 {
		t.Fatal("nil and empty slice should produce same hash")
	}
}

func TestComputeLogHash_SingleEntry(t *testing.T) {
	entries := []AuditEntry{
		{LogID: "id1", AgentID: "a1", Action: "act1", Status: "ok"},
	}

	h := ComputeLogHash(entries)
	if h == "" {
		t.Fatal("expected non-empty hash")
	}

	decoded, err := hex.DecodeString(h)
	if err != nil {
		t.Fatalf("hash is not valid hex: %v", err)
	}
	if len(decoded) != sha256.Size {
		t.Fatalf("expected %d byte hash, got %d", sha256.Size, len(decoded))
	}
}

func TestComputeLogHash_MultipleEntries(t *testing.T) {
	entries := []AuditEntry{
		{LogID: "id1", AgentID: "a1", Action: "act1", Status: "ok"},
		{LogID: "id2", AgentID: "a2", Action: "act2", Status: "fail"},
	}

	h1 := ComputeLogHash(entries)
	h2 := ComputeLogHash(entries[:1])

	if h1 == h2 {
		t.Fatal("different entry sets should produce different hashes")
	}
}

func TestComputeLogHash_Deterministic(t *testing.T) {
	entries := []AuditEntry{
		{LogID: "id1", AgentID: "a1", Action: "act1", Status: "ok"},
		{LogID: "id2", AgentID: "a2", Action: "act2", Status: "fail"},
	}

	h1 := ComputeLogHash(entries)
	h2 := ComputeLogHash(entries)

	if h1 != h2 {
		t.Fatal("same entries should produce same hash")
	}
}

func TestComputeLogHash_OnlyUsesKeyFields(t *testing.T) {
	base := AuditEntry{LogID: "id1", AgentID: "a1", Action: "act1", Status: "ok"}

	entrySame := AuditEntry{
		LogID: base.LogID, AgentID: base.AgentID,
		Action: base.Action, Status: base.Status,
		ResourceID: "different", Method: "PUT",
		Params: map[string]interface{}{"extra": true},
		Result: map[string]interface{}{"extra": true},
		TrustBefore: 0.5, TrustAfter: 0.9, SessionID: "sess-1",
	}

	h1 := ComputeLogHash([]AuditEntry{base})
	h2 := ComputeLogHash([]AuditEntry{entrySame})

	if h1 != h2 {
		t.Fatal("hash should only depend on LogID, AgentID, Action, Status")
	}
}

func TestAuditEntry_JSONTags(t *testing.T) {
	entry := AuditEntry{
		LogID:       "json-test",
		AgentID:     "agent-1",
		ResourceID:  "res-1",
		Action:      "register",
		Method:      "POST",
		Params:      map[string]interface{}{"k": "v"},
		Result:      map[string]interface{}{"ok": true},
		Status:      "success",
		TrustBefore: 0.5,
		TrustAfter:  0.9,
		SessionID:   "sess-1",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	s := string(data)
	for _, field := range []string{
		`"log_id"`, `"agent_id"`, `"resource_id"`, `"action"`, `"method"`,
		`"params"`, `"result"`, `"result_status"`, `"trust_score_before"`,
		`"trust_score_after"`, `"session_id"`,
	} {
		if !contains(s, field) {
			t.Errorf("missing JSON field %q in output: %s", field, s)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}