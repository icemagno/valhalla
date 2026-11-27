package integration_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc"
	"github.com/icemagno/valhalla/pkg/crc/portico/transport"
	"github.com/icemagno/valhalla/pkg/crc/portico/wire"
	"github.com/icemagno/valhalla/pkg/crc/portico/wire/metamodel"
	"github.com/icemagno/valhalla/pkg/lrc"
)

// TestCrossProtocol_NativeToPortico_DeleteObject verifies that when a Native federate
// deletes an object, a connected Portico federate receives official wire.MsgDeleteObject (64)
// and valid Portico Java serialization, NOT alien message code 47 with SSZ bytes.
func TestCrossProtocol_NativeToPortico_DeleteObject(t *testing.T) {
	server := crc.NewServer("127.0.0.1:0")
	if err := server.Start(); err != nil {
		t.Fatalf("failed to start CRC: %v", err)
	}
	defer server.Stop()

	crcAddr := server.Addr().String()
	fedName := "CrossProtocolDeleteFed"

	// 1. Connect Portico client (Thor)
	clientThor := transport.NewTCPClient(crcAddr)
	if err := clientThor.Connect(3 * time.Second); err != nil {
		t.Fatalf("failed to connect Thor: %v", err)
	}
	defer clientThor.Close()

	createObj := wire.NewCreateFederationObject(fedName)
	createPayload, _ := wire.SerializePorticoControlMessage(createObj)
	_ = clientThor.SendPacket(wire.NewControlRequest(wire.MsgCreateFederation, 1, 0, 0, uint32(len(createPayload))), createPayload)
	_, _, _ = clientThor.ReadPacketTimeout(2 * time.Second)

	joinObj := wire.NewJoinFederationObject(fedName, "ThorMember", "ThorType")
	joinPayload, _ := wire.SerializePorticoControlMessage(joinObj)
	_ = clientThor.SendPacket(wire.NewControlRequest(wire.MsgJoinFederation, 2, 0, 0, uint32(len(joinPayload))), joinPayload)
	_, welcomePayload, _ := clientThor.ReadPacketTimeout(2 * time.Second)
	wpInfo, _ := wire.ParseWelcomePack(welcomePayload)

	// Thor subscribes to HLAobjectRoot to receive DiscoverObject and RemoveObjectInstance
	subObj := metamodel.NewSubscribeObjectClass(int32(common.HLAobjectRootHandle), []any{metamodel.NewJavaInteger(int32(common.HLAprivilegeToDeleteObjectHandle))})
	subPayload, _ := wire.SerializePorticoControlMessage(subObj)
	_ = clientThor.SendPacket(wire.NewControlRequest(wire.MsgSubscribeObjectClass, 3, uint16(wpInfo.FederateHandle), 0, uint32(len(subPayload))), subPayload)
	_, _, _ = clientThor.ReadPacketTimeout(2 * time.Second)

	// 2. Connect Native federate (Valhalla)
	ambValhalla := lrc.NewRTIambassador()
	fedAmb := &lrc.NullFederateAmbassador{}
	if err := ambValhalla.Connect(fedAmb, crcAddr); err != nil {
		t.Fatalf("Valhalla Connect failed: %v", err)
	}
	defer ambValhalla.Disconnect()

	valHandle, err := ambValhalla.JoinFederationExecution("ValhallaMember", fedName)
	if err != nil || valHandle <= 0 {
		t.Fatalf("Valhalla Join failed: %v", err)
	}

	// 3. Valhalla publishes and registers an object
	if err := ambValhalla.PublishObjectClassAttributes(int32(common.HLAobjectRootHandle), []int32{int32(common.HLAprivilegeToDeleteObjectHandle)}); err != nil {
		t.Fatalf("Valhalla PublishObjectClassAttributes failed: %v", err)
	}
	objHandle, err := ambValhalla.RegisterObjectInstance(int32(common.HLAobjectRootHandle), "TargetAsset_01")
	if err != nil {
		t.Fatalf("Valhalla RegisterObjectInstance failed: %v", err)
	}

	// Drain any discovery packets Thor received
	time.Sleep(50 * time.Millisecond)
	for {
		_, _, err := clientThor.ReadPacketTimeout(50 * time.Millisecond)
		if err != nil {
			break
		}
	}

	// 4. Valhalla deletes the object
	deleteTag := []byte("DELETION_TAG_VALHALLA")
	if err := ambValhalla.DeleteObjectInstance(objHandle, deleteTag); err != nil {
		t.Fatalf("Valhalla DeleteObjectInstance failed: %v", err)
	}

	// 5. Thor reads the notification packet
	hdr, payload, err := clientThor.ReadPacketTimeout(2 * time.Second)
	if err != nil {
		t.Fatalf("Thor did not receive delete notification: %v", err)
	}

	t.Logf("Thor received notification: MessageType=%d (%s), PayloadLen=%d",
		hdr.MessageType, hdr.MessageType.String(), len(payload))

	// CRITICAL ASSERTION: MessageType must be Portico's MsgDeleteObject (64), NOT Native code 47!
	if hdr.MessageType != wire.MsgDeleteObject {
		t.Fatalf("PROTOCOL CORRUPTION: Thor received MessageType %d (%s), expected wire.MsgDeleteObject (64)!",
			hdr.MessageType, hdr.MessageType.String())
	}

	delObj, err := wire.DeserializePorticoControlMessage(payload)
	if err != nil {
		t.Fatalf("Failed to deserialize Portico DeleteObject message: %v", err)
	}
	if oh, ok := delObj.Get("objectHandle").(int32); !ok || oh != objHandle {
		t.Fatalf("Expected objectHandle %d in Portico payload, got %v", objHandle, delObj.Get("objectHandle"))
	}
}

