package crc

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc/portico/wire"
)

func TestCRCServer_DualProtocolMultiplexing(t *testing.T) {
	server := NewServer("127.0.0.1:0")
	if err := server.Start(); err != nil {
		t.Fatalf("falha ao iniciar servidor CRC: %v", err)
	}
	defer server.Stop()

	serverAddr := server.Addr().String()

	// 1. Testar conexão de Cliente Portico (Legado / Server-First)
	porticoConn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		t.Fatalf("falha conectando como Portico: %v", err)
	}
	defer porticoConn.Close()

	// Portico cliente lê 0xBEEF do servidor
	var welcomeBuf [8]byte
	if _, err := porticoConn.Read(welcomeBuf[:]); err != nil {
		t.Fatalf("falha lendo 0xBEEF: %v", err)
	}
	// Lê string
	strBuf := make([]byte, len(PorticoWelcomeMsg))
	if _, err := porticoConn.Read(strBuf); err != nil {
		t.Fatalf("falha lendo mensagem de boas-vindas: %v", err)
	}

	// Lê 0xFEED do servidor
	var srvFeed [4]byte
	if _, err := porticoConn.Read(srvFeed[:]); err != nil {
		t.Fatalf("falha lendo 0xFEED do servidor: %v", err)
	}

	// Responde 0xFEED
	var ready [4]byte
	ready[2], ready[3] = 0xFE, 0xED
	if _, err := porticoConn.Write(ready[:]); err != nil {
		t.Fatalf("falha enviando 0xFEED: %v", err)
	}

	porticoSession := NewPorticoSession(porticoConn)

	// Cria a federação formalmente no Portico
	createObj := wire.NewCreateFederationObject("ExampleFederation")
	createPayload, errCreate := wire.SerializePorticoControlMessage(createObj)
	if errCreate != nil {
		t.Fatalf("falha serializando CreateFederation Portico: %v", errCreate)
	}
	createHdr := wire.NewControlRequest(wire.MsgCreateFederation, 1, 0, 0, uint32(len(createPayload)))
	if err := porticoSession.SendPacket(createHdr, createPayload); err != nil {
		t.Fatalf("falha enviando CreateFederation Portico: %v", err)
	}
	cfHdr, _, err := porticoSession.ReadPacket(porticoConn)
	if err != nil || cfHdr.CallType != wire.CallTypeControlResponseOK {
		t.Fatalf("falha esperando resposta de CreateFederation Portico: %v", err)
	}

	// Envia JoinFederation
	joinObj := wire.NewJoinFederationObject("ExampleFederation", "ThorLegado", "ieee1516e")
	joinPayload, _ := wire.SerializePorticoControlMessage(joinObj)
	joinHdr := wire.NewControlRequest(wire.MsgJoinFederation, 2, 0, 0, uint32(len(joinPayload)))
	if err := porticoSession.SendPacket(joinHdr, joinPayload); err != nil {
		t.Fatalf("falha enviando Join Portico: %v", err)
	}

	// Recebe WelcomePack
	_, wpPayload, err := porticoSession.ReadPacket(porticoConn)
	if err != nil {
		t.Fatalf("falha lendo WelcomePack Portico: %v", err)
	}
	wpInfo, err := wire.ParseWelcomePack(wpPayload)
	if err != nil {
		t.Fatalf("falha parseando WelcomePack: %v", err)
	}
	if wpInfo.FederateHandle != 1 {
		t.Errorf("esperado handle 1 para Thor, obtido %d", wpInfo.FederateHandle)
	}

	// 2. Testar conexão de Cliente Valhalla Nativo (Client-First: envia VAL4)
	nativeConn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		t.Fatalf("falha conectando como Nativo: %v", err)
	}
	defer nativeConn.Close()

	// Envia "VAL4" + NativeMsgJoin
	if _, err := nativeConn.Write([]byte("VAL4")); err != nil {
		t.Fatalf("falha enviando preambulo VAL4: %v", err)
	}

	joinReq := common.JoinFederationPayload{
		FederateName:   "ValhallaNativo",
		FederateType:   "native-hla4",
		FederationName: "ExampleFederation",
	}
	joinPayload, errMarshal := joinReq.MarshalBinary()
	if errMarshal != nil {
		t.Fatalf("falha serializando JoinFederationPayload: %v", errMarshal)
	}

	hdr := NativeHeader{
		Magic:      NativeHeaderMagic,
		Version:    NativeProtocolVer,
		Flags:      0x01,
		PayloadLen: uint32(len(joinPayload)),
		MsgType:    NativeMsgJoin,
		ReqID:      100,
	}
	nativeSession := NewNativeSession(nativeConn)
	if err := nativeSession.SendMessage(hdr, joinPayload); err != nil {
		t.Fatalf("falha enviando mensagem nativa de join: %v", err)
	}

	// Recebe resposta NativeMsgJoinSuccess
	respHdr, err := ReadNativeHeader(nativeConn)
	if err != nil {
		t.Fatalf("falha lendo resposta nativa: %v", err)
	}
	if respHdr.MsgType != NativeMsgJoinSuccess {
		t.Errorf("esperado MsgType NativeMsgJoinSuccess (%d), obtido %d", NativeMsgJoinSuccess, respHdr.MsgType)
	}

	// Federado nativo deve ter recebido handle 2 (pois Thor foi 1 na mesma federação!)
	if respHdr.FederateID != 2 {
		t.Errorf("esperado handle 2 para ValhallaNativo na mesma federacao, obtido %d", respHdr.FederateID)
	}

	time.Sleep(50 * time.Millisecond)
}

