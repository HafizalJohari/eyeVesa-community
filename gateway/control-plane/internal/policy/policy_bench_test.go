package policy

import (
	"context"
	"testing"
)

func BenchmarkLocalEvaluate(b *testing.B) {
	input := PolicyInput{}
	input.Agent.ID = "bench-agent"
	input.Agent.Owner = "bench-owner"
	input.Agent.TrustScore = 0.85
	input.Agent.AllowedTools = []string{"read", "write", "search"}
	input.Action.Tool = "read"
	input.Action.ResourceID = "doc-001"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		LocalEvaluate(input)
	}
}

func BenchmarkLocalEvaluateParallel(b *testing.B) {
	input := PolicyInput{}
	input.Agent.ID = "bench-agent"
	input.Agent.Owner = "bench-owner"
	input.Agent.TrustScore = 0.85
	input.Agent.AllowedTools = []string{"read", "write", "search"}
	input.Action.Tool = "read"
	input.Action.ResourceID = "doc-001"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			LocalEvaluate(input)
		}
	})
}

func BenchmarkEmbeddedOPAEvaluate(b *testing.B) {
	eopa, err := NewEmbeddedOPA("../../policies")
	if err != nil {
		b.Fatalf("Failed to create embedded OPA: %v", err)
	}

	input := PolicyInput{}
	input.Agent.ID = "bench-agent"
	input.Agent.Owner = "bench-owner"
	input.Agent.TrustScore = 0.85
	input.Agent.AllowedTools = []string{"read", "write", "search"}
	input.Action.Tool = "read"
	input.Action.ResourceID = "doc-001"

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		eopa.Evaluate(ctx, input)
	}
}

func BenchmarkEmbeddedOPAEvaluateParallel(b *testing.B) {
	eopa, err := NewEmbeddedOPA("../../policies")
	if err != nil {
		b.Fatalf("Failed to create embedded OPA: %v", err)
	}

	input := PolicyInput{}
	input.Agent.ID = "bench-agent"
	input.Agent.Owner = "bench-owner"
	input.Agent.TrustScore = 0.85
	input.Agent.AllowedTools = []string{"read", "write", "search"}
	input.Action.Tool = "read"

	ctx := context.Background()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			eopa.Evaluate(ctx, input)
		}
	})
}

func BenchmarkPolicyEngineReload(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		eng := NewPolicyEngine("../../policies", "")
		if eng.embeddedOPA == nil {
			b.Fatal("embedded OPA should not be nil")
		}
	}
}