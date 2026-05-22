package merchanttrust

import "testing"

func TestClamp(t *testing.T) {
	if got := clamp(-1, 0, 1); got != 0 {
		t.Fatalf("expected 0, got %f", got)
	}
	if got := clamp(2, 0, 1); got != 1 {
		t.Fatalf("expected 1, got %f", got)
	}
	if got := clamp(0.5, 0, 1); got != 0.5 {
		t.Fatalf("expected 0.5, got %f", got)
	}
}
