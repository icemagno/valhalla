// Project Valhalla - Sovereign HLA-4 Run-Time Infrastructure
// Lead Developer: Carlos Magno Oliveira de Abreu (@icemagno / magno.mabreu@gmail.com)

package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/icemagno/valhalla/pkg/crc"
)

const (
	ValhallaOK             = 0
	ValhallaErrInvalidArg  = 2
	ValhallaErrNotMember   = 11
)

func TestCABI_NullPointerSafety(t *testing.T) {
	amb := CABI_CreateAmbassador()
	if amb == 0 {
		t.Fatalf("failed to create ambassador")
	}
	defer CABI_DestroyAmbassador(amb)

	results := CABI_NullPointerChecks(amb)
	for i, code := range results {
		if code != ValhallaErrInvalidArg {
			t.Fatalf("null pointer check %d returned code %d, expected %d (VALHALLA_ERR_INVALID_ARG)", i, code, ValhallaErrInvalidArg)
		}
	}
}

func TestCABI_InteractionLifecycleAndCallbacks(t *testing.T) {
	Bridge_ResetState()

	// 1. Start ephemeral CRC server
	server := crc.NewServer("127.0.0.1:0")
	if err := server.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer server.Stop()

	crcAddr := server.Addr().String()
	fedName := "CABI_Interaction_Test_Federation"

	// 2. Setup Sender & Receiver
	sender := CABI_CreateAmbassador()
	defer CABI_DestroyAmbassador(sender)

	receiver := CABI_CreateAmbassador()
	defer CABI_DestroyAmbassador(receiver)

	if code := CABI_SetTestCallbacks(receiver); code != ValhallaOK {
		t.Fatalf("set test callbacks failed on receiver: %d", code)
	}

	if code := CABI_Connect(sender, crcAddr); code != ValhallaOK {
		t.Fatalf("sender connect failed: %d", code)
	}
	defer CABI_Disconnect(sender)

	if code := CABI_Connect(receiver, crcAddr); code != ValhallaOK {
		t.Fatalf("receiver connect failed: %d", code)
	}
	defer CABI_Disconnect(receiver)

	if code := CABI_CreateFederation(sender, fedName); code != ValhallaOK {
		t.Fatalf("create federation failed: %d", code)
	}
	defer CABI_DestroyFederation(sender, fedName)

	if _, code := CABI_JoinFederation(sender, "CABISender", fedName); code != ValhallaOK {
		t.Fatalf("sender join failed: %d", code)
	}

	if _, code := CABI_JoinFederation(receiver, "CABIReceiver", fedName); code != ValhallaOK {
		t.Fatalf("receiver join failed: %d", code)
	}

	// 3. Publish & Subscribe Interaction
	interactionClass := int32(1) // HLAinteractionRoot
	if code := CABI_PublishInteractionClass(sender, interactionClass); code != ValhallaOK {
		t.Fatalf("publish interaction failed: %d", code)
	}

	if code := CABI_SubscribeInteractionClass(receiver, interactionClass); code != ValhallaOK {
		t.Fatalf("subscribe interaction failed: %d", code)
	}

	time.Sleep(50 * time.Millisecond)

	// 4. Send Interaction with Parameters and Tag
	params := map[int32][]byte{
		101: []byte("VelocityVectorX"),
		102: []byte("SensorStatusBitmask"),
	}
	tag := []byte("CombatNetMessageTag")

	if code := CABI_SendInteraction(sender, interactionClass, params, tag); code != ValhallaOK {
		t.Fatalf("send interaction failed: %d", code)
	}

	time.Sleep(50 * time.Millisecond)

	// 5. Evoke Callback on Receiver
	evoked, code := CABI_EvokeCallback(receiver, 0.2)
	if code != ValhallaOK {
		t.Fatalf("evoke callback failed: %d", code)
	}
	if !evoked {
		t.Fatalf("expected callback to be evoked on receiver")
	}

	// 6. Assert Callback Payload Integrity
	recvClass, recvParams, recvTag := Bridge_GetReceivedInteraction()
	if recvClass != interactionClass {
		t.Fatalf("expected interaction class %d, got %d", interactionClass, recvClass)
	}
	if len(recvParams) != 2 {
		t.Fatalf("expected 2 parameters received, got %d", len(recvParams))
	}
	if !bytes.Equal(recvParams[101], params[101]) {
		t.Fatalf("param 101 mismatch: expected %q, got %q", string(params[101]), string(recvParams[101]))
	}
	if !bytes.Equal(recvParams[102], params[102]) {
		t.Fatalf("param 102 mismatch: expected %q, got %q", string(params[102]), string(recvParams[102]))
	}
	if !bytes.Equal(recvTag, tag) {
		t.Fatalf("tag mismatch: expected %q, got %q", string(tag), string(recvTag))
	}
}

