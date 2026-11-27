package crc

import (
	"errors"
	"testing"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc/core"
)

type noopSession struct {
	id int32
}

func (n *noopSession) FederateID() int32                                         { return n.id }
func (n *noopSession) SetFederateID(id int32)                                    { n.id = id }
func (n *noopSession) Protocol() ProtocolType                                    { return ProtocolNative }
func (n *noopSession) SendNotification(msgType uint8, payload []byte) error      { return nil }
func (n *noopSession) SendNotificationWithMerkle(uint8, []byte, [32]byte) error  { return nil }
func (n *noopSession) Close() error                                              { return nil }

func TestDomainService_TSOLookaheadViolation_ReturnsError(t *testing.T) {
	fed := NewFederationExecution(1, "TSOErrorFed")

	s1 := &noopSession{}
	s2 := &noopSession{}
	f1, err := fed.Join("RegFed", "type", ProtocolNative, s1)
	if err != nil {
		t.Fatalf("Join f1 failed: %v", err)
	}
	f2, err := fed.Join("ConFed", "type", ProtocolNative, s2)
	if err != nil {
		t.Fatalf("Join f2 failed: %v", err)
	}

	cls, err := fed.OMT().AddObjectClass("Unit", core.HLAobjectRootHandle)
	if err != nil {
		t.Fatalf("AddObjectClass failed: %v", err)
	}
	attr, err := fed.OMT().AddAttribute(cls.Handle, "Pos", "HLAoctet")
	if err != nil {
		t.Fatalf("AddAttribute failed: %v", err)
	}

	if err := fed.Router().PublishObjectClass(common.FederateHandle(f1.ID), cls.Handle, []common.AttributeHandle{attr.Handle}); err != nil {
		t.Fatalf("PublishObjectClass failed: %v", err)
	}
	if err := fed.Router().SubscribeObjectClass(common.FederateHandle(f2.ID), cls.Handle, []common.AttributeHandle{attr.Handle}, true); err != nil {
		t.Fatalf("SubscribeObjectClass failed: %v", err)
	}

	// f1 is regulating at t=0, lookahead=2.0 -> min valid TSO timestamp is 2.0
	if err := fed.TimeManager().EnableTimeRegulation(common.FederateHandle(f1.ID), 2.0); err != nil {
		t.Fatalf("EnableTimeRegulation failed: %v", err)
	}
	if err := fed.TimeManager().EnableTimeConstrained(common.FederateHandle(f2.ID)); err != nil {
		t.Fatalf("EnableTimeConstrained failed: %v", err)
	}

	obj, err := fed.RegisterObjectInstance(int32(cls.Handle), "Unit-1", f1.ID)
	if err != nil {
		t.Fatalf("RegisterObjectInstance failed: %v", err)
	}

	// Send TSO update with timestamp=1.0 (< lookahead 2.0) -> MUST return ErrInvalidLogicalTime!
	_, err = fed.UpdateAndRouteAttributes(
		f1.ID,
		common.ObjectInstanceHandle(obj.Handle),
		map[common.AttributeHandle][]byte{attr.Handle: []byte("bad-tso")},
		nil,
		true,
		1.0,
		nil,
		[32]byte{},
	)
	if !errors.Is(err, common.ErrInvalidLogicalTime) {
		t.Fatalf("expected ErrInvalidLogicalTime on lookahead violation, got: %v", err)
	}
}
