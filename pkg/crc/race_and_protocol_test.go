package crc

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/icemagno/valhalla/pkg/common"
)

type mockGrantSession struct {
	id       int32
	protocol ProtocolType
	ch       chan uint8
}

func (m *mockGrantSession) FederateID() int32      { return m.id }
func (m *mockGrantSession) Protocol() ProtocolType { return m.protocol }
func (m *mockGrantSession) SendNotification(msgType uint8, payload []byte) error {
	m.ch <- msgType
	return nil
}
func (m *mockGrantSession) SendNotificationWithMerkle(msgType uint8, payload []byte, _ [32]byte) error {
	m.ch <- msgType
	return nil
}
func (m *mockGrantSession) Close() error { return nil }

func TestWebSession_SendPong_WritesRFC6455PongOpcode(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	ws := NewWebSession(c1)
	done := make(chan error, 1)
	go func() {
		done <- ws.SendPong([]byte("ping-data"))
	}()

	opcode, payload, err := readWebSocketFrame(c2)
	if err != nil {
		t.Fatalf("readWebSocketFrame failed: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("SendPong failed: %v", err)
	}
	if opcode != 0x0A {
		t.Fatalf("expected Pong opcode 0x0A, got 0x%02X", opcode)
	}
	if !bytes.Equal(payload, []byte("ping-data")) {
		t.Fatalf("expected Pong payload 'ping-data', got %q", string(payload))
	}
}

func TestFederation_ResignRegulator_UnblocksConstrainedFederate(t *testing.T) {
	fed := NewFederationExecution(1, "ResignTimeUnblockFed")
	sessA := &mockGrantSession{id: 1, protocol: ProtocolNative, ch: make(chan uint8, 8)}
	sessB := &mockGrantSession{id: 2, protocol: ProtocolNative, ch: make(chan uint8, 8)}

	fedA, err := fed.Join("FedA", "sim", ProtocolNative, sessA)
	if err != nil {
		t.Fatalf("Join FedA failed: %v", err)
	}
	fedB, err := fed.Join("FedB", "sim", ProtocolNative, sessB)
	if err != nil {
		t.Fatalf("Join FedB failed: %v", err)
	}

	if err := fed.TimeManager().EnableTimeRegulation(common.FederateHandle(fedA.ID), 1.0); err != nil {
		t.Fatalf("EnableTimeRegulation failed: %v", err)
	}
	if err := fed.TimeManager().EnableTimeConstrained(common.FederateHandle(fedB.ID)); err != nil {
		t.Fatalf("EnableTimeConstrained failed: %v", err)
	}

	// FedB requests TAR(10.0), blocked because FedA is regulating at t=0, lookahead=1.0 (LBTS=1.0)
	grants, err := fed.TimeManager().TimeAdvanceRequest(common.FederateHandle(fedB.ID), 10.0)
	if err != nil {
		t.Fatalf("TAR failed: %v", err)
	}
	if len(grants) != 0 {
		t.Fatalf("expected FedB to block on TAR(10.0), got %d grants", len(grants))
	}

	// FedA resigns -> FedB must immediately receive TimeAdvanceGrant!
	if _, err := fed.Resign(fedA.ID); err != nil {
		t.Fatalf("Resign FedA failed: %v", err)
	}

	select {
	case msgType := <-sessB.ch:
		if msgType != NativeMsgTimeAdvanceGrant {
			t.Fatalf("expected NativeMsgTimeAdvanceGrant (%d), got %d", NativeMsgTimeAdvanceGrant, msgType)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("deadlock: constrained federate never received TAG after regulator resigned")
	}
}
