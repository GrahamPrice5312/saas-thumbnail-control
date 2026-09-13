package thumbnail

import (
	"errors"
	"fmt"
	"sync"
)

type AccountState string

const (
	StateOnboarding AccountState = "onboarding"
	StateActive     AccountState = "active"
	StateSuspended  AccountState = "suspended"
	StateClosed     AccountState = "closed"
)

type Variant struct {
	Name   string `json:"name"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

var responsiveVariants = []Variant{
	{Name: "compact", Width: 320, Height: 180},
	{Name: "standard", Width: 640, Height: 360},
	{Name: "wide", Width: 1280, Height: 720},
}

var allowedTransitions = map[AccountState]map[AccountState]bool{
	StateOnboarding: {StateActive: true, StateClosed: true},
	StateActive:     {StateSuspended: true, StateClosed: true},
	StateSuspended:  {StateActive: true, StateClosed: true},
}

type Registry struct {
	mu      sync.RWMutex
	tenants map[string]AccountState
}

func NewRegistry() *Registry {
	return &Registry{tenants: make(map[string]AccountState)}
}

func (r *Registry) Onboard(id string) error {
	if id == "" {
		return errors.New("tenant id is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tenants[id]; exists {
		return fmt.Errorf("tenant %q already exists", id)
	}
	r.tenants[id] = StateOnboarding
	return nil
}

func (r *Registry) Transition(id string, next AccountState) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, exists := r.tenants[id]
	if !exists {
		return fmt.Errorf("tenant %q not found", id)
	}
	if !allowedTransitions[current][next] {
		return fmt.Errorf("cannot move tenant %q from %s to %s", id, current, next)
	}
	r.tenants[id] = next
	return nil
}

func (r *Registry) ThumbnailPlan(id string) ([]Variant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	state, exists := r.tenants[id]
	if !exists {
		return nil, fmt.Errorf("tenant %q not found", id)
	}
	if state != StateActive {
		return nil, fmt.Errorf("tenant %q is %s", id, state)
	}
	return append([]Variant(nil), responsiveVariants...), nil
}
