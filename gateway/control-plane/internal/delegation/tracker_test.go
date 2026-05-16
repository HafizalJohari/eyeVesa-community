package delegation

import (
	"testing"
)

func TestNewDelegationTracker(t *testing.T) {
	tracker := NewDelegationTracker(nil, nil)
	if tracker == nil {
		t.Fatal("NewDelegationTracker returned nil")
	}
}