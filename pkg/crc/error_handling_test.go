package crc

import (
	"io"
	"net"
	"testing"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc/portico/wire"
)

func TestNativeHandler_UpdateObjectAttributes_ErrorPropagation(t *testing.T) {
	srv := NewServer("127.0.0.1:0")
	if err := srv.Start(); err != nil {
		t.Fatalf("Server.Start failed: %v", err)
	}
	defer srv.Stop()

	conn, err := net.Dial("tcp", srv.Addr().String())
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("VAL4")); err != nil {
		t.Fatalf("Write VAL4 failed: %v", err)
	}

	session := NewNativeSession(conn)

	// 1. Create Federation
	createReq := common.FederationExecutionPayload{
		FederationName: "ErrorPropFed",
	}
	createPayload, _ := createReq.MarshalBinary()
	createHdr := NativeHeader{
		Magic:      NativeHeaderMagic,
		Version:    NativeProtocolVer,
		Flags:      0x01,
		PayloadLen: uint32(len(createPayload)),
		MsgType:    NativeMsgCreateFederation,
		ReqID:      1,
	}
	if err := session.SendMessage(createHdr, createPayload); err != nil {
		t.Fatalf("Create send failed: %v", err)
	}
	respHdr, err := ReadNativeHeader(conn)
	if err != nil || respHdr.MsgType != NativeMsgSuccessOK {
		t.Fatalf("Create failed: %v, msgType=%d", err, respHdr.MsgType)
	}

	// 2. Join Federation
	joinReq := common.JoinFederationPayload{
		FederateName:   "Fed1",
		FederateType:   "native",
		FederationName: "ErrorPropFed",
	}
	joinPayload, _ := joinReq.MarshalBinary()
	joinHdr := NativeHeader{
		Magic:      NativeHeaderMagic,
		Version:    NativeProtocolVer,
		Flags:      0x01,
		PayloadLen: uint32(len(joinPayload)),
		MsgType:    NativeMsgJoin,
		ReqID:      2,
	}
	if err := session.SendMessage(joinHdr, joinPayload); err != nil {
		t.Fatalf("Join send failed: %v", err)
	}
	respHdr, err = ReadNativeHeader(conn)
	if err != nil || respHdr.MsgType != NativeMsgJoinSuccess {
		t.Fatalf("Join failed: %v, msgType=%d", err, respHdr.MsgType)
	}
	if respHdr.PayloadLen > 0 {
		buf := make([]byte, respHdr.PayloadLen)
		_, _ = conn.Read(buf)
	}

	// 3. Try to update an object that does NOT exist (handle 999999)
	uavReq := common.UpdateAttributesPayload{
		ObjectHandle: 999999,
		Attributes: map[common.AttributeHandle][]byte{
			1: []byte("val"),
		},
	}
	uavPayload, _ := uavReq.MarshalBinary()
	uavHdr := NativeHeader{
		Magic:      NativeHeaderMagic,
		Version:    NativeProtocolVer,
		Flags:      common.FlagNativeSSZ,
		PayloadLen: uint32(len(uavPayload)),
		MsgType:    NativeMsgUpdateAttributes,
		ReqID:      3,
	}
	if err := session.SendMessage(uavHdr, uavPayload); err != nil {
		t.Fatalf("UAV send failed: %v", err)
	}

	respHdr, err = ReadNativeHeader(conn)
	if err != nil {
		t.Fatalf("Read response failed: %v", err)
	}

	// Must be an error (404), NEVER SuccessOK
	if respHdr.MsgType != NativeMsgError {
		t.Fatalf("Expected NativeMsgError (%d), but got %d", NativeMsgError, respHdr.MsgType)
	}

	errPayload := make([]byte, respHdr.PayloadLen)
	if _, err := io.ReadFull(conn, errPayload); err != nil {
		t.Fatalf("Read error payload failed: %v", err)
	}
	var errResp common.ErrorResponsePayload
	if err := errResp.UnmarshalBinary(errPayload); err != nil {
		t.Fatalf("Unmarshal error response failed: %v", err)
	}
	if errResp.ErrorCode != 404 {
		t.Fatalf("Expected ErrorCode 404 (ObjectInstanceNotKnown), got %d: %s", errResp.ErrorCode, errResp.Message)
	}
}

