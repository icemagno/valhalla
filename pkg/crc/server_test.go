package crc

import (
	"net"
	"testing"
	"time"

	"github.com/icemagno/valhalla/pkg/portico/wire"
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

	// Envia JoinFederation
	porticoSession := NewPorticoSession(porticoConn)
	joinObj := wire.NewJoinFederationObject("ExampleFederation", "ThorLegado", "ieee1516e")
	joinPayload, _ := wire.SerializePorticoControlMessage(joinObj)
	joinHdr := wire.NewControlRequest(wire.MsgJoinFederation, 1, 0, 0, uint32(len(joinPayload)))
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

	nodeName := "ValhallaNativo"
	hdr := NativeHeader{
		Magic:      NativeHeaderMagic,
		Version:    NativeProtocolVer,
		Flags:      0x01,
		PayloadLen: uint32(len(nodeName)),
		MsgType:    NativeMsgJoin,
		ReqID:      100,
	}
	nativeSession := NewNativeSession(nativeConn)
	if err := nativeSession.SendMessage(hdr, []byte(nodeName)); err != nil {
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
