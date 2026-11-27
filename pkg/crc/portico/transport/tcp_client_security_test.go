package transport

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"github.com/icemagno/valhalla/pkg/crc/portico/wire"
)

func TestTCPClient_SendPacket_FailedMarshalDoesNotCorruptStream(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	received := make(chan []byte, 1)
	errCh := make(chan error, 1)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			errCh <- err
			return
		}
		defer conn.Close()

		// Perform server-side Portico handshake (0xBEEF + msg + 0xFEED, expect 0xFEED)
		_ = binary.Write(conn, binary.BigEndian, MagicWelcome)
		msg := "Welcome"
		_ = binary.Write(conn, binary.BigEndian, int32(len(msg)))
		_, _ = conn.Write([]byte(msg))
		_ = binary.Write(conn, binary.BigEndian, MagicReady)
		var clientReady int32
		if err := binary.Read(conn, binary.BigEndian, &clientReady); err != nil {
			errCh <- err
			return
		}

		// Now read the next TCP bundle: must be the VALID packet, with zero orphan bytes
		var magic int32
		if err := binary.Read(conn, binary.BigEndian, &magic); err != nil {
			errCh <- err
			return
		}
		if magic != MagicTCPBundle {
			errCh <- ErrInvalidBundleMagic
			return
		}
		var bundleLen int32
		if err := binary.Read(conn, binary.BigEndian, &bundleLen); err != nil {
			errCh <- err
			return
		}
		if bundleLen != int32(wire.HeaderLength+5) {
			errCh <- io.ErrUnexpectedEOF
			return
		}
		frame := make([]byte, bundleLen)
		if _, err := io.ReadFull(conn, frame); err != nil {
			errCh <- err
			return
		}
		received <- frame[wire.HeaderLength:]
	}()

	client := NewTCPClient(ln.Addr().String())
	if err := client.Connect(2 * time.Second); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	// 1. Send an oversized payload (> 24-bit max 0x00FFFFFF) so MarshalTo fails
	oversized := make([]byte, 0x01000000)
	err = client.SendPacket(wire.Header{MessageType: 1}, oversized)
	if err == nil {
		t.Fatalf("expected error for oversized payload > 0x00FFFFFF, got nil")
	}

	// 2. Send a valid 5-byte payload on the SAME connection; stream must remain intact
	validPayload := []byte("HELLO")
	if err := client.SendPacket(wire.Header{MessageType: 1}, validPayload); err != nil {
		t.Fatalf("valid SendPacket failed after rejected oversized packet: %v", err)
	}

	select {
	case err := <-errCh:
		t.Fatalf("peer detected stream corruption: %v", err)
	case got := <-received:
		if !bytes.Equal(got, validPayload) {
			t.Fatalf("expected %q, got %q", validPayload, got)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for valid packet on peer")
	}
}

func TestTCPClient_Connect_HandshakeRespectsTimeout(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	done := make(chan struct{})
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		// Intentionally never send 0xBEEF handshake
		<-done
	}()
	defer close(done)

	client := NewTCPClient(ln.Addr().String())
	start := time.Now()
	err = client.Connect(150 * time.Millisecond)
	elapsed := time.Since(start)
	if err == nil {
		_ = client.Close()
		t.Fatalf("expected handshake timeout error, got nil")
	}
	if elapsed > 1*time.Second {
		t.Fatalf("Connect took %v, expected <= 1s for 150ms timeout", elapsed)
	}
}
