package integration_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc"
	"github.com/icemagno/valhalla/pkg/lrc"
)

type burstCallbackRecorder struct {
	lrc.NullFederateAmbassador
	mu           sync.Mutex
	interactions []string
}

func (r *burstCallbackRecorder) ReceiveInteraction(interactionClass int32, parameters map[int32][]byte, tag []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.interactions = append(r.interactions, string(tag))
}

func (r *burstCallbackRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.interactions)
}

func (r *burstCallbackRecorder) getTags() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	res := make([]string, len(r.interactions))
	copy(res, r.interactions)
	return res
}

// TestIntegration_CallbackQueueBurstEndToEnd verifies with a real CRC server
// that an unconsumed burst of 2,500 interactions is delivered without a single drop,
// proving the fix for Item 3.3 in an end-to-end distributed topology.
func TestIntegration_CallbackQueueBurstEndToEnd(t *testing.T) {
	server := crc.NewServer("127.0.0.1:0")
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start CRC: %v", err)
	}
	defer server.Stop()

	crcAddr := server.Addr().String()
	fedName := "BurstQueueFed"

	// 1. Publisher Federate
	ambPub := lrc.NewRTIambassador()
	cbPub := &burstCallbackRecorder{}
	if err := ambPub.Connect(cbPub, crcAddr); err != nil {
		t.Fatalf("Publisher connect failed: %v", err)
	}
	defer ambPub.Disconnect()

	if err := ambPub.CreateFederationExecution(fedName); err != nil {
		t.Fatalf("Create federation failed: %v", err)
	}

	pubH, err := ambPub.JoinFederationExecution("Publisher", fedName)
	if err != nil || pubH <= 0 {
		t.Fatalf("Publisher join failed: %v", err)
	}

	// 2. Subscriber Federate
	ambSub := lrc.NewRTIambassador()
	cbSub := &burstCallbackRecorder{}
	if err := ambSub.Connect(cbSub, crcAddr); err != nil {
		t.Fatalf("Subscriber connect failed: %v", err)
	}
	defer ambSub.Disconnect()

	subH, err := ambSub.JoinFederationExecution("Subscriber", fedName)
	if err != nil || subH <= 0 {
		t.Fatalf("Subscriber join failed: %v", err)
	}

	// 3. Setup Pub/Sub on Interaction Class (HLAinteractionRootHandle)
	interactionClass := int32(common.HLAinteractionRootHandle)
	if err := ambPub.PublishInteractionClass(interactionClass); err != nil {
		t.Fatalf("PublishInteractionClass failed: %v", err)
	}
	if err := ambSub.SubscribeInteractionClass(interactionClass); err != nil {
		t.Fatalf("SubscribeInteractionClass failed: %v", err)
	}

	// Give CRC a moment to register pub/sub tables
	time.Sleep(50 * time.Millisecond)

	// 4. Publisher fires a burst of 2,500 interactions in a tight loop.
	// Subscriber DOES NOT call EvokeCallback during the entire burst!
	const burstSize = 2500
	for i := 0; i < burstSize; i++ {
		tag := []byte(fmt.Sprintf("burst-msg-%05d", i))
		if err := ambPub.SendInteraction(interactionClass, nil, tag); err != nil {
			t.Fatalf("SendInteraction failed at index %d: %v", i, err)
		}
	}

	// Wait for CRC and OS TCP buffers to transfer packets
	time.Sleep(300 * time.Millisecond)

	// Check subscriber queue status before evoking
	qLen, saturated, dropped := ambSub.GetCallbackQueueStatus()
	t.Logf("Subscriber queue before evocation: len=%d, saturated=%v, dropped=%d", qLen, saturated, dropped)
	if saturated {
		t.Fatalf("Unexpected saturation before evocation")
	}
	if dropped != 0 {
		t.Fatalf("Unexpected drops before evocation: %d", dropped)
	}

	// 5. Subscriber evokes all pending callbacks
	deadline := time.Now().Add(5 * time.Second)
	for cbSub.count() < burstSize && time.Now().Before(deadline) {
		evoked, err := ambSub.EvokeCallback(50 * time.Millisecond)
		if err != nil {
			t.Fatalf("EvokeCallback failed: %v", err)
		}
		if !evoked {
			time.Sleep(10 * time.Millisecond)
		}
	}

	receivedCount := cbSub.count()
	if receivedCount != burstSize {
		t.Fatalf("SILENT DROP DETECTED: expected %d interactions, but subscriber received %d (lost %d)",
			burstSize, receivedCount, burstSize-receivedCount)
	}

	// Verify exact FIFO ordering
	tags := cbSub.getTags()
	for i := 0; i < burstSize; i++ {
		expectedTag := fmt.Sprintf("burst-msg-%05d", i)
		if tags[i] != expectedTag {
			t.Fatalf("FIFO ordering violation at index %d: expected %s, got %s", i, expectedTag, tags[i])
		}
	}

	t.Logf("Successfully received all %d interactions in exact FIFO order with 0 drops", receivedCount)
}

