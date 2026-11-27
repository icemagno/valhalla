package integration_test

import (
	"sync"
	"testing"
	"time"

	"github.com/icemagno/valhalla/pkg/crc"
	"github.com/icemagno/valhalla/pkg/lrc"
)

type timeCallbackRecorder struct {
	lrc.NullFederateAmbassador
	mu            sync.Mutex
	grants        []float64
	receivedAttrs int
}

func (r *timeCallbackRecorder) TimeAdvanceGrant(t float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.grants = append(r.grants, t)
}

func (r *timeCallbackRecorder) ReflectAttributeValues(objectHandle int32, attributes map[int32][]byte, tag []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.receivedAttrs++
}

func (r *timeCallbackRecorder) getGrants() []float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	res := make([]float64, len(r.grants))
	copy(res, r.grants)
	return res
}

func (r *timeCallbackRecorder) getReceivedAttrs() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.receivedAttrs
}

func TestIntegration_TimeManagement_LockstepAndTSO(t *testing.T) {
	// 1. Start CRC server on ephemeral port
	server := crc.NewServer("127.0.0.1:0")
	if err := server.Start(); err != nil {
		t.Fatalf("failed to start CRC server: %v", err)
	}
	defer server.Stop()

	crcAddr := server.Addr().String()

	// 2. Federate 1 (Publisher & Regulator)
	cb1 := &timeCallbackRecorder{}
	amb1 := lrc.NewRTIambassador()
	if err := amb1.Connect(cb1, crcAddr); err != nil {
		t.Fatalf("amb1 failed to connect: %v", err)
	}
	defer amb1.Disconnect()

	fedName := "TimeTestFederation"
	if err := amb1.CreateFederationExecution(fedName); err != nil {
		t.Fatalf("amb1 failed to create federation: %v", err)
	}

	h1, err := amb1.JoinFederationExecution("AircraftSim", fedName)
	if err != nil {
		t.Fatalf("amb1 failed to join: %v", err)
	}
	if h1 <= 0 {
		t.Fatalf("amb1 invalid handle: %d", h1)
	}

	// 3. Federate 2 (Subscriber & Constrained)
	cb2 := &timeCallbackRecorder{}
	amb2 := lrc.NewRTIambassador()
	if err := amb2.Connect(cb2, crcAddr); err != nil {
		t.Fatalf("amb2 failed to connect: %v", err)
	}
	defer amb2.Disconnect()

	h2, err := amb2.JoinFederationExecution("ControlTowerSim", fedName)
	if err != nil {
		t.Fatalf("amb2 failed to join: %v", err)
	}
	if h2 <= 0 {
		t.Fatalf("amb2 invalid handle: %d", h2)
	}

	// 4. Configure Time Regulation and Constraint
	// Amb1: Regulating (lookahead 1.0) and Constrained
	if err := amb1.EnableTimeRegulation(1.0); err != nil {
		t.Fatalf("amb1 EnableTimeRegulation failed: %v", err)
	}
	if err := amb1.EnableTimeConstrained(); err != nil {
		t.Fatalf("amb1 EnableTimeConstrained failed: %v", err)
	}

	// Amb2: Regulating (lookahead 1.0) and Constrained
	if err := amb2.EnableTimeRegulation(1.0); err != nil {
		t.Fatalf("amb2 EnableTimeRegulation failed: %v", err)
	}
	if err := amb2.EnableTimeConstrained(); err != nil {
		t.Fatalf("amb2 EnableTimeConstrained failed: %v", err)
	}

	// 5. Real LBTS Lockstep Blocking:
	// Both federates are at t=0.0 with lookahead=1.0 -> LBTS for Amb1 is 1.0.
	// Requesting TAR(5.0) on Amb1 MUST block until Amb2 advances!
	if err := amb1.TimeAdvanceRequest(5.0); err != nil {
		t.Fatalf("amb1 TAR(5.0) failed: %v", err)
	}
	evoked, err := amb1.EvokeCallback(100 * time.Millisecond)
	if err != nil {
		t.Fatalf("amb1 EvokeCallback failed: %v", err)
	}
	if evoked || len(cb1.getGrants()) != 0 {
		t.Fatalf("LBTS LOCKSTEP VIOLATION: amb1 received TAG(5.0) while amb2 was still at t=0.0! grants=%v", cb1.getGrants())
	}

	// 6. Real object publish/subscribe + attribute update while Amb1 is advancing
	clsHandle, err := amb1.GetObjectClassHandle("HLAobjectRoot")
	if err != nil {
		t.Fatalf("GetObjectClassHandle failed: %v", err)
	}
	privHandle, err := amb1.GetAttributeHandle(clsHandle, "HLAprivilegeToDeleteObject")
	if err != nil {
		t.Fatalf("GetAttributeHandle failed: %v", err)
	}
	if err := amb2.PublishObjectClassAttributes(clsHandle, []int32{privHandle}); err != nil {
		t.Fatalf("amb2 publish failed: %v", err)
	}
	if err := amb1.SubscribeObjectClassAttributes(clsHandle, []int32{privHandle}); err != nil {
		t.Fatalf("amb1 subscribe failed: %v", err)
	}
	objHandle, err := amb2.RegisterObjectInstance(clsHandle, "LockstepEntity")
	if err != nil {
		t.Fatalf("amb2 register object failed: %v", err)
	}
	// Send a real Timestamp-Ordered (TSO) attribute update at logical time t=3.0 (>= lookahead 1.0, <= TAR 5.0).
	if err := amb2.UpdateAttributeValuesWithTime(objHandle, map[int32][]byte{privHandle: []byte("tso-payload")}, 3.0, []byte("tag")); err != nil {
		t.Fatalf("amb2 TSO UpdateAttributeValuesWithTime(3.0) failed: %v", err)
	}
	// Amb1 is still blocked below LBTS 3.0, so TSO attribute must NOT be delivered yet.
	_, _ = amb1.EvokeCallback(50 * time.Millisecond)
	if cb1.getReceivedAttrs() != 0 {
		t.Fatalf("TSO ORDERING VIOLATION: amb1 received TSO reflection at t=3.0 before LBTS reached 3.0")
	}

	// 7. Now Amb2 advances to 5.0 -> unlocks LBTS for BOTH Amb1 and Amb2
	if err := amb2.TimeAdvanceRequest(5.0); err != nil {
		t.Fatalf("amb2 TAR(5.0) failed: %v", err)
	}

	// Drain callbacks on Amb1 until TAG(5.0) and reflected attribute arrive
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, _ = amb1.EvokeCallback(20 * time.Millisecond)
		if len(cb1.getGrants()) > 0 && cb1.getReceivedAttrs() > 0 {
			break
		}
	}
	grants1 := cb1.getGrants()
	if len(grants1) != 1 || grants1[0] != 5.0 {
		t.Fatalf("amb1 expected TAG(5.0) after Amb2 advanced, got: %v", grants1)
	}
	if cb1.getReceivedAttrs() == 0 {
		t.Fatalf("amb1 expected reflected attributes from Amb2, got 0")
	}

	// Drain callbacks on Amb2 until TAG(5.0) arrives
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, _ = amb2.EvokeCallback(20 * time.Millisecond)
		if len(cb2.getGrants()) > 0 {
			break
		}
	}
	grants2 := cb2.getGrants()
	if len(grants2) != 1 || grants2[0] != 5.0 {
		t.Fatalf("amb2 expected TAG(5.0), got: %v", grants2)
	}

	// 8. Resign both federates
	if err := amb1.ResignFederationExecution(); err != nil {
		t.Fatalf("amb1 resign failed: %v", err)
	}
	if err := amb2.ResignFederationExecution(); err != nil {
		t.Fatalf("amb2 resign failed: %v", err)
	}

	// 9. Destroy federation
	if err := amb1.DestroyFederationExecution(fedName); err != nil {
		t.Fatalf("destroy federation failed: %v", err)
	}
}
