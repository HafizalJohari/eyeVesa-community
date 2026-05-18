package hitl

import (
	"testing"
)

func BenchmarkDetermineEscalationLevel(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DetermineEscalationLevel(0.85, "read", nil, "low")
	}
}

func BenchmarkDetermineEscalationLevelHighRisk(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DetermineEscalationLevel(0.4, "bank_transfer", map[string]interface{}{"amount": float64(2000)}, "high")
	}
}

func BenchmarkTrustDeltaForDecision(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = TrustDeltaForDecision("approved")
	}
}

func BenchmarkRequiredApprovals(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = RequiredApprovals(LevelHITL)
	}
}