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

	// 5. Coordinated Advance Step 1: Advance to 1.0
	// Amb1 requests 1.0
	if err := amb1.TimeAdvanceRequest(1.0); err != nil {
		t.Fatalf("amb1 TAR(1.0) failed: %v", err)
	}

	// Evoke callbacks on amb1 to process the grant
	for i := 0; i < 10; i++ {
		_, _ = amb1.EvokeCallback(50 * time.Millisecond)
		if len(cb1.getGrants()) > 0 {
			break
		}
	}
	grants1 := cb1.getGrants()
	if len(grants1) != 1 || grants1[0] != 1.0 {
		t.Fatalf("amb1 expected TAG(1.0), got: %v", grants1)
	}

	// Amb2 requests 1.0
	if err := amb2.TimeAdvanceRequest(1.0); err != nil {
		t.Fatalf("amb2 TAR(1.0) failed: %v", err)
	}

	// Evoke callbacks on amb2 to process the grant
	for i := 0; i < 10; i++ {
		_, _ = amb2.EvokeCallback(50 * time.Millisecond)
		if len(cb2.getGrants()) > 0 {
			break
		}
	}
	grants2 := cb2.getGrants()
	if len(grants2) != 1 || grants2[0] != 1.0 {
		t.Fatalf("amb2 expected TAG(1.0), got: %v", grants2)
	}

	// 6. Resign both federates
	if err := amb1.ResignFederationExecution(); err != nil {
		t.Fatalf("amb1 resign failed: %v", err)
	}
	if err := amb2.ResignFederationExecution(); err != nil {
		t.Fatalf("amb2 resign failed: %v", err)
	}

	// 7. Destroy federation
	if err := amb1.DestroyFederationExecution(fedName); err != nil {
		t.Fatalf("destroy federation failed: %v", err)
	}
}
