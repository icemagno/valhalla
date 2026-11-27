package crc

import (
	"testing"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc/core"
)

type recordSession struct {
	id          int32
	proto       ProtocolType
	notifs      []uint8
	payloads    [][]byte
	merkleRoots [][32]byte
}

func (s *recordSession) FederateID() int32       { return s.id }
func (s *recordSession) Protocol() ProtocolType  { return s.proto }
func (s *recordSession) Close() error            { return nil }
func (s *recordSession) SendNotification(msgType uint8, payload []byte) error {
	s.notifs = append(s.notifs, msgType)
	s.payloads = append(s.payloads, payload)
	s.merkleRoots = append(s.merkleRoots, [32]byte{})
	return nil
}
func (s *recordSession) SendNotificationWithMerkle(msgType uint8, payload []byte, root [32]byte) error {
	s.notifs = append(s.notifs, msgType)
	s.payloads = append(s.payloads, payload)
	s.merkleRoots = append(s.merkleRoots, root)
	return nil
}

func setupTestFederation(t *testing.T) (*FederationExecution, *recordSession, *recordSession) {
	t.Helper()
	fed := NewFederationExecution(1, "DomainTestFed")

	// Setup mock OMT classes
	cls, err := fed.OMT().AddObjectClass("TestClass", core.HLAobjectRootHandle)
	if err != nil {
		t.Fatalf("AddObjectClass failed: %v", err)
	}
	if _, err := fed.OMT().AddAttribute(cls.Handle, "Attr1", "HLAunicodeString"); err != nil {
		t.Fatalf("AddAttribute Attr1 failed: %v", err)
	}
	if _, err := fed.OMT().AddAttribute(cls.Handle, "Attr2", "HLAunicodeString"); err != nil {
		t.Fatalf("AddAttribute Attr2 failed: %v", err)
	}
	if _, err := fed.OMT().AddInteractionClass("TestInteraction", core.HLAinteractionRootHandle); err != nil {
		t.Fatalf("AddInteractionClass failed: %v", err)
	}

	sess1 := &recordSession{id: 1, proto: ProtocolNative}
	sess2 := &recordSession{id: 2, proto: ProtocolNative}

	if _, err := fed.Join("Fed1", "native", ProtocolNative, sess1); err != nil {
		t.Fatalf("Join Fed1 failed: %v", err)
	}
	if _, err := fed.Join("Fed2", "native", ProtocolNative, sess2); err != nil {
		t.Fatalf("Join Fed2 failed: %v", err)
	}

	return fed, sess1, sess2
}

func TestDomainService_RegisterAndBroadcastObject(t *testing.T) {
	fed, _, sess2 := setupTestFederation(t)

	cls, _ := fed.OMT().GetObjectClassByName("TestClass")
	obj, err := fed.RegisterAndBroadcastObject(int32(cls.Handle), "Entity1", 1)
	if err != nil {
		t.Fatalf("RegisterAndBroadcastObject failed: %v", err)
	}
	if obj.Handle <= 0 || obj.Name != "Entity1" {
		t.Fatalf("unexpected object: %+v", obj)
	}

	// Fed2 must have received DiscoverObject notification
	if len(sess2.notifs) != 1 || sess2.notifs[0] != common.NativeMsgDiscoverObject {
		t.Fatalf("expected DiscoverObject notification for Fed2, got %v", sess2.notifs)
	}

	var disc common.DiscoverObjectInstancePayload
	if err := disc.UnmarshalBinary(sess2.payloads[0]); err != nil {
		t.Fatalf("failed to unmarshal DiscoverObjectInstancePayload: %v", err)
	}
	if int32(disc.ObjectHandle) != obj.Handle || disc.ObjectName != "Entity1" {
		t.Fatalf("unexpected discovery payload: %+v", disc)
	}
}

func TestDomainService_DeleteAndBroadcastObject(t *testing.T) {
	fed, _, sess2 := setupTestFederation(t)

	cls, _ := fed.OMT().GetObjectClassByName("TestClass")
	obj, err := fed.RegisterAndBroadcastObject(int32(cls.Handle), "EntityToDelete", 1)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Adversarial: non-owner (Fed2) attempts deletion -> must fail with ErrDeletePrivilegeNotHeld
	_, err = fed.DeleteAndBroadcastObject(obj.Handle, 2, []byte("tag"))
	if err == nil || err != common.ErrDeletePrivilegeNotHeld {
		t.Fatalf("expected ErrDeletePrivilegeNotHeld, got %v", err)
	}

	// Owner (Fed1) deletes -> must succeed
	delObj, err := fed.DeleteAndBroadcastObject(obj.Handle, 1, []byte("deleteTag"))
	if err != nil {
		t.Fatalf("owner delete failed: %v", err)
	}
	if delObj.Handle != obj.Handle {
		t.Fatalf("expected deleted object handle %d, got %d", obj.Handle, delObj.Handle)
	}

	// Fed2 must have received RemoveObjectInstance notification
	foundRemove := false
	for i, n := range sess2.notifs {
		if n == common.NativeMsgRemoveObjectInstance {
			foundRemove = true
			var rem common.RemoveObjectInstancePayload
			if err := rem.UnmarshalBinary(sess2.payloads[i]); err != nil {
				t.Fatalf("failed to unmarshal RemoveObjectInstancePayload: %v", err)
			}
			if int32(rem.ObjectHandle) != obj.Handle || string(rem.Tag) != "deleteTag" {
				t.Fatalf("unexpected remove payload: %+v", rem)
			}
		}
	}
	if !foundRemove {
		t.Fatalf("expected RemoveObjectInstance notification on Fed2")
	}
}