func TestServer_WebGateway_EphemeralPortNeverBindsToPort1(t *testing.T) {
	testCases := []struct {
		name        string
		inputAddr   string
		expectedWeb string
	}{
		{
			name:        "IPv4 localhost ephemeral",
			inputAddr:   "127.0.0.1:0",
			expectedWeb: "127.0.0.1:0",
		},
		{
			name:        "All interfaces ephemeral",
			inputAddr:   ":0",
			expectedWeb: ":0",
		},
		{
			name:        "IPv6 localhost ephemeral",
			inputAddr:   "[::1]:0",
			expectedWeb: "[::1]:0",
		},
		{
			name:        "Max port boundary rollover",
			inputAddr:   ":65535",
			expectedWeb: ":0",
		},
		{
			name:        "Standard fixed port increment",
			inputAddr:   ":52295",
			expectedWeb: ":52296",
		},
		{
			name:        "IPv4 fixed port increment",
			inputAddr:   "192.168.1.100:8000",
			expectedWeb: "192.168.1.100:8001",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			srv := NewServer(tc.inputAddr)
			if srv.webAddr == "127.0.0.1:1" || strings.HasSuffix(srv.webAddr, ":1") {
				t.Fatalf("VULNERABILITY: NewServer derived privileged port 1 for WebGateway: %s (input: %s)", srv.webAddr, tc.inputAddr)
			}
			if srv.webAddr != tc.expectedWeb {
				t.Errorf("Expected webAddr %q, got %q (input: %s)", tc.expectedWeb, srv.webAddr, tc.inputAddr)
			}
		})
	}
}

func TestServer_WebGateway_StartWithEphemeralPort_EndToEnd(t *testing.T) {
	srv := NewServer("127.0.0.1:0")
	if err := srv.Start(); err != nil {
		t.Fatalf("Server.Start failed: %v", err)
	}
	defer srv.Stop()

	crcAddr := srv.Addr()
	if crcAddr == nil {
		t.Fatal("Server.Addr() returned nil")
	}
	crcTCP := crcAddr.(*net.TCPAddr)
	if crcTCP.Port <= 1024 {
		t.Errorf("CRC port %d is not in unprivileged ephemeral range (>1024)", crcTCP.Port)
	}

	webAddr := srv.WebAddr()
	if webAddr == nil {
		t.Fatal("Server.WebAddr() returned nil - WebGateway failed to start")
	}
	webTCP := webAddr.(*net.TCPAddr)
	if webTCP.Port <= 1024 {
		t.Errorf("WebGateway port %d is not in unprivileged ephemeral range (>1024)", webTCP.Port)
	}
	if webTCP.Port == 1 {
		t.Fatalf("VULNERABILITY: WebGateway bound to privileged port 1!")
	}
	if webTCP.Port == crcTCP.Port {
		t.Errorf("WebGateway port %d collided with CRC port %d", webTCP.Port, crcTCP.Port)
	}

	// Verify HTTP /api/v1/status responds on the dynamic WebGateway port
	resp, err := http.Get(fmt.Sprintf("http://%s/api/v1/status", webAddr.String()))
	if err != nil {
		t.Fatalf("Failed to reach WebGateway /api/v1/status on %s: %v", webAddr.String(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected HTTP 200 OK from WebGateway, got %d", resp.StatusCode)
	}
}
