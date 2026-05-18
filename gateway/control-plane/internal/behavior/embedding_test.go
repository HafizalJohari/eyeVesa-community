package behavior

import (
	"testing"
)

func TestFormatVector(t *testing.T) {
	tests := []struct {
		vec      []float32
		expected string
	}{
		{[]float32{1.0, 2.0, 3.0}, "[1.000000,2.000000,3.000000]"},
		{[]float32{}, "[]"},
		{[]float32{0.5}, "[0.500000]"},
	}
	for _, tt := range tests {
		got := formatVector(tt.vec)
		if got != tt.expected {
			t.Errorf("formatVector(%v) = %q, want %q", tt.vec, got, tt.expected)
		}
	}
}

func TestNilIfEmpty(t *testing.T) {
	if nilIfEmpty("") != nil {
		t.Fatal("empty string should return nil")
	}
	if nilIfEmpty("value") != "value" {
		t.Fatal("non-empty string should return the string")
	}
}

func TestNewEmbeddingService(t *testing.T) {
	svc := NewEmbeddingService(nil, nil)
	if svc == nil {
		t.Fatal("NewEmbeddingService returned nil")
	}
	if svc.vecDim != 1536 {
		t.Fatalf("vecDim should be 1536, got %d", svc.vecDim)
	}
}

func TestBehavioralAnomaly_Fields(t *testing.T) {
	a := BehavioralAnomaly{
		AnomalyID:        "anom-1",
		AgentID:         "a1",
		SimilarityScore: 0.65,
		BaselineBehavior: "baseline",
		DetectedBehavior: "unusual",
		AnomalyType:     "drift",
	}
	if a.AnomalyID != "anom-1" {
		t.Fatalf("AnomalyID mismatch: got %s", a.AnomalyID)
	}
	if a.SimilarityScore != 0.65 {
		t.Fatalf("SimilarityScore mismatch: got %f", a.SimilarityScore)
	}
}