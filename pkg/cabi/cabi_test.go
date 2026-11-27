// Project Valhalla - Sovereign HLA-4 Run-Time Infrastructure
// Lead Developer: Carlos Magno Oliveira de Abreu (@icemagno / magno.mabreu@gmail.com)

package main

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc"
	"github.com/icemagno/valhalla/pkg/crc/core"
	"github.com/icemagno/valhalla/pkg/lrc"
)

const (
	ValhallaOK             = 0
	ValhallaErrNotConnected = 1
	ValhallaErrInvalidArg  = 2
	ValhallaErrRtiException = 3
	ValhallaErrNotOwner    = 4
	ValhallaErrFederationExists = 6
	ValhallaErrFederationNotFound = 7
	ValhallaErrClassNotFound = 8
	ValhallaErrAttrNotFound = 9
	ValhallaErrObjectNotFound = 10
	ValhallaErrNotMember   = 11
	ValhallaErrFederatesJoined = 12
	ValhallaErrFederateNameInUse = 13
	ValhallaErrInteractionNotFound = 14
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

	nullAmbResults := CABI_NullAmbassadorChecks()
	for i, code := range nullAmbResults {
		if code != ValhallaErrInvalidArg {
			t.Fatalf("null ambassador check %d returned code %d, expected %d (VALHALLA_ERR_INVALID_ARG)", i, code, ValhallaErrInvalidArg)
		}
	}
}

func TestCABI_TypedErrorMapping(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		expected int32
	}{
		{"nil error", nil, ValhallaOK},
		{"direct federation exists", common.ErrFederationExecutionAlreadyExists, ValhallaErrFederationExists},
		{"wrapped federation exists", fmt.Errorf("context: %w", common.ErrFederationExecutionAlreadyExists), ValhallaErrFederationExists},
		{"direct federation not found", common.ErrFederationExecutionDoesNotExist, ValhallaErrFederationNotFound},
		{"wrapped federation not found", fmt.Errorf("rpc failure: %w", common.ErrFederationExecutionDoesNotExist), ValhallaErrFederationNotFound},
		{"direct federates joined", common.ErrFederatesCurrentlyJoined, ValhallaErrFederatesJoined},
		{"wrapped federates joined", fmt.Errorf("destroy failed: %w", common.ErrFederatesCurrentlyJoined), ValhallaErrFederatesJoined},
		{"direct federate name in use", common.ErrFederateNameAlreadyInUse, ValhallaErrFederateNameInUse},
		{"wrapped federate name in use", fmt.Errorf("join failed: %w", common.ErrFederateNameAlreadyInUse), ValhallaErrFederateNameInUse},
		{"direct class not defined", common.ErrObjectClassNotDefined, ValhallaErrClassNotFound},
		{"wrapped core class not found", fmt.Errorf("metamodel: %w", core.ErrClassNotFound), ValhallaErrClassNotFound},
		{"direct attr not defined", common.ErrAttributeNotDefined, ValhallaErrAttrNotFound},
		{"wrapped core attr not found", fmt.Errorf("omt: %w", core.ErrAttributeNotFound), ValhallaErrAttrNotFound},
		{"direct interaction not defined", common.ErrInteractionClassNotDefined, ValhallaErrInteractionNotFound},
		{"wrapped core interaction not found", fmt.Errorf("fom: %w", core.ErrInteractionNotFound), ValhallaErrInteractionNotFound},
		{"direct object not known", common.ErrObjectInstanceNotKnown, ValhallaErrObjectNotFound},
		{"wrapped object not known", fmt.Errorf("reflect error: %w", common.ErrObjectInstanceNotKnown), ValhallaErrObjectNotFound},
		{"direct attribute not owned", common.ErrAttributeNotOwned, ValhallaErrNotOwner},
		{"wrapped delete priv not held", fmt.Errorf("delete failed: %w", common.ErrDeletePrivilegeNotHeld), ValhallaErrNotOwner},
		{"direct not member", common.ErrFederateNotExecutionMember, ValhallaErrNotMember},
		{"wrapped lrc not member", fmt.Errorf("membership required: %w", lrc.ErrFederateNotExecutionMember), ValhallaErrNotMember},
		{"wrapped lrc already member", fmt.Errorf("cannot join twice: %w", lrc.ErrFederateAlreadyExecutionMember), ValhallaErrNotMember},
		{"direct not connected", lrc.ErrNotConnected, ValhallaErrNotConnected},
		{"wrapped connection closed", fmt.Errorf("socket EOF: %w", lrc.ErrConnectionClosed), ValhallaErrNotConnected},
		{"invalid parent class", core.ErrInvalidParentClass, ValhallaErrInvalidArg},
		{"ambiguous class", core.ErrAmbiguousClass, ValhallaErrInvalidArg},
		{"ambiguous interaction", core.ErrAmbiguousInteraction, ValhallaErrInvalidArg},
		{"invalid logical time", common.ErrInvalidLogicalTime, ValhallaErrInvalidArg},
		{"invalid lookahead", common.ErrInvalidLookahead, ValhallaErrInvalidArg},
		{"unknown custom error falls back to rti exception", errors.New("some unexpected internal panic"), ValhallaErrRtiException},
	}

	for _, tc := range cases {
		code := CABI_TestMapError(tc.err)
		if code != tc.expected {
			t.Errorf("error mapping for %s: got code %d, expected %d", tc.name, code, tc.expected)
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

func TestCABI_Heartbeat(t *testing.T) {
	server := crc.NewServer("127.0.0.1:0")
	if err := server.Start(); err != nil {
		t.Fatalf("failed to start CRC server: %v", err)
	}
	defer server.Stop()

	crcAddr := server.Addr().String()
	amb := CABI_CreateAmbassador()
	if amb == 0 {
		t.Fatalf("failed to create ambassador")
	}
	defer CABI_DestroyAmbassador(amb)

	// Heartbeat before connect must fail with NOT_CONNECTED
	if code := CABI_Heartbeat(amb); code != ValhallaErrNotConnected {
		t.Fatalf("expected VALHALLA_ERR_NOT_CONNECTED before connect, got %d", code)
	}

	if code := CABI_Connect(amb, crcAddr); code != ValhallaOK {
		t.Fatalf("connect failed: %d", code)
	}
	defer CABI_Disconnect(amb)

	// Heartbeat before joining must fail with NOT_MEMBER
	if code := CABI_Heartbeat(amb); code != ValhallaErrNotMember {
		t.Fatalf("expected VALHALLA_ERR_NOT_MEMBER before join, got %d", code)
	}

	fedName := "CABI_Heartbeat_Fed"
	_ = CABI_CreateFederation(amb, fedName)

	h, code := CABI_JoinFederation(amb, "HeartbeatCABI", fedName)
	if code != ValhallaOK || h <= 0 {
		t.Fatalf("join failed: %d (handle: %d)", code, h)
	}

	// Heartbeat after join must succeed
	if code := CABI_Heartbeat(amb); code != ValhallaOK {
		t.Fatalf("expected CABI_Heartbeat to succeed, got %d", code)
	}
}