// TestCrossProtocol_NativeToPortico_ReflectAttributes verifies that when a Native federate
// updates attributes, a connected Portico federate receives wire.MsgReflectAttributes (254),
// NOT alien code 43 (which in Portico is MsgAchieveSyncPoint).
func TestCrossProtocol_NativeToPortico_ReflectAttributes(t *testing.T) {
	server := crc.NewServer("127.0.0.1:0")
	if err := server.Start(); err != nil {
		t.Fatalf("failed to start CRC: %v", err)
	}
	defer server.Stop()

	crcAddr := server.Addr().String()
	fedName := "CrossProtocolReflectFed"

	clientThor := transport.NewTCPClient(crcAddr)
	if err := clientThor.Connect(3 * time.Second); err != nil {
		t.Fatalf("failed to connect Thor: %v", err)
	}
	defer clientThor.Close()

	createObj := wire.NewCreateFederationObject(fedName)
	createPayload, _ := wire.SerializePorticoControlMessage(createObj)
	_ = clientThor.SendPacket(wire.NewControlRequest(wire.MsgCreateFederation, 1, 0, 0, uint32(len(createPayload))), createPayload)
	_, _, _ = clientThor.ReadPacketTimeout(2 * time.Second)

	joinObj := wire.NewJoinFederationObject(fedName, "ThorMember", "ThorType")
	joinPayload, _ := wire.SerializePorticoControlMessage(joinObj)
	_ = clientThor.SendPacket(wire.NewControlRequest(wire.MsgJoinFederation, 2, 0, 0, uint32(len(joinPayload))), joinPayload)
	_, welcomePayload, _ := clientThor.ReadPacketTimeout(2 * time.Second)
	wpInfo, _ := wire.ParseWelcomePack(welcomePayload)

	subObj := metamodel.NewSubscribeObjectClass(int32(common.HLAobjectRootHandle), []any{int32(common.HLAprivilegeToDeleteObjectHandle)})
	subPayload, _ := wire.SerializePorticoControlMessage(subObj)
	_ = clientThor.SendPacket(wire.NewControlRequest(wire.MsgSubscribeObjectClass, 3, uint16(wpInfo.FederateHandle), 0, uint32(len(subPayload))), subPayload)
	_, _, _ = clientThor.ReadPacketTimeout(2 * time.Second)

	ambValhalla := lrc.NewRTIambassador()
	fedAmb := &lrc.NullFederateAmbassador{}
	_ = ambValhalla.Connect(fedAmb, crcAddr)
	defer ambValhalla.Disconnect()
	_, _ = ambValhalla.JoinFederationExecution("ValhallaMember", fedName)

	_ = ambValhalla.PublishObjectClassAttributes(int32(common.HLAobjectRootHandle), []int32{int32(common.HLAprivilegeToDeleteObjectHandle)})
	objHandle, _ := ambValhalla.RegisterObjectInstance(int32(common.HLAobjectRootHandle), "RadarAsset")

	time.Sleep(50 * time.Millisecond)
	for {
		_, _, err := clientThor.ReadPacketTimeout(50 * time.Millisecond)
		if err != nil {
			break
		}
	}

	updateData := []byte("TargetLocation:Lat45_Long12")
	err := ambValhalla.UpdateAttributeValues(objHandle, map[int32][]byte{
		int32(common.HLAprivilegeToDeleteObjectHandle): updateData,
	}, []byte("UAV_TAG"))
	if err != nil {
		t.Fatalf("UpdateAttributeValues failed: %v", err)
	}

	hdr, payload, err := clientThor.ReadPacketTimeout(2 * time.Second)
	if err != nil {
		t.Fatalf("Thor did not receive reflection notification: %v", err)
	}

	t.Logf("Thor received reflection: MessageType=%d (%s), PayloadLen=%d",
		hdr.MessageType, hdr.MessageType.String(), len(payload))

	// CRITICAL ASSERTION: MessageType must be Portico's MsgReflectAttributes (254), NOT Native code 43!
	if hdr.MessageType != wire.MsgReflectAttributes {
		t.Fatalf("PROTOCOL CORRUPTION: Thor received MessageType %d (%s), expected wire.MsgReflectAttributes (254)!",
			hdr.MessageType, hdr.MessageType.String())
	}

	ravObj, err := wire.DeserializePorticoControlMessage(payload)
	if err != nil {
		t.Fatalf("Failed to deserialize Portico ReflectAttributes payload: %v", err)
	}
	if oid, ok := ravObj.Get("objectId").(int32); !ok || oid != objHandle {
		t.Fatalf("Expected objectId %d in Portico ReflectAttributes, got %v", objHandle, ravObj.Get("objectId"))
	}
	if !bytes.Contains(payload, updateData) {
		t.Fatalf("Expected Portico ReflectAttributes payload to contain %q", string(updateData))
	}
}

