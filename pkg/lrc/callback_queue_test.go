package lrc

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/icemagno/valhalla/pkg/common"
)

// TestCallbackQueue_BurstAbsorption proves that an asynchronous callback burst
// exceeding 1,024 items is completely preserved without silent message drops.
func TestCallbackQueue_BurstAbsorption(t *testing.T) {
	// Set up a mock CRC server that sends VAL4 preamble then bursts callbacks
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer ln.Close()

	const burstCount = 3000

	serverBurstDone := make(chan struct{})
	testDone := make(chan struct{})
	go func() {
		defer close(serverBurstDone)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Read VAL4 preamble (4 bytes)
		preamble := make([]byte, 4)
		if _, err := conn.Read(preamble); err != nil {
			return
		}

		// Burst 3,000 asynchronous callbacks (ReqID = 0)
		for i := 0; i < burstCount; i++ {
			payload := []byte(fmt.Sprintf("event-%05d", i))
			hdr := common.NativeHeader{
				Magic:      common.NativeHeaderMagic,
				Version:    common.NativeProtocolVer,
				MsgType:    common.NativeMsgReflectAttributes,
				PayloadLen: uint32(len(payload)),
				ReqID:      0, // Asynchronous notification
			}
			hBytes := common.WriteNativeHeader(hdr)
			if _, err := conn.Write(hBytes[:]); err != nil {
				return
			}
			if _, err := conn.Write(payload); err != nil {
				return
			}
		}

		// Signal burst sent, but keep conn open until consumer finishes
		<-testDone
	}()

	client, err := NewClient(ln.Addr().String())
	if err != nil {
		t.Fatalf("Failed to connect client: %v", err)
	}
	defer client.Close()
	defer close(testDone)

	// Wait for server to finish bursting all 3,000 messages over the wire
	time.Sleep(300 * time.Millisecond)

	// Give readLoop a moment to ingest TCP buffer
	time.Sleep(200 * time.Millisecond)

	// Now consume from client.callbacks (or via EvokeCallback)
	received := 0
	deadline := time.After(3 * time.Second)
consumeLoop:
	for received < burstCount {
		select {
		case item := <-client.callbacks:
			expectedPayload := fmt.Sprintf("event-%05d", received)
			if string(item.Payload) != expectedPayload {
				t.Fatalf("Payload mismatch at index %d: expected %s, got %s", received, expectedPayload, string(item.Payload))
			}
			received++
		case <-deadline:
			break consumeLoop
		}
	}

	if received != burstCount {
		t.Fatalf("SILENT DROP DETECTED: Expected %d callbacks, but only received %d (dropped %d messages!)",
			burstCount, received, burstCount-received)
	}
}

// TestCallbackQueue_NoDeadlockWithSynchronousRequests ensures that synchronous requests
// (e.g. SendRequest) are not blocked or deadlocked when the callback queue has thousands
// of pending unevoked items.
func TestCallbackQueue_NoDeadlockWithSynchronousRequests(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer ln.Close()

	const burstCount = 2000

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		preamble := make([]byte, 4)
		_, _ = conn.Read(preamble)

		// 1. Send burst of async callbacks
		for i := 0; i < burstCount; i++ {
			hdr := common.NativeHeader{
				Magic:      common.NativeHeaderMagic,
				Version:    common.NativeProtocolVer,
				MsgType:    common.NativeMsgReflectAttributes,
				PayloadLen: 0,
				ReqID:      0,
			}
			hBytes := common.WriteNativeHeader(hdr)
			_, _ = conn.Write(hBytes[:])
		}

		// 2. Handle incoming synchronous request
		for {
			reqHdr, err := common.ReadNativeHeader(conn)
			if err != nil {
				return
			}
			if reqHdr.PayloadLen > 0 {
				p := make([]byte, reqHdr.PayloadLen)
				_, _ = conn.Read(p)
			}
			// Send response for this ReqID
			respHdr := common.NativeHeader{
				Magic:      common.NativeHeaderMagic,
				Version:    common.NativeProtocolVer,
				MsgType:    common.NativeMsgSuccessOK,
				PayloadLen: 0,
				ReqID:      reqHdr.ReqID,
			}
			rBytes := common.WriteNativeHeader(respHdr)
			_, _ = conn.Write(rBytes[:])
		}
	}()

	client, err := NewClient(ln.Addr().String())
	if err != nil {
		t.Fatalf("Failed to connect client: %v", err)
	}
	defer client.Close()

	// Give time for async burst to be transmitted
	time.Sleep(200 * time.Millisecond)

	// While 2,000 callbacks are buffered and NOT evoked, issue a synchronous request
	start := time.Now()
	resp, err := client.SendRequest(common.NativeMsgHeartbeat, 1, 1, nil, 2*time.Second)
	if err != nil {
		t.Fatalf("SendRequest failed during callback backlog: %v", err)
	}
	if resp.Header.MsgType != common.NativeMsgSuccessOK {
		t.Fatalf("Expected NativeMsgSuccessOK, got %d", resp.Header.MsgType)
	}
	if time.Since(start) > 1*time.Second {
		t.Fatalf("SendRequest took excessively long (%v), possible lock contention or stall", time.Since(start))
	}
}

