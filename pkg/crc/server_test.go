package crc

import (
	"fmt"
	"io"
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

func TestServer_DedicatedPorticoPort_ZeroLatency(t *testing.T) {
	srv := NewServer("127.0.0.1:0")
	// Set 100ms peek timeout on multiplexed port to make the contrast obvious
	srv.SetPeekTimeout(100 * time.Millisecond)
	srv.SetPorticoAddr("127.0.0.1:0")

	if err := srv.Start(); err != nil {
		t.Fatalf("Server.Start failed: %v", err)
	}
	defer srv.Stop()

	porticoAddr := srv.PorticoAddr()
	if porticoAddr == nil {
		t.Fatal("Server.PorticoAddr() is nil")
	}

	// 1. Conectar na porta dedicada do Portico: Latencia de handshake deve ser quase zero (< 20ms)
	start := time.Now()
	connDedicated, err := net.Dial("tcp", porticoAddr.String())
	if err != nil {
		t.Fatalf("Failed to connect to dedicated Portico port: %v", err)
	}
	defer connDedicated.Close()

	var welcomeBuf [8]byte
	if _, err := connDedicated.Read(welcomeBuf[:]); err != nil {
		t.Fatalf("Failed to read Portico greeting on dedicated port: %v", err)
	}
	elapsedDedicated := time.Since(start)

	if elapsedDedicated >= 50*time.Millisecond {
		t.Errorf("Dedicated Portico port suffered peeking delay: %v (expected < 20ms)", elapsedDedicated)
	}

	// 2. Conectar na porta unificada multiplexada (peekTimeout = 100ms): Deve esperar os 100ms
	startMux := time.Now()
	connMux, err := net.Dial("tcp", srv.Addr().String())
	if err != nil {
		t.Fatalf("Failed to connect to multiplexed port: %v", err)
	}
	defer connMux.Close()

	if _, err := connMux.Read(welcomeBuf[:]); err != nil {
		t.Fatalf("Failed to read Portico greeting on multiplexed port: %v", err)
	}
	elapsedMux := time.Since(startMux)

	if elapsedMux < 90*time.Millisecond {
		t.Errorf("Multiplexed port did not respect peek timeout: %v (expected >= 90ms)", elapsedMux)
	}
}

func TestServer_DedicatedNativePort_ZeroLatency(t *testing.T) {
	srv := NewServer("127.0.0.1:0")
	srv.SetNativeAddr("127.0.0.1:0")

	if err := srv.Start(); err != nil {
		t.Fatalf("Server.Start failed: %v", err)
	}
	defer srv.Stop()

	nativeAddr := srv.NativeAddr()
	if nativeAddr == nil {
		t.Fatal("Server.NativeAddr() is nil")
	}

	// Conectar diretamente na porta dedicada Nativa
	conn, err := net.Dial("tcp", nativeAddr.String())
	if err != nil {
		t.Fatalf("Failed to connect to dedicated Native port: %v", err)
	}
	defer conn.Close()

	// Enviar preambulo VAL4
	if _, err := conn.Write([]byte("VAL4")); err != nil {
		t.Fatalf("Failed to write VAL4 preamble: %v", err)
	}

	session := NewNativeSession(conn)

	// 1. Criar federação
	createReq := common.FederationExecutionPayload{
		FederationName: "DedicatedFed",
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
		t.Fatalf("Failed to send NativeMsgCreateFederation: %v", err)
	}
	respHdr, err := ReadNativeHeader(conn)
	if err != nil {
		t.Fatalf("Failed to read Create response: %v", err)
	}
	if respHdr.MsgType != NativeMsgSuccessOK {
		t.Fatalf("Expected NativeMsgSuccessOK (%d), got %d", NativeMsgSuccessOK, respHdr.MsgType)
	}

	// 2. Join na federação
	joinReq := common.JoinFederationPayload{
		FederateName:   "FastNative",
		FederateType:   "native-fast",
		FederationName: "DedicatedFed",
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
	start := time.Now()
	if err := session.SendMessage(joinHdr, joinPayload); err != nil {
		t.Fatalf("Failed to send NativeMsgJoin: %v", err)
	}

	respHdr, err = ReadNativeHeader(conn)
	if err != nil {
		t.Fatalf("Failed to read Native response on dedicated port: %v", err)
	}
	elapsed := time.Since(start)
	if respHdr.MsgType != NativeMsgJoinSuccess {
		t.Errorf("Expected NativeMsgJoinSuccess (%d), got %d", NativeMsgJoinSuccess, respHdr.MsgType)
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("Dedicated native port Join RTT too high: %v (expected < 200ms)", elapsed)
	}
}

func TestServer_ConfigurablePeekTimeout(t *testing.T) {
	srv := NewServer("127.0.0.1:0")
	srv.SetPeekTimeout(10 * time.Millisecond)
	if srv.PeekTimeout() != 10*time.Millisecond {
		t.Errorf("Expected peekTimeout 10ms, got %v", srv.PeekTimeout())
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("Server.Start failed: %v", err)
	}
	defer srv.Stop()

	// Portico na porta unificada com peekTimeout reduzido (10ms)
	start := time.Now()
	conn, err := net.Dial("tcp", srv.Addr().String())
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	var welcomeBuf [8]byte
	if _, err := conn.Read(welcomeBuf[:]); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	elapsed := time.Since(start)

	// Com timeout de 10ms, deve demorar entre 10ms e 40ms, bem abaixo dos 50ms forçados anteriores
	if elapsed > 45*time.Millisecond {
		t.Errorf("Elapsed time too high for 10ms peek timeout: %v", elapsed)
	}
}

func TestServer_NativeRejectPQCFlag(t *testing.T) {
	srv := NewServer("127.0.0.1:0")
	if err := srv.Start(); err != nil {
		t.Fatalf("Server.Start failed: %v", err)
	}
	defer srv.Stop()

	conn, err := net.Dial("tcp", srv.Addr().String())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("VAL4")); err != nil {
		t.Fatalf("Failed to write VAL4: %v", err)
	}

	session := NewNativeSession(conn)
	hdr := NativeHeader{
		Magic:      NativeHeaderMagic,
		Version:    NativeProtocolVer,
		Flags:      common.FlagNativePQC,
		MsgType:    NativeMsgCreateFederation,
		ReqID:      42,
		PayloadLen: 0,
	}

	if err := session.SendMessage(hdr, nil); err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	respHdr, err := ReadNativeHeader(conn)
	if err != nil {
		t.Fatalf("Failed to read header: %v", err)
	}

	if respHdr.MsgType != NativeMsgError {
		t.Fatalf("Expected NativeMsgError (%d), got %d", NativeMsgError, respHdr.MsgType)
	}

	errPayload := make([]byte, respHdr.PayloadLen)
	if _, err := io.ReadFull(conn, errPayload); err != nil {
		t.Fatalf("Failed to read error payload: %v", err)
	}

	var errResp common.ErrorResponsePayload
	if err := errResp.UnmarshalBinary(errPayload); err != nil {
		t.Fatalf("Failed to unmarshal error response: %v", err)
	}

	if errResp.ErrorCode != 400 || !strings.Contains(errResp.Message, "post-quantum cryptography") {
		t.Fatalf("Expected 400 with PQC rejection, got code %d, message %q", errResp.ErrorCode, errResp.Message)
	}
}
