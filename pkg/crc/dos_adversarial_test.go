package crc

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"

	"github.com/icemagno/valhalla/pkg/common"
)

func TestWebSocket_RejectsOversizedPayloadBeforeAlloc(t *testing.T) {
	// Frame: FIN=1, opcode=1 (text), MASK=0, lenByte=127 (64-bit length), payloadLen = 1 << 50
	var buf bytes.Buffer
	buf.Write([]byte{0x81, 127})
	var lenBuf [8]byte
	binary.BigEndian.PutUint64(lenBuf[:], uint64(1)<<50)
	buf.Write(lenBuf[:])

	_, _, err := readWebSocketFrame(&buf)
	if err == nil {
		t.Fatalf("expected error for oversized WebSocket payloadLen, got nil")
	}
}

func TestNativeHandler_RejectsOversizedPayloadBeforeAlloc(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	var hdr common.NativeHeader
	hdr.Magic = common.NativeHeaderMagic
	hdr.Version = common.NativeProtocolVer
	hdr.MsgType = common.NativeMsgHeartbeat
	hdr.PayloadLen = common.MaxNativePayloadLen + 1

	var buf bytes.Buffer
	hdrBytes := common.WriteNativeHeader(hdr)
	buf.Write(hdrBytes[:])

	fm := NewFederationManager()
	err := HandleNativeConnection(serverConn, &buf, fm)
	if err == nil {
		t.Fatalf("expected error for Native payload exceeding MaxNativePayloadLen, got nil")
	}
}