func TestPorticoHandler_DeleteObjectInstance_ErrorPropagation(t *testing.T) {
	srv := NewServer("127.0.0.1:0")
	if err := srv.Start(); err != nil {
		t.Fatalf("Server.Start failed: %v", err)
	}
	defer srv.Stop()

	// 1. Conectar como Portico
	conn, err := net.Dial("tcp", srv.Addr().String())
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	// Read welcome bundle (8 bytes: 0xCAFE + length)
	buf := make([]byte, 8)
	if _, err := conn.Read(buf); err != nil {
		t.Fatalf("Read welcome failed: %v", err)
	}
	msgBuf := make([]byte, len(PorticoWelcomeMsg))
	if _, err := conn.Read(msgBuf); err != nil {
		t.Fatalf("Read welcome msg failed: %v", err)
	}
	var srvFeed [4]byte
	if _, err := conn.Read(srvFeed[:]); err != nil {
		t.Fatalf("Read 0xFEED failed: %v", err)
	}
	// Reply with client handshake (0xFEED)
	var ready [4]byte
	ready[0], ready[1], ready[2], ready[3] = 0x00, 0x00, 0xFE, 0xED
	if _, err := conn.Write(ready[:]); err != nil {
		t.Fatalf("Write handshake failed: %v", err)
	}

	clientSession := NewPorticoSession(conn)

	readControlResponse := func() (wire.Header, []byte, error) {
		for {
			hdr, payload, err := clientSession.ReadPacket(conn)
			if err != nil {
				return hdr, nil, err
			}
			if hdr.CallType == wire.CallTypeControlResponseOK || hdr.CallType == wire.CallTypeControlResponseErr {
				return hdr, payload, nil
			}
		}
	}

	// Create and join federation
	createObj := wire.NewCreateFederationObject("PorticoErrorFed")
	createPayload, err := wire.SerializePorticoControlMessage(createObj)
	if err != nil {
		t.Fatalf("Serialize create failed: %v", err)
	}
	createHdr := wire.NewControlRequest(wire.MsgCreateFederation, 1, 0, 0, uint32(len(createPayload)))
	if err := clientSession.SendPacket(createHdr, createPayload); err != nil {
		t.Fatalf("Send Create failed: %v", err)
	}
	respHdr, _, err := readControlResponse()
	if err != nil || respHdr.CallType != wire.CallTypeControlResponseOK {
		t.Fatalf("Create federation failed: %v, callType=%v", err, respHdr.CallType)
	}

	// Join
	joinObj := wire.NewJoinFederationObject("PorticoErrorFed", "FedOwner", "ieee1516e")
	joinPayload, err := wire.SerializePorticoControlMessage(joinObj)
	if err != nil {
		t.Fatalf("Serialize join failed: %v", err)
	}
	joinHdr := wire.NewControlRequest(wire.MsgJoinFederation, 2, 0, 0, uint32(len(joinPayload)))
	if err := clientSession.SendPacket(joinHdr, joinPayload); err != nil {
		t.Fatalf("Send Join failed: %v", err)
	}
	respHdr, _, err = readControlResponse()
	if err != nil || respHdr.CallType != wire.CallTypeControlResponseOK {
		t.Fatalf("Join failed: %v", err)
	}

	// Try to delete a non-existent object handle (handle 888888)
	deletePayload, err := wire.BuildDeleteObjectPayload(888888, []byte("tag"))
	if err != nil {
		t.Fatalf("BuildDeleteObjectPayload failed: %v", err)
	}
	deleteHdr := wire.NewControlRequest(wire.MsgDeleteObject, 3, 1, 0, uint32(len(deletePayload)))
	if err := clientSession.SendPacket(deleteHdr, deletePayload); err != nil {
		t.Fatalf("Send Delete failed: %v", err)
	}

	respHdr, _, err = readControlResponse()
	if err != nil {
		t.Fatalf("Read delete response failed: %v", err)
	}

	// Must be ControlResponseErr (callType=4), NOT ControlResponseOK!
	if respHdr.CallType != wire.CallTypeControlResponseErr {
		t.Fatalf("Expected ControlResponseErr (4), got %v", respHdr.CallType)
	}
}