// TestCrossProtocol_PorticoToWeb_ReflectAttributes verifies that when a Portico federate
// updates attributes, a connected Web Profile federate receives valid JSON with callback
// 'reflectAttributeValues', NOT raw Java binary serialization bytes.
func TestCrossProtocol_PorticoToWeb_ReflectAttributes(t *testing.T) {
	server := crc.NewServer("127.0.0.1:0")
	server.SetWebAddr("127.0.0.1:0")
	if err := server.Start(); err != nil {
		t.Fatalf("failed to start CRC: %v", err)
	}
	defer server.Stop()

	crcAddr := server.Addr().String()
	fedName := "CrossProtocolWebFed"

	// 1. Connect Portico federate
	clientThor := transport.NewTCPClient(crcAddr)
	if err := clientThor.Connect(3 * time.Second); err != nil {
		t.Fatalf("failed to connect Thor: %v", err)
	}
	defer clientThor.Close()

	createObj := wire.NewCreateFederationObject(fedName)
	createPayload, _ := wire.SerializePorticoControlMessage(createObj)
	_ = clientThor.SendPacket(wire.NewControlRequest(wire.MsgCreateFederation, 1, 0, 0, uint32(len(createPayload))), createPayload)
	_, _, _ = clientThor.ReadPacketTimeout(2 * time.Second)

	joinObj := wire.NewJoinFederationObject(fedName, "ThorMember", "ThorType")
	joinPayload, _ := wire.SerializePorticoControlMessage(joinObj)
	_ = clientThor.SendPacket(wire.NewControlRequest(wire.MsgJoinFederation, 2, 0, 0, uint32(len(joinPayload))), joinPayload)
	_, welcomePayload, _ := clientThor.ReadPacketTimeout(2 * time.Second)
	wpInfo, _ := wire.ParseWelcomePack(welcomePayload)

	pubObj := metamodel.NewPublishObjectClass(int32(common.HLAobjectRootHandle), []any{metamodel.NewJavaInteger(int32(common.HLAprivilegeToDeleteObjectHandle))})
	pubPayload, _ := wire.SerializePorticoControlMessage(pubObj)
	_ = clientThor.SendPacket(wire.NewControlRequest(wire.MsgPublishObjectClass, 3, uint16(wpInfo.FederateHandle), 0, uint32(len(pubPayload))), pubPayload)
	_, _, _ = clientThor.ReadPacketTimeout(1 * time.Second)

	regPayload, _ := wire.NewRegisterObjectMsg(int32(common.HLAobjectRootHandle), "ThorShip")
	_ = clientThor.SendPacket(wire.NewControlRequest(wire.MsgRegisterObject, 4, uint16(wpInfo.FederateHandle), 0, uint32(len(regPayload))), regPayload)
	_, regResp, _ := clientThor.ReadPacketTimeout(2 * time.Second)
	regObj, _ := wire.DeserializePorticoControlMessage(regResp)
	var objHandle int32 = 1
	if res, ok := regObj.Get("result").(*metamodel.JavaObject); ok {
		if v, ok := res.Get("value").(int32); ok {
			objHandle = v
		}
	}

	// 2. Connect Web federate via server's built-in WebGateway
	webAddr := server.WebAddr().String()
	wsConn, err := net.Dial("tcp", webAddr)
	if err != nil {
		t.Fatalf("Failed to connect to Web gateway: %v", err)
	}
	defer wsConn.Close()

	handshake := "GET /ws HTTP/1.1\r\n" +
		"Host: " + webAddr + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	_, _ = wsConn.Write([]byte(handshake))

	wsReader := bufio.NewReader(wsConn)
	statusLine, err := wsReader.ReadString('\n')
	if err != nil || !strings.Contains(statusLine, "101") {
		t.Fatalf("WebSocket handshake failed: %s", statusLine)
	}
	for {
		line, _ := wsReader.ReadString('\n')
		if line == "\r\n" {
			break
		}
	}

	// Helper to send JSON request
	sendWSJSON := func(action string, params map[string]interface{}) {
		req := map[string]interface{}{
			"id":     1,
			"action": action,
			"params": params,
		}
		data, _ := json.Marshal(req)
		_ = writeMaskedWebSocketFrame(wsConn, data)
		_, _, _ = readWSFrame(wsReader) // read ack response
	}

	sendWSJSON("joinFederationExecution", map[string]interface{}{
		"federateName":   "WebWatcher",
		"federationName": fedName,
	})

	sendWSJSON("subscribeObjectClassAttributes", map[string]interface{}{
		"classHandle": int32(common.HLAobjectRootHandle),
		"attributes":  []int32{int32(common.HLAprivilegeToDeleteObjectHandle)},
	})

	time.Sleep(50 * time.Millisecond)

	// 3. Thor updates attributes
	uavPayload, _ := wire.NewUpdateAttributesMsg(objHandle, map[int32][]byte{
		int32(common.HLAprivilegeToDeleteObjectHandle): []byte("Speed:120_Heading:090"),
	}, []byte("THOR_TAG"))
	_ = clientThor.SendPacket(wire.NewControlRequest(wire.MsgUpdateAttributes, 5, uint16(wpInfo.FederateHandle), 0, uint32(len(uavPayload))), uavPayload)
	_, _, _ = clientThor.ReadPacketTimeout(1 * time.Second)

	// 4. Web reads notification frames: catch-up discoverObjectInstance followed by reflectAttributeValues
	_ = wsConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var gotReflect bool
	for i := 0; i < 2 && !gotReflect; i++ {
		opcode, framePayload, err := readWSFrame(wsReader)
		if err != nil {
			t.Fatalf("Web client did not receive reflection notification: %v", err)
		}

		if opcode != 1 {
			t.Fatalf("PROTOCOL CORRUPTION: Expected Text frame opcode 1 (JSON), got %d!", opcode)
		}

		var notif map[string]interface{}
		if err := json.Unmarshal(framePayload, &notif); err != nil {
			t.Fatalf("PROTOCOL CORRUPTION: Web client received invalid JSON: %v, raw: %s", err, string(framePayload))
		}

		cb, _ := notif["callback"].(string)
		if cb == "discoverObjectInstance" {
			continue
		}
		if cb != "reflectAttributeValues" {
			t.Fatalf("Expected callback 'reflectAttributeValues', got %q", cb)
		}
		params, _ := notif["params"].(map[string]interface{})
		attrs, _ := params["attributes"].(map[string]interface{})
		if attrs["1"] != "Speed:120_Heading:090" {
			t.Fatalf("Expected reflected attribute value 'Speed:120_Heading:090', got %v", attrs["1"])
		}
		gotReflect = true
	}
	if !gotReflect {
		t.Fatalf("Web client never received reflectAttributeValues notification")
	}
}

