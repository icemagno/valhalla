package main

import (
	"testing"
	"time"
)

func TestFederateCallbackHandler_WaitForGrant_SuccessAndTimeout(t *testing.T) {
	h := NewCallbackHandler()

	// 1. Timeout when no grant arrives
	start := time.Now()
	if h.WaitForGrant(10.0, 50*time.Millisecond) {
		t.Fatalf("expected WaitForGrant to return false on timeout")
	}
	if time.Since(start) < 40*time.Millisecond {
		t.Fatalf("WaitForGrant returned too early before timeout")
	}

	// 2. Immediate wakeup via sync.Cond when TimeAdvanceGrant arrives
	go func() {
		time.Sleep(25 * time.Millisecond)
		h.TimeAdvanceGrant(10.0)
	}()

	start = time.Now()
	if !h.WaitForGrant(10.0, 2*time.Second) {
		t.Fatalf("expected WaitForGrant to return true after TimeAdvanceGrant(10.0)")
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Fatalf("WaitForGrant took too long after grant wakeup: %v", time.Since(start))
	}

	// 3. FederationSynchronized updates state
	h.FederationSynchronized("Ready")
	h.mu.Lock()
	synced := h.isSynchronized
	h.mu.Unlock()
	if !synced {
		t.Fatalf("expected isSynchronized=true after FederationSynchronized")
	}
}
