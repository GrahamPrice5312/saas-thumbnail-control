package thumbnail

import "testing"

func TestThumbnailPlanFollowsAccountLifecycle(t *testing.T) {
	tests := []struct {
		name        string
		transitions []AccountState
		wantAllowed bool
	}{
		{name: "onboarding tenant is gated", wantAllowed: false},
		{name: "active tenant gets responsive set", transitions: []AccountState{StateActive}, wantAllowed: true},
		{name: "suspended tenant is gated", transitions: []AccountState{StateActive, StateSuspended}, wantAllowed: false},
		{name: "reactivated tenant gets responsive set", transitions: []AccountState{StateActive, StateSuspended, StateActive}, wantAllowed: true},
		{name: "closed tenant is gated", transitions: []AccountState{StateClosed}, wantAllowed: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRegistry()
			if err := r.Onboard("acme"); err != nil {
				t.Fatal(err)
			}
			for _, state := range tt.transitions {
				if err := r.Transition("acme", state); err != nil {
					t.Fatal(err)
				}
			}
			plan, err := r.ThumbnailPlan("acme")
			if tt.wantAllowed {
				if err != nil {
					t.Fatalf("expected plan: %v", err)
				}
				if len(plan) != 3 || plan[0].Width != 320 || plan[2].Width != 1280 {
					t.Fatalf("unexpected plan: %#v", plan)
				}
			} else if err == nil {
				t.Fatalf("expected lifecycle gate, got %#v", plan)
			}
		})
	}
}