// TestCrossProtocol_NativeToPortico_DiscoverObject verifies that when a Native federate
// registers an object, a connected Portico federate receives wire.MsgDiscoverObject (61)
// with valid Java Object Serialization payload, NOT Native code 41.
func TestCrossProtocol_NativeToPortico_DiscoverObject(t *testing.T) {
	server := crc.NewServer("127.0.0.1:0")
	if err := server.Start(); err != nil {
		t.Fatalf("failed to start CRC: %v", err)
	}
	defer server.Stop()

	crcAddr := server.Addr().String()
	fedName := "CrossProtocolDiscFed"

	clientThor := transport.NewTCPClient(crcAddr)
	if err := clientThor.Connect(3 * time.Second); err != nil {
		t.Fatalf("failed to connect Thor: %v", err)
	}
	defer clientThor.Close()

	createObj := wire.NewCreateFederationObject(fedName)
	createPayload, _ := wire.SerializePorticoControlMessage(createObj)
	_ = clientThor.SendPacket(wire.NewControlRequest(wire.MsgCreateFederation, 1, 0, 0, uint32(len(createPayload))), createPayload)
	_, _, _ = clientThor.ReadPacketTimeout(2 * time.Second)

	joinObj := wire.NewJoinFederationObject(fedName, "ThorMember", "ThorType")
	joinPayload, _ := wire.SerializePorticoControlMessage(joinObj)
	_ = clientThor.SendPacket(wire.NewControlRequest(wire.MsgJoinFederation, 2, 0, 0, uint32(len(joinPayload))), joinPayload)
	_, welcomePayload, _ := clientThor.ReadPacketTimeout(2 * time.Second)
	wpInfo, _ := wire.ParseWelcomePack(welcomePayload)

	subObj := metamodel.NewSubscribeObjectClass(int32(common.HLAobjectRootHandle), []any{int32(common.HLAprivilegeToDeleteObjectHandle)})
	subPayload, _ := wire.SerializePorticoControlMessage(subObj)
	_ = clientThor.SendPacket(wire.NewControlRequest(wire.MsgSubscribeObjectClass, 3, uint16(wpInfo.FederateHandle), 0, uint32(len(subPayload))), subPayload)
	_, _, _ = clientThor.ReadPacketTimeout(2 * time.Second)

	ambValhalla := lrc.NewRTIambassador()
	fedAmb := &lrc.NullFederateAmbassador{}
	_ = ambValhalla.Connect(fedAmb, crcAddr)
	defer ambValhalla.Disconnect()
	_, _ = ambValhalla.JoinFederationExecution("ValhallaMember", fedName)

	_ = ambValhalla.PublishObjectClassAttributes(int32(common.HLAobjectRootHandle), []int32{int32(common.HLAprivilegeToDeleteObjectHandle)})

	// Clear any pending packets before object registration
	for {
		_, _, err := clientThor.ReadPacketTimeout(50 * time.Millisecond)
		if err != nil {
			break
		}
	}

	objHandle, err := ambValhalla.RegisterObjectInstance(int32(common.HLAobjectRootHandle), "ThorSatellite")
	if err != nil {
		t.Fatalf("RegisterObjectInstance failed: %v", err)
	}

	hdr, payload, err := clientThor.ReadPacketTimeout(2 * time.Second)
	if err != nil {
		t.Fatalf("Thor did not receive discover notification: %v", err)
	}

	t.Logf("Thor received discover: MessageType=%d (%s), PayloadLen=%d",
		hdr.MessageType, hdr.MessageType.String(), len(payload))

	if hdr.MessageType != wire.MsgDiscoverObject {
		t.Fatalf("PROTOCOL CORRUPTION: Thor received MessageType %d (%s), expected wire.MsgDiscoverObject (61)!",
			hdr.MessageType, hdr.MessageType.String())
	}

	discObj, err := wire.DeserializePorticoControlMessage(payload)
	if err != nil {
		t.Fatalf("Failed to deserialize Portico DiscoverObject message: %v", err)
	}
	if oh, ok := discObj.Get("objectHandle").(int32); !ok || oh != objHandle {
		t.Fatalf("Expected objectHandle %d, got %v", objHandle, discObj.Get("objectHandle"))
	}
	if name, ok := discObj.Get("objectName").(string); !ok || name != "ThorSatellite" {
		t.Fatalf("Expected objectName 'ThorSatellite', got %v", discObj.Get("objectName"))
	}
}

