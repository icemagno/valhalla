package lrc

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/icemagno/valhalla/pkg/common"
)

func TestClient_ReadLoop_RejectsOversizedPayloadLenWithoutOOM(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		var preamble [4]byte
		if _, err := io.ReadFull(conn, preamble[:]); err != nil {
			return
		}

		// Send malicious NativeHeader claiming 4 GiB payload (0xFFFFFFFF)
		hdr := common.NativeHeader{
			Magic:      common.NativeHeaderMagic,
			Version:    common.NativeProtocolVer,
			Flags:      common.FlagNativeSSZ,
			PayloadLen: 0xFFFFFFFF,
			MsgType:    common.NativeMsgReflectAttributes,
			ReqID:      0,
		}
		raw := common.WriteNativeHeader(hdr)
		_, _ = conn.Write(raw[:])
	}()

	client, err := NewClient(ln.Addr().String())
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer client.Close()

	// Wait deterministically for readLoop to reject the oversized frame and close the client
	select {
	case <-client.closedChan:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected client to disconnect after receiving oversized PayloadLen frame")
	}
	<-serverDone
}
