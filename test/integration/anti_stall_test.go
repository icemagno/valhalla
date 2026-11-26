package integration_test

import (
	"sync"
	"testing"
	"time"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc"
	"github.com/icemagno/valhalla/pkg/lrc"
)

type antiStallCallbackRecorder struct {
	lrc.NullFederateAmbassador
	mu     sync.Mutex
	grants []float64
}

func (r *antiStallCallbackRecorder) TimeAdvanceGrant(t float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.grants = append(r.grants, t)
}

func (r *antiStallCallbackRecorder) getGrants() []float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	res := make([]float64, len(r.grants))
	copy(res, r.grants)
	return res
}

func TestIntegration_AntiStall_EvictsDeadlockedRegulatingFederate(t *testing.T) {
	// 1. Start CRC server on ephemeral port with fast anti-stall settings (timeout=200ms, interval=40ms)
	server := crc.NewServer("127.0.0.1:0")
	server.SetAntiStall(200*time.Millisecond, 40*time.Millisecond)
	if err := server.Start(); err != nil {
		t.Fatalf("failed to start CRC server: %v", err)
	}
	defer server.Stop()

	crcAddr := server.Addr().String()
	fedName := "AntiStallTestFed"

	// 2. Federate 1 (Healthy)
	cb1 := &antiStallCallbackRecorder{}
	amb1 := lrc.NewRTIambassador()
	if err := amb1.Connect(cb1, crcAddr); err != nil {
		t.Fatalf("amb1 failed to connect: %v", err)
	}
	defer amb1.Disconnect()

	if err := amb1.CreateFederationExecution(fedName); err != nil {
		t.Fatalf("failed to create federation: %v", err)
	}

	h1, err := amb1.JoinFederationExecution("HealthyFed", fedName)
	if err != nil || h1 <= 0 {
		t.Fatalf("amb1 join failed: %v", err)
	}

	// 3. Federate 2 (Stalling)
	cb2 := &antiStallCallbackRecorder{}
	amb2 := lrc.NewRTIambassador()
	if err := amb2.Connect(cb2, crcAddr); err != nil {
		t.Fatalf("amb2 failed to connect: %v", err)
	}
	defer amb2.Disconnect()

	h2, err := amb2.JoinFederationExecution("StallingFed", fedName)
	if err != nil || h2 <= 0 {
		t.Fatalf("amb2 join failed: %v", err)
	}

	// 4. Both enable Time Regulation (lookahead=1.0) and Time Constrained
	if err := amb1.EnableTimeRegulation(1.0); err != nil {
		t.Fatalf("amb1 EnableTimeRegulation failed: %v", err)
	}
	if err := amb1.EnableTimeConstrained(); err != nil {
		t.Fatalf("amb1 EnableTimeConstrained failed: %v", err)
	}

	if err := amb2.EnableTimeRegulation(1.0); err != nil {
		t.Fatalf("amb2 EnableTimeRegulation failed: %v", err)
	}
	if err := amb2.EnableTimeConstrained(); err != nil {
		t.Fatalf("amb2 EnableTimeConstrained failed: %v", err)
	}

	// 5. Initial lockstep advance to 1.0
	if err := amb1.TimeAdvanceRequest(1.0); err != nil {
		t.Fatalf("amb1 TAR(1.0) failed: %v", err)
	}
	if err := amb2.TimeAdvanceRequest(1.0); err != nil {
		t.Fatalf("amb2 TAR(1.0) failed: %v", err)
	}

	for i := 0; i < 10; i++ {
		_, _ = amb1.EvokeCallback(30 * time.Millisecond)
		_, _ = amb2.EvokeCallback(30 * time.Millisecond)
		if len(cb1.getGrants()) > 0 && len(cb2.getGrants()) > 0 {
			break
		}
	}
	if len(cb1.getGrants()) != 1 || cb1.getGrants()[0] != 1.0 {
		t.Fatalf("expected amb1 TAG(1.0), got %v", cb1.getGrants())
	}
	if len(cb2.getGrants()) != 1 || cb2.getGrants()[0] != 1.0 {
		t.Fatalf("expected amb2 TAG(1.0), got %v", cb2.getGrants())
	}

	// 6. Healthy federate requests TAR(5.0)
	if err := amb1.TimeAdvanceRequest(5.0); err != nil {
		t.Fatalf("amb1 TAR(5.0) failed: %v", err)
	}

	// Stalling federate goes completely silent!
	// Initially, amb1 must NOT be granted 5.0 immediately because stalling federate is at 1.0 (lookahead 1.0 -> LBTS=2.0)
	evoked, _ := amb1.EvokeCallback(50 * time.Millisecond)
	if evoked && len(cb1.getGrants()) > 1 {
		t.Fatalf("amb1 should be blocked by regulating peer, but got grant early: %v", cb1.getGrants())
	}

	// 7. Wait for Anti-Stall Engine to detect stalling federate (> 200ms timeout)
	// Background monitor will evict/quarantine StallingFed and dispatch unblocked grant to amb1
	time.Sleep(300 * time.Millisecond)

	// Healthy federate evokes callbacks -> MUST receive TAG(5.0)!
	for i := 0; i < 10; i++ {
		_, _ = amb1.EvokeCallback(50 * time.Millisecond)
		if len(cb1.getGrants()) >= 2 {
			break
		}
	}

	grants := cb1.getGrants()
	if len(grants) < 2 || grants[len(grants)-1] != 5.0 {
		t.Fatalf("Anti-Stall failed to unblock healthy federate! Expected TAG(5.0), got grants: %v", grants)
	}

	// Verify in federation model that HealthyFed is NOT quarantined, while StallingFed IS quarantined
	fedExec, ok := server.FederationManager().GetFederation(fedName)
	if !ok {
		t.Fatalf("federation %q not found", fedName)
	}
	st1, err := fedExec.TimeManager().GetStatus(common.FederateHandle(h1))
	if err != nil {
		t.Fatalf("failed to get status for healthy federate: %v", err)
	}
	if st1.Quarantined {
		t.Fatalf("healthy federate %d should NOT be quarantined, but got Quarantined=true", h1)
	}

	st2, err := fedExec.TimeManager().GetStatus(common.FederateHandle(h2))
	if err != nil {
		t.Fatalf("failed to get status for stalled federate: %v", err)
	}
	if !st2.Quarantined {
		t.Fatalf("expected federate %d to be quarantined, but got Quarantined=false", h2)
	}

	// 8. Stalled federate recovers by sending a Heartbeat
	if err := amb2.Heartbeat(); err != nil {
		t.Fatalf("amb2 Heartbeat failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	stRecovered, err := fedExec.TimeManager().GetStatus(common.FederateHandle(h2))
	if err != nil {
		t.Fatalf("failed to get status for recovered federate: %v", err)
	}
	if stRecovered.Quarantined {
		t.Fatalf("expected federate %d to be unquarantined after Heartbeat, but still Quarantined=true", h2)
	}
}

func TestIntegration_AntiStall_HeartbeatPreventsEviction(t *testing.T) {
	// Start CRC with anti-stall timeout = 250ms
	server := crc.NewServer("127.0.0.1:0")
	server.SetAntiStall(250*time.Millisecond, 40*time.Millisecond)
	if err := server.Start(); err != nil {
		t.Fatalf("failed to start CRC server: %v", err)
	}
	defer server.Stop()

	crcAddr := server.Addr().String()
	fedName := "HeartbeatVitalityFed"

	amb := lrc.NewRTIambassador()
	if err := amb.Connect(&lrc.NullFederateAmbassador{}, crcAddr); err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer amb.Disconnect()

	if err := amb.CreateFederationExecution(fedName); err != nil {
		t.Fatalf("create federation failed: %v", err)
	}

	h, err := amb.JoinFederationExecution("HeartbeatFed", fedName)
	if err != nil || h <= 0 {
		t.Fatalf("join failed: %v", err)
	}

	if err := amb.EnableTimeRegulation(1.0); err != nil {
		t.Fatalf("EnableTimeRegulation failed: %v", err)
	}

	// Send heartbeats every 60ms for a total of 400ms (> 250ms stall timeout)
	start := time.Now()
	for time.Since(start) < 400*time.Millisecond {
		time.Sleep(60 * time.Millisecond)
		if err := amb.Heartbeat(); err != nil {
			t.Fatalf("Heartbeat failed: %v", err)
		}
	}

	// Federate MUST NOT be quarantined
	fedExec, ok := server.FederationManager().GetFederation(fedName)
	if !ok {
		t.Fatalf("federation %q not found", fedName)
	}
	status, err := fedExec.TimeManager().GetStatus(common.FederateHandle(h))
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.Quarantined {
		t.Fatalf("federate was quarantined despite sending regular heartbeats!")
	}
}
