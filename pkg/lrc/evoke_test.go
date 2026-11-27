package lrc

import (
	"testing"
	"time"

	"github.com/icemagno/valhalla/pkg/common"
)

type countingTimeAmbassador struct {
	NullFederateAmbassador
	grants int
	lastT  float64
}

func (c *countingTimeAmbassador) TimeAdvanceGrant(t float64) {
	c.grants++
	c.lastT = t
}

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
// with a real FederateAmbassador and real SSZ TimeAdvanceGrant payload allocates 0 heap objects.
func TestEvokeCallback_ZeroHeapAllocsWhenCallbackReady(t *testing.T) {
	fedAmb := &countingTimeAmbassador{}
	amb := &RTIambassadorImpl{
		fedAmb: fedAmb,
		client: &Client{
			callbacks:  make(chan callbackItem, 2000),
			closedChan: make(chan struct{}),
		},
	}

	validPayload, err := common.TimeFloat64Payload{Time: 42.5}.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary failed: %v", err)
	}

	for i := 0; i < 2000; i++ {
		amb.client.callbacks <- callbackItem{
			MsgType: common.NativeMsgTimeAdvanceGrant,
			Payload: validPayload,
		}
	}

	allocs := testing.AllocsPerRun(500, func() {
		evoked, err := amb.EvokeCallback(50 * time.Millisecond)
		if !evoked || err != nil {
			t.Fatalf("expected (true, nil), got (%v, %v)", evoked, err)
		}
	})

	if fedAmb.grants == 0 || fedAmb.lastT != 42.5 {
		t.Fatalf("expected real TimeAdvanceGrant(42.5) dispatch, got grants=%d lastT=%v", fedAmb.grants, fedAmb.lastT)
	}
	if allocs > 0 {
		t.Fatalf("HEAP ALLOCATION DETECTED: EvokeCallback allocated %v allocs/op on heap (expected 0)", allocs)
	}
}