func TestDomainService_UpdateAndRouteAttributes(t *testing.T) {
	fed, _, sess2 := setupTestFederation(t)

	cls, _ := fed.OMT().GetObjectClassByName("TestClass")
	obj, err := fed.RegisterAndBroadcastObject(int32(cls.Handle), "EntityUpdate", 1)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	attr1, _ := fed.OMT().ResolveAttributeByName(cls.Handle, "Attr1")

	// Fed2 subscribes to Attr1
	if err := fed.Router().SubscribeObjectClass(2, cls.Handle, []common.AttributeHandle{attr1.Handle}, true); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Clear setup notifications
	sess2.notifs = nil
	sess2.payloads = nil

	// Fed1 updates Attr1
	attrs := map[common.AttributeHandle][]byte{
		attr1.Handle: []byte("valhalla-state-42"),
	}
	root, err := fed.UpdateAndRouteAttributes(1, common.ObjectInstanceHandle(obj.Handle), attrs, []byte("tag1"), false, 0, nil, [32]byte{})
	if err != nil {
		t.Fatalf("UpdateAndRouteAttributes failed: %v", err)
	}
	if root == [32]byte{} {
		t.Fatalf("expected non-zero Merkle root")
	}

	// Fed2 must have received ReflectAttributes
	if len(sess2.notifs) != 1 || sess2.notifs[0] != common.NativeMsgReflectAttributes {
		t.Fatalf("expected ReflectAttributes notification, got %v", sess2.notifs)
	}

	// Adversarial: non-owner updates without acquisition -> must fail
	_, err = fed.UpdateAndRouteAttributes(2, common.ObjectInstanceHandle(obj.Handle), attrs, nil, false, 0, nil, [32]byte{})
	if err == nil || err != core.ErrNotAttributeOwner {
		t.Fatalf("expected ErrNotAttributeOwner, got %v", err)
	}

	// Adversarial: update non-existent object -> must fail with ErrObjectInstanceNotKnown
	_, err = fed.UpdateAndRouteAttributes(1, 999999, attrs, nil, false, 0, nil, [32]byte{})
	if err == nil || err != common.ErrObjectInstanceNotKnown {
		t.Fatalf("expected ErrObjectInstanceNotKnown, got %v", err)
	}
}

func TestDomainService_RouteInteraction(t *testing.T) {
	fed, _, sess2 := setupTestFederation(t)

	interCls, _ := fed.OMT().GetInteractionClassByName("TestInteraction")

	// Fed2 subscribes to TestInteraction
	if err := fed.Router().SubscribeInteractionClass(2, interCls.Handle, true); err != nil {
		t.Fatalf("SubscribeInteractionClass failed: %v", err)
	}

	sess2.notifs = nil
	sess2.payloads = nil

	params := map[common.ParameterHandle][]byte{
		1: []byte("param-data"),
	}

	if err := fed.RouteInteraction(1, interCls.Handle, params, []byte("interTag"), false, 0); err != nil {
		t.Fatalf("RouteInteraction failed: %v", err)
	}

	if len(sess2.notifs) != 1 || sess2.notifs[0] != common.NativeMsgReceiveInteraction {
		t.Fatalf("expected ReceiveInteraction, got %v", sess2.notifs)
	}

	var rec common.SendInteractionPayload
	if err := rec.UnmarshalBinary(sess2.payloads[0]); err != nil {
		t.Fatalf("failed to unmarshal SendInteractionPayload: %v", err)
	}
	if rec.ClassHandle != interCls.Handle || string(rec.Tag) != "interTag" {
		t.Fatalf("unexpected interaction payload: %+v", rec)
	}
}

func TestDomainService_SyncPointAnnounceAndSynchronized(t *testing.T) {
	fed, sess1, sess2 := setupTestFederation(t)

	sess1.notifs = nil
	sess2.notifs = nil

	if err := fed.BroadcastSyncPointAnnounce("Ready", []byte("syncTag")); err != nil {
		t.Fatalf("BroadcastSyncPointAnnounce failed: %v", err)
	}

	// Both Fed1 and Fed2 must receive AnnounceSyncPoint
	if len(sess1.notifs) != 1 || sess1.notifs[0] != common.NativeMsgAnnounceSyncPoint {
		t.Fatalf("expected AnnounceSyncPoint on Fed1, got %v", sess1.notifs)
	}
	if len(sess2.notifs) != 1 || sess2.notifs[0] != common.NativeMsgAnnounceSyncPoint {
		t.Fatalf("expected AnnounceSyncPoint on Fed2, got %v", sess2.notifs)
	}

	sess1.notifs = nil
	sess2.notifs = nil

	if err := fed.BroadcastFederationSynchronized("Ready"); err != nil {
		t.Fatalf("BroadcastFederationSynchronized failed: %v", err)
	}

	// Both Fed1 and Fed2 must receive FederationSynchronized
	if len(sess1.notifs) != 1 || sess1.notifs[0] != common.NativeMsgFederationSynchronized {
		t.Fatalf("expected FederationSynchronized on Fed1, got %v", sess1.notifs)
	}
	if len(sess2.notifs) != 1 || sess2.notifs[0] != common.NativeMsgFederationSynchronized {
		t.Fatalf("expected FederationSynchronized on Fed2, got %v", sess2.notifs)
	}
}