// TestCrossProtocol_NativeToPortico_SyncPoint verifies that when a Native federate announces
// a sync point, a Portico federate receives wire.MsgAnnounceSyncPoint (42). When Portico achieves it,
// both receive wire.MsgFederationSynchronized (44).
func TestCrossProtocol_NativeToPortico_SyncPoint(t *testing.T) {
	server := crc.NewServer("127.0.0.1:0")
	if err := server.Start(); err != nil {
		t.Fatalf("failed to start CRC: %v", err)
	}
	defer server.Stop()

	crcAddr := server.Addr().String()
	fedName := "CrossProtocolSyncFed"

	clientThor := transport.NewTCPClient(crcAddr)
	if err := clientThor.Connect(3 * time.Second); err != nil {
		t.Fatalf("failed to connect Thor: %v", err)
	}
	defer clientThor.Close()

	createObj := wire.NewCreateFederationObject(fedName)
	createPayload, _ := wire.SerializePorticoControlMessage(createObj)
	_ = clientThor.SendPacket(wire.NewControlRequest(wire.MsgCreateFederation, 1, 0, 0, uint32(len(createPayload))), createPayload)
	_, _, _ = clientThor.ReadPacketTimeout(2 * time.Second)

	joinObj := wire.NewJoinFederationObject(fedName, "ThorMember", "ThorType")
	joinPayload, _ := wire.SerializePorticoControlMessage(joinObj)
	_ = clientThor.SendPacket(wire.NewControlRequest(wire.MsgJoinFederation, 2, 0, 0, uint32(len(joinPayload))), joinPayload)
	_, welcomePayload, _ := clientThor.ReadPacketTimeout(2 * time.Second)
	wpInfo, _ := wire.ParseWelcomePack(welcomePayload)

	ambValhalla := lrc.NewRTIambassador()
	fedAmb := &lrc.NullFederateAmbassador{}
	_ = ambValhalla.Connect(fedAmb, crcAddr)
	defer ambValhalla.Disconnect()
	_, _ = ambValhalla.JoinFederationExecution("ValhallaMember", fedName)

	syncPointLabel := "MissionBriefingAlpha"
	if err := ambValhalla.RegisterFederationSynchronizationPoint(syncPointLabel, []byte("BRIEFING_TAG")); err != nil {
		t.Fatalf("RegisterFederationSynchronizationPoint failed: %v", err)
	}

	hdr, payload, err := clientThor.ReadPacketTimeout(2 * time.Second)
	if err != nil {
		t.Fatalf("Thor did not receive AnnounceSyncPoint notification: %v", err)
	}

	t.Logf("Thor received announce sync point: MessageType=%d (%s), PayloadLen=%d",
		hdr.MessageType, hdr.MessageType.String(), len(payload))

	if hdr.MessageType != wire.MsgAnnounceSyncPoint {
		t.Fatalf("PROTOCOL CORRUPTION: Thor received MessageType %d (%s), expected wire.MsgAnnounceSyncPoint (42)!",
			hdr.MessageType, hdr.MessageType.String())
	}

	annObj, err := wire.DeserializePorticoControlMessage(payload)
	if err != nil {
		t.Fatalf("Failed to deserialize AnnounceSyncPoint: %v", err)
	}
	if label, ok := annObj.Get("label").(string); !ok || label != syncPointLabel {
		t.Fatalf("Expected sync point label %q, got %q", syncPointLabel, label)
	}

	// Valhalla achieves sync point
	if err := ambValhalla.SynchronizationPointAchieved(syncPointLabel); err != nil {
		t.Fatalf("Valhalla SynchronizationPointAchieved failed: %v", err)
	}

	// Thor achieves sync point
	achieveObj := metamodel.NewSyncPointAchieved(syncPointLabel, int32(wpInfo.FederateHandle))
	achievePayload, _ := wire.SerializePorticoControlMessage(achieveObj)
	_ = clientThor.SendPacket(wire.NewControlRequest(wire.MsgAchieveSyncPoint, 5, uint16(wpInfo.FederateHandle), 0, uint32(len(achievePayload))), achievePayload)
	_, _, _ = clientThor.ReadPacketTimeout(2 * time.Second) // ack

	// Thor should receive FederationSynchronized (44)
	hdrSync, payloadSync, err := clientThor.ReadPacketTimeout(2 * time.Second)
	if err != nil {
		t.Fatalf("Thor did not receive FederationSynchronized notification: %v", err)
	}

	if hdrSync.MessageType != wire.MsgFederationSynchronized {
		t.Fatalf("PROTOCOL CORRUPTION: Thor received MessageType %d (%s), expected wire.MsgFederationSynchronized (44)!",
			hdrSync.MessageType, hdrSync.MessageType.String())
	}

	syncObj, err := wire.DeserializePorticoControlMessage(payloadSync)
	if err != nil {
		t.Fatalf("Failed to deserialize FederationSynchronized: %v", err)
	}
	if label, ok := syncObj.Get("label").(string); !ok || label != syncPointLabel {
		t.Fatalf("Expected FederationSynchronized label %q, got %q", syncPointLabel, label)
	}
}
