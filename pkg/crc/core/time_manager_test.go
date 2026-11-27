package core

import (
	"sync"
	"testing"
	"time"

	"github.com/icemagno/valhalla/pkg/common"
)

func TestTimeManager_LifecycleAndFlags(t *testing.T) {
	tm := NewTimeManager()
	fed1 := common.FederateHandle(101)

	tm.RegisterFederate(fed1)
	status, err := tm.GetStatus(fed1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.CurrentTime != 0.0 || status.IsRegulating || status.IsConstrained {
		t.Fatalf("unexpected initial status: %+v", status)
	}

	// Negative lookahead must be rejected
	if err := tm.EnableTimeRegulation(fed1, -1.0); err != common.ErrInvalidLookahead {
		t.Fatalf("expected ErrInvalidLookahead, got: %v", err)
	}

	// Valid enable regulation
	if err := tm.EnableTimeRegulation(fed1, 1.0); err != nil {
		t.Fatalf("failed to enable regulation: %v", err)
	}

	// Duplicate enable regulation must fail
	if err := tm.EnableTimeRegulation(fed1, 1.0); err != common.ErrTimeRegulationAlreadyEnabled {
		t.Fatalf("expected ErrTimeRegulationAlreadyEnabled, got: %v", err)
	}

	// Enable constrained
	if err := tm.EnableTimeConstrained(fed1); err != nil {
		t.Fatalf("failed to enable constrained: %v", err)
	}

	// Duplicate enable constrained must fail
	if err := tm.EnableTimeConstrained(fed1); err != common.ErrTimeConstrainedAlreadyEnabled {
		t.Fatalf("expected ErrTimeConstrainedAlreadyEnabled, got: %v", err)
	}

	// Modify lookahead
	if err := tm.ModifyLookahead(fed1, 2.5); err != nil {
		t.Fatalf("failed to modify lookahead: %v", err)
	}
	status, _ = tm.GetStatus(fed1)
	if status.Lookahead != 2.5 {
		t.Fatalf("expected lookahead 2.5, got %v", status.Lookahead)
	}
}

func TestTimeManager_UnconstrainedImmediateGrant(t *testing.T) {
	tm := NewTimeManager()
	fed1 := common.FederateHandle(1)
	tm.RegisterFederate(fed1)

	// An unconstrained federate should be granted target time immediately
	grants, err := tm.TimeAdvanceRequest(fed1, 10.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(grants) != 1 {
		t.Fatalf("expected 1 grant, got %d", len(grants))
	}
	if grants[0].Federate != fed1 || grants[0].Time != 10.0 {
		t.Fatalf("unexpected grant: %+v", grants[0])
	}

	status, _ := tm.GetStatus(fed1)
	if status.CurrentTime != 10.0 || status.State != TimeStateGranted {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestTimeManager_CoordinatedAdvanceLockstep(t *testing.T) {
	tm := NewTimeManager()
	fed1 := common.FederateHandle(1)
	fed2 := common.FederateHandle(2)

	tm.RegisterFederate(fed1)
	tm.RegisterFederate(fed2)

	// Both regulating and constrained with lookahead = 1.0
	_ = tm.EnableTimeRegulation(fed1, 1.0)
	_ = tm.EnableTimeConstrained(fed1)

	_ = tm.EnableTimeRegulation(fed2, 1.0)
	_ = tm.EnableTimeConstrained(fed2)

	// Current times: fed1=0.0 (impending=1.0), fed2=0.0 (impending=1.0)
	// Both request advance to 1.0
	grants1, err := tm.TimeAdvanceRequest(fed1, 1.0)
	if err != nil {
		t.Fatalf("fed1 request failed: %v", err)
	}
	// Fed1 cannot advance yet because Fed2's impending time is 0 + 1.0 = 1.0.
	// Target is 1.0 <= LBTS (1.0), so fed1 CAN be granted!
	if len(grants1) != 1 || grants1[0].Time != 1.0 {
		t.Fatalf("fed1 should be granted 1.0 since fed2 impending is 1.0, got: %+v", grants1)
	}

	// Now Fed1 is at 1.0 with lookahead 1.0 -> impending is 2.0
	// Fed2 requests 1.0 -> LBTS is 2.0 -> Fed2 is granted 1.0
	grants2, err := tm.TimeAdvanceRequest(fed2, 1.0)
	if err != nil {
		t.Fatalf("fed2 request failed: %v", err)
	}
	if len(grants2) != 1 || grants2[0].Time != 1.0 {
		t.Fatalf("expected fed2 grant 1.0, got: %+v", grants2)
	}

	// Now both are at 1.0.
	// Fed1 requests 3.0 -> impending of Fed2 is 1.0 + 1.0 = 2.0.
	// 3.0 > 2.0, so Fed1 must WAIT!
	grants3, err := tm.TimeAdvanceRequest(fed1, 3.0)
	if err != nil {
		t.Fatalf("fed1 request failed: %v", err)
	}
	if len(grants3) != 0 {
		t.Fatalf("fed1 should not be granted 3.0 yet, got: %+v", grants3)
	}

	// Now Fed2 requests 2.0.
	// Fed1's target is 3.0 + lookahead 1.0 = 4.0 impending.
	// Fed2 target is 2.0 <= 4.0 -> Fed2 is granted 2.0!
	// With Fed2 at 2.0 (impending 3.0), Fed1's target 3.0 <= 3.0 -> Fed1 is granted 3.0!
	grants4, err := tm.TimeAdvanceRequest(fed2, 2.0)
	if err != nil {
		t.Fatalf("fed2 request failed: %v", err)
	}

	// Both Fed2 (grant 2.0) and Fed1 (grant 3.0) should be granted!
	if len(grants4) != 2 {
		t.Fatalf("expected 2 cascade grants, got %d: %+v", len(grants4), grants4)
	}

	s1, _ := tm.GetStatus(fed1)
	s2, _ := tm.GetStatus(fed2)
	if s1.CurrentTime != 3.0 || s2.CurrentTime != 2.0 {
		t.Fatalf("unexpected times: fed1=%v, fed2=%v", s1.CurrentTime, s2.CurrentTime)
	}
}

func TestTimeManager_TSOQueueAndOrdering(t *testing.T) {
	tm := NewTimeManager()
	sender := common.FederateHandle(1)
	receiver := common.FederateHandle(2)

	tm.RegisterFederate(sender)
	tm.RegisterFederate(receiver)

	_ = tm.EnableTimeRegulation(sender, 2.0)
	_ = tm.EnableTimeConstrained(receiver)

	// Sender is at 0.0 with lookahead 2.0 -> min valid timestamp is 2.0
	// Attempt to enqueue a message with timestamp 1.5 must fail
	invalidMsg := TSOMessage{
		Sender:    sender,
		Receiver:  receiver,
		Timestamp: 1.5,
		Payload:   []byte("too early"),
	}
	if _, err := tm.EnqueueTSOMessage(invalidMsg); err != common.ErrInvalidLogicalTime {
		t.Fatalf("expected ErrInvalidLogicalTime, got: %v", err)
	}

	// Enqueue messages out of order: 5.0, 3.0, 4.0, 3.0 (tie-break test)
	msg1 := TSOMessage{Sender: sender, Receiver: receiver, Timestamp: 5.0, Payload: []byte("msg-5.0")}
	msg2 := TSOMessage{Sender: sender, Receiver: receiver, Timestamp: 3.0, Payload: []byte("msg-3.0-first")}
	msg3 := TSOMessage{Sender: sender, Receiver: receiver, Timestamp: 4.0, Payload: []byte("msg-4.0")}
	msg4 := TSOMessage{Sender: sender, Receiver: receiver, Timestamp: 3.0, Payload: []byte("msg-3.0-second")}

	_, _ = tm.EnqueueTSOMessage(msg1)
	_, _ = tm.EnqueueTSOMessage(msg2)
	_, _ = tm.EnqueueTSOMessage(msg3)
	_, _ = tm.EnqueueTSOMessage(msg4)

	// Sender advances time to 10.0 (impending 12.0)
	_, _ = tm.TimeAdvanceRequest(sender, 10.0)

	// Receiver requests advance to 4.0
	grants, err := tm.TimeAdvanceRequest(receiver, 4.0)
	if err != nil {
		t.Fatalf("advance request failed: %v", err)
	}
	if len(grants) != 1 {
		t.Fatalf("expected 1 grant, got %d", len(grants))
	}

	delivered := grants[0].Messages
	if len(delivered) != 3 {
		t.Fatalf("expected 3 delivered messages (3.0, 3.0, 4.0), got %d", len(delivered))
	}

	// Verify strict order: 3.0 first, 3.0 second, then 4.0
	if string(delivered[0].Payload) != "msg-3.0-first" ||
		string(delivered[1].Payload) != "msg-3.0-second" ||
		string(delivered[2].Payload) != "msg-4.0" {
		t.Fatalf("incorrect message ordering: %+v", delivered)
	}

	// Next advance to 6.0 delivers the remaining message (5.0)
	grantsNext, _ := tm.TimeAdvanceRequest(receiver, 6.0)
	if len(grantsNext) != 1 || len(grantsNext[0].Messages) != 1 || string(grantsNext[0].Messages[0].Payload) != "msg-5.0" {
		t.Fatalf("expected msg-5.0 on next advance, got: %+v", grantsNext)
	}
}

func TestTimeManager_NextMessageRequest(t *testing.T) {
	tm := NewTimeManager()
	sender := common.FederateHandle(1)
	receiver := common.FederateHandle(2)

	tm.RegisterFederate(sender)
	tm.RegisterFederate(receiver)

	_ = tm.EnableTimeRegulation(sender, 1.0)
	_ = tm.EnableTimeConstrained(receiver)

	// Sender at 0.0 with lookahead 1.0 sends a message at 2.5
	msg := TSOMessage{
		Sender:    sender,
		Receiver:  receiver,
		Timestamp: 2.5,
		Payload:   []byte("sample"),
	}
	_, _ = tm.EnqueueTSOMessage(msg)

	// Sender advances to 10.0 -> impending is 11.0
	_, _ = tm.TimeAdvanceRequest(sender, 10.0)

	// Receiver calls NextMessageRequest(5.0)
	// Even though target is 5.0, NMR should grant exactly 2.5 (the message timestamp)
	grants, err := tm.NextMessageRequest(receiver, 5.0)
	if err != nil {
		t.Fatalf("NMR failed: %v", err)
	}
	if len(grants) != 1 || grants[0].Time != 2.5 {
		t.Fatalf("NMR expected grant 2.5, got: %+v", grants)
	}
	if len(grants[0].Messages) != 1 {
		t.Fatalf("expected 1 delivered message, got %d", len(grants[0].Messages))
	}
}

func TestTimeManager_EvictStalledFederates(t *testing.T) {
	tm := NewTimeManager()
	healthy := common.FederateHandle(1)
	stalled := common.FederateHandle(2)

	tm.RegisterFederate(healthy)
	tm.RegisterFederate(stalled)

	_ = tm.EnableTimeRegulation(healthy, 1.0)
	_ = tm.EnableTimeConstrained(healthy)

	_ = tm.EnableTimeRegulation(stalled, 1.0)
	_ = tm.EnableTimeConstrained(stalled)

	// Healthy requests advance to 5.0
	// It is blocked because Stalled is at 0.0 + lookahead 1.0 = 1.0 LBTS
	grants, _ := tm.TimeAdvanceRequest(healthy, 5.0)
	if len(grants) != 0 {
		t.Fatalf("healthy should be blocked, got: %+v", grants)
	}

	// Sleep 60ms, keep healthy alive with a heartbeat, allow stalled to timeout (50ms)
	time.Sleep(60 * time.Millisecond)
	tm.Heartbeat(healthy)
	evicted, unblockGrants := tm.EvictStalledFederates(50 * time.Millisecond)

	if len(evicted) != 1 || evicted[0] != stalled {
		t.Fatalf("expected stalled federate to be evicted, got: %v", evicted)
	}

	// Healthy federate should now be unblocked and granted 5.0
	if len(unblockGrants) != 1 || unblockGrants[0].Federate != healthy || unblockGrants[0].Time != 5.0 {
		t.Fatalf("healthy federate should be granted 5.0 after eviction, got: %+v", unblockGrants)
	}
}

func TestTimeManager_ConcurrencyRaceFree(t *testing.T) {
	tm := NewTimeManager()
	const numFederates = 16
	var wg sync.WaitGroup

	for i := 1; i <= numFederates; i++ {
		h := common.FederateHandle(i)
		tm.RegisterFederate(h)
		if err := tm.EnableTimeRegulation(h, 0.5); err != nil {
			t.Fatalf("EnableTimeRegulation(%d) failed: %v", i, err)
		}
		if err := tm.EnableTimeConstrained(h); err != nil {
			t.Fatalf("EnableTimeConstrained(%d) failed: %v", i, err)
		}
	}

	wg.Add(numFederates)
	for i := 1; i <= numFederates; i++ {
		h := common.FederateHandle(i)
		go func(handle common.FederateHandle) {
			defer wg.Done()
			for step := 1; step <= 20; step++ {
				target := float64(step) * 0.5
				_, err := tm.TimeAdvanceRequest(handle, target)
				if err != nil && err != common.ErrTimeAdvanceAlreadyInProgress {
					t.Errorf("unexpected TimeAdvanceRequest error for federate %d at %f: %v", handle, target, err)
				}
				tm.Heartbeat(handle)
				lbts := tm.CalculateLBTS(handle)
				if lbts < 0 {
					t.Errorf("negative LBTS %f for federate %d", lbts, handle)
				}
			}
		}(h)
	}

	wg.Wait()

	for i := 1; i <= numFederates; i++ {
		h := common.FederateHandle(i)
		st, err := tm.GetStatus(h)
		if err != nil {
			t.Fatalf("federate %d missing after concurrent run: %v", i, err)
		}
		if !st.IsRegulating || !st.IsConstrained || st.CurrentTime < 0 {
			t.Fatalf("federate %d corrupted state: %+v", i, st)
		}
	}
}

func BenchmarkTimeManager_CalculateLBTS(b *testing.B) {
	tm := NewTimeManager()
	for i := 1; i <= 64; i++ {
		h := common.FederateHandle(i)
		tm.RegisterFederate(h)
		_ = tm.EnableTimeRegulation(h, 1.0)
		_ = tm.EnableTimeConstrained(h)
	}

	target := common.FederateHandle(1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tm.CalculateLBTS(target)
	}
}