func TestCABI_SynchronizationPointLifecycleAndCallbacks(t *testing.T) {
	Bridge_ResetState()

	// 1. Start ephemeral CRC server
	server := crc.NewServer("127.0.0.1:0")
	if err := server.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer server.Stop()

	crcAddr := server.Addr().String()
	fedName := "CABI_SyncPoint_Test_Federation"

	// 2. Setup Sender & Receiver
	sender := CABI_CreateAmbassador()
	defer CABI_DestroyAmbassador(sender)

	receiver := CABI_CreateAmbassador()
	defer CABI_DestroyAmbassador(receiver)

	if code := CABI_SetTestCallbacks(sender); code != ValhallaOK {
		t.Fatalf("set test callbacks failed on sender: %d", code)
	}
	if code := CABI_SetTestCallbacks(receiver); code != ValhallaOK {
		t.Fatalf("set test callbacks failed on receiver: %d", code)
	}

	if code := CABI_Connect(sender, crcAddr); code != ValhallaOK {
		t.Fatalf("sender connect failed: %d", code)
	}
	defer CABI_Disconnect(sender)

	if code := CABI_Connect(receiver, crcAddr); code != ValhallaOK {
		t.Fatalf("receiver connect failed: %d", code)
	}
	defer CABI_Disconnect(receiver)

	if code := CABI_CreateFederation(sender, fedName); code != ValhallaOK {
		t.Fatalf("create federation failed: %d", code)
	}
	defer CABI_DestroyFederation(sender, fedName)

	if _, code := CABI_JoinFederation(sender, "CABISyncPublisher", fedName); code != ValhallaOK {
		t.Fatalf("sender join failed: %d", code)
	}

	if _, code := CABI_JoinFederation(receiver, "CABISyncSubscriber", fedName); code != ValhallaOK {
		t.Fatalf("receiver join failed: %d", code)
	}

	// 3. Register Synchronization Point
	syncLabel := "MissionPhaseInit"
	syncTag := []byte("SyncTagAlpha")

	if code := CABI_RegisterFederationSyncPoint(sender, syncLabel, syncTag); code != ValhallaOK {
		t.Fatalf("register sync point failed: %d", code)
	}

	time.Sleep(50 * time.Millisecond)

	var evoked bool
	// Sender evokes callback -> receives onSyncPointRegistrationSucceeded
	for i := 0; i < 5; i++ {
		evoked, _ = CABI_EvokeCallback(sender, 0.1)
		if !evoked {
			break
		}
		if succLabel := Bridge_GetSyncRegSucceeded(); succLabel == syncLabel {
			break
		}
	}

	if succLabel := Bridge_GetSyncRegSucceeded(); succLabel != syncLabel {
		t.Fatalf("expected onSyncPointRegistrationSucceeded for %q, got %q", syncLabel, succLabel)
	}

	// Duplicate registration attempt must fail and invoke onSyncPointRegistrationFailed with real reason
	if code := CABI_RegisterFederationSyncPoint(sender, syncLabel, syncTag); code == ValhallaOK {
		t.Fatalf("expected duplicate register sync point to fail, got %d", code)
	}
	for i := 0; i < 5; i++ {
		evoked, _ = CABI_EvokeCallback(sender, 0.05)
		if !evoked {
			break
		}
		if failLabel, _ := Bridge_GetSyncRegFailed(); failLabel != "" {
			break
		}
	}
	failLabel, failReason := Bridge_GetSyncRegFailed()
	if failLabel != syncLabel {
		t.Fatalf("expected onSyncPointRegistrationFailed for %q, got %q", syncLabel, failLabel)
	}
	if !strings.Contains(failReason, "not unique") {
		t.Fatalf("expected real reason containing 'not unique', got %q", failReason)
	}

	// Receiver evokes callback -> receives onAnnounceSynchronizationPoint
	evoked, _ = CABI_EvokeCallback(receiver, 0.1)
	if !evoked {
		t.Fatalf("expected callback evoked on receiver for sync point announce")
	}

	annLabel, annTag := Bridge_GetAnnouncedSyncPoint()
	if annLabel != syncLabel {
		t.Fatalf("expected onAnnounceSynchronizationPoint for %q, got %q", syncLabel, annLabel)
	}
	if !bytes.Equal(annTag, syncTag) {
		t.Fatalf("expected announced tag %q, got %q", string(syncTag), string(annTag))
	}

	// 4. Both achieve sync point
	if code := CABI_SyncPointAchieved(sender, syncLabel); code != ValhallaOK {
		t.Fatalf("sender achieve sync point failed: %d", code)
	}
	if code := CABI_SyncPointAchieved(receiver, syncLabel); code != ValhallaOK {
		t.Fatalf("receiver achieve sync point failed: %d", code)
	}

	time.Sleep(50 * time.Millisecond)

	// Both evoke callbacks -> receives onFederationSynchronized
	_, _ = CABI_EvokeCallback(sender, 0.1)
	_, _ = CABI_EvokeCallback(receiver, 0.1)

	if fedLabel := Bridge_GetFederationSynchronized(); fedLabel != syncLabel {
		t.Fatalf("expected onFederationSynchronized for %q, got %q", syncLabel, fedLabel)
	}
}