// TestIntegration_CallbackQueueSaturationAlerting verifies that when subscriber queue capacity
// is reached, saturation is flagged and EvokeCallback returns ErrCallbackQueueSaturated.
func TestIntegration_CallbackQueueSaturationAlerting(t *testing.T) {
	server := crc.NewServer("127.0.0.1:0")
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start CRC: %v", err)
	}
	defer server.Stop()

	crcAddr := server.Addr().String()
	fedName := "SaturationQueueFed"

	ambPub := lrc.NewRTIambassador()
	cbPub := &burstCallbackRecorder{}
	if err := ambPub.Connect(cbPub, crcAddr); err != nil {
		t.Fatalf("Publisher connect failed: %v", err)
	}
	defer ambPub.Disconnect()

	_ = ambPub.CreateFederationExecution(fedName)
	_, _ = ambPub.JoinFederationExecution("Publisher", fedName)

	ambSub := lrc.NewRTIambassador()
	cbSub := &burstCallbackRecorder{}
	if err := ambSub.Connect(cbSub, crcAddr); err != nil {
		t.Fatalf("Subscriber connect failed: %v", err)
	}
	defer ambSub.Disconnect()

	_, _ = ambSub.JoinFederationExecution("Subscriber", fedName)

	interactionClass := int32(common.HLAinteractionRootHandle)
	if err := ambPub.PublishInteractionClass(interactionClass); err != nil {
		t.Fatalf("PublishInteractionClass failed: %v", err)
	}
	if err := ambSub.SubscribeInteractionClass(interactionClass); err != nil {
		t.Fatalf("SubscribeInteractionClass failed: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	// Set small queue ceiling on subscriber
	const smallCapacity = 50
	ambSub.SetCallbackQueueCapacity(smallCapacity)

	// Publisher sends 120 interactions
	const totalSent = 120
	for i := 0; i < totalSent; i++ {
		_ = ambPub.SendInteraction(interactionClass, nil, []byte(fmt.Sprintf("tag-%d", i)))
	}

	time.Sleep(200 * time.Millisecond)

	// Verify saturation detected
	qLen, saturated, dropped := ambSub.GetCallbackQueueStatus()
	t.Logf("Subscriber queue status: len=%d, saturated=%v, dropped=%d", qLen, saturated, dropped)

	if !saturated {
		t.Fatalf("Expected saturated=true, got false")
	}
	if dropped != (totalSent - smallCapacity) {
		t.Fatalf("Expected %d drops, got %d", totalSent-smallCapacity, dropped)
	}

	// Evoking must return ErrCallbackQueueSaturated
	evoked, err := ambSub.EvokeCallback(100 * time.Millisecond)
	if !evoked {
		t.Fatalf("Expected callback to be evoked")
	}
	if err != lrc.ErrCallbackQueueSaturated {
		t.Fatalf("Expected ErrCallbackQueueSaturated, got %v", err)
	}
}
