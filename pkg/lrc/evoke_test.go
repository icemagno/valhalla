package lrc

import (
	"testing"
	"time"
)

// TestEvokeCallback_ZeroTimeoutNonBlocking proves that EvokeCallback(0) must return
// immediately with (false, nil) when no callbacks are pending, instead of hanging forever.
func TestEvokeCallback_ZeroTimeoutNonBlocking(t *testing.T) {
	amb := &RTIambassadorImpl{
		client: &Client{
			callbacks:  make(chan callbackItem, 10),
			closedChan: make(chan struct{}),
		},
	}

	done := make(chan struct{})
	go func() {
		evoked, err := amb.EvokeCallback(0)
		if evoked || err != nil {
			t.Errorf("expected (false, nil), got (%v, %v)", evoked, err)
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("DEADLOCK DETECTED: EvokeCallback(0) blocked indefinitely when callback queue was empty!")
	}
}

// TestEvokeCallback_ZeroHeapAllocsWhenCallbackReady verifies that invoking EvokeCallback
// with a timeout does not allocate timers on the heap when callbacks are readily available.
func TestEvokeCallback_ZeroHeapAllocsWhenCallbackReady(t *testing.T) {
	amb := &RTIambassadorImpl{
		client: &Client{
			callbacks:  make(chan callbackItem, 2000),
			closedChan: make(chan struct{}),
		},
	}

	for i := 0; i < 2000; i++ {
		amb.client.callbacks <- callbackItem{
			MsgType: 43,
			Payload: []byte("test"),
		}
	}

	allocs := testing.AllocsPerRun(500, func() {
		_, _ = amb.EvokeCallback(50 * time.Millisecond)
	})

	if allocs > 0 {
		t.Fatalf("HEAP ALLOCATION DETECTED: EvokeCallback allocated %v allocs/op on heap (expected 0)", allocs)
	}
}