// TestCallbackQueue_SaturationCeilingExplicitReporting verifies that when an explicit
// safety capacity limit is reached, drops are NOT silent:
// 1. IsQueueSaturated() becomes true
// 2. DroppedCallbackCount() tracks exact drop count
// 3. EvokeCallback returns ErrCallbackQueueSaturated
func TestCallbackQueue_SaturationCeilingExplicitReporting(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer ln.Close()

	const maxCap = 50
	const totalSent = 120
	const expectedDropped = totalSent - maxCap

	serverBurstDone := make(chan struct{})
	testDone := make(chan struct{})
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		preamble := make([]byte, 4)
		if _, err := conn.Read(preamble); err != nil {
			return
		}

		for i := 0; i < totalSent; i++ {
			hdr := common.NativeHeader{
				Magic:      common.NativeHeaderMagic,
				Version:    common.NativeProtocolVer,
				MsgType:    common.NativeMsgReflectAttributes,
				PayloadLen: 0,
				ReqID:      0,
			}
			hBytes := common.WriteNativeHeader(hdr)
			if _, err := conn.Write(hBytes[:]); err != nil {
				return
			}
		}

		close(serverBurstDone)
		<-testDone
	}()

	client, err := NewClient(ln.Addr().String())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()
	defer close(testDone)

	// Configure small capacity to trigger saturation
	client.SetMaxQueueCapacity(maxCap)

	// Wait for transmission
	select {
	case <-serverBurstDone:
	case <-time.After(3 * time.Second):
		t.Fatalf("Timeout waiting for mock server burst transmission")
	}

	time.Sleep(100 * time.Millisecond)

	if !client.IsQueueSaturated() {
		t.Fatalf("Expected IsQueueSaturated() to be true, got false")
	}

	dropped := client.DroppedCallbackCount()
	if dropped != expectedDropped {
		t.Fatalf("Expected %d dropped callbacks, got %d", expectedDropped, dropped)
	}

	// Verify ambassador integration with EvokeCallback
	amb := &RTIambassadorImpl{
		client: client,
	}

	// Evoking when saturated must return ErrCallbackQueueSaturated
	evoked, err := amb.EvokeCallback(100 * time.Millisecond)
	if !evoked {
		t.Fatalf("Expected callback to be evoked from buffer")
	}
	if err != ErrCallbackQueueSaturated {
		t.Fatalf("Expected ErrCallbackQueueSaturated, got %v", err)
	}

	length, saturated, dropCount := amb.GetCallbackQueueStatus()
	if !saturated {
		t.Fatalf("GetCallbackQueueStatus saturated flag mismatch")
	}
	if dropCount != expectedDropped {
		t.Fatalf("GetCallbackQueueStatus dropCount mismatch: %d != %d", dropCount, expectedDropped)
	}
	if length > maxCap {
		t.Fatalf("Queue length %d exceeded maxCap %d", length, maxCap)
	}

	// Reset saturation and verify it clears
	amb.ResetCallbackQueueSaturation()
	if client.IsQueueSaturated() {
		t.Fatalf("Expected IsQueueSaturated() to be false after reset")
	}
	if client.DroppedCallbackCount() != 0 {
		t.Fatalf("Expected DroppedCallbackCount() to be 0 after reset")
	}
}
