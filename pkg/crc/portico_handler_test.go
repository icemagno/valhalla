package crc

import (
	"net"
	"testing"
	"time"

	"github.com/icemagno/valhalla/pkg/crc/portico/wire"
)

func TestPorticoHandler_FullLifecycle(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	fedManager := NewFederationManager()

	// Iniciar handler do servidor em goroutine
	serverErrChan := make(chan error, 1)
	go func() {
		err := HandlePorticoConnection(serverConn, serverConn, fedManager)
		serverErrChan <- err
	}()

	// Simular cliente Portico (ex: Thor)
	clientSession := NewPorticoSession(clientConn)

	// 1. Handshake do cliente: ler 0xBEEF, enviar 0xFEED
	var welcomeMagic int32
	var welcomeLen int32
	if err := clientConn.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
		t.Fatalf("falha set deadline: %v", err)
	}

	buf := make([]byte, 8)
	if _, err := clientConn.Read(buf); err != nil {
		t.Fatalf("falha lendo magic de boas-vindas: %v", err)
	}
	welcomeMagic = int32(wire.HeaderLength) // apenas variavel
	_ = welcomeMagic
	_ = welcomeLen

	msgBuf := make([]byte, len(PorticoWelcomeMsg))
	if _, err := clientConn.Read(msgBuf); err != nil {
		t.Fatalf("falha lendo texto de boas-vindas: %v", err)
	}

	// Ler 0xFEED do servidor
	var srvFeed [4]byte
	if _, err := clientConn.Read(srvFeed[:]); err != nil {
		t.Fatalf("falha lendo 0xFEED do servidor: %v", err)
	}

	// Responder com 0xFEED (65261)
	var ready [4]byte
	ready[0], ready[1], ready[2], ready[3] = 0x00, 0x00, 0xFE, 0xED
	if _, err := clientConn.Write(ready[:]); err != nil {
		t.Fatalf("falha enviando 0xFEED: %v", err)
	}

	// 2. Enviar JoinFederation
	joinObj := wire.NewJoinFederationObject("ExampleFederation", "Thor", "ieee1516e")
	joinPayload, err := wire.SerializePorticoControlMessage(joinObj)
	if err != nil {
		t.Fatalf("falha serializando Join: %v", err)
	}

	joinHdr := wire.NewControlRequest(wire.MsgJoinFederation, 1, 0, 0, uint32(len(joinPayload)))
	if err := clientSession.SendPacket(joinHdr, joinPayload); err != nil {
		t.Fatalf("falha enviando Join: %v", err)
	}

	// Ler resposta do Join (WelcomePack)
	resHdr, resPayload, err := clientSession.ReadPacket(clientConn)
	if err != nil {
		t.Fatalf("falha lendo resposta do Join: %v", err)
	}

	if resHdr.CallType != wire.CallTypeControlResponseOK {
		t.Fatalf("esperado ControlResponseOK, obtido %s", resHdr.CallType)
	}

	wpInfo, err := wire.ParseWelcomePack(resPayload)
	if err != nil {
		t.Fatalf("falha parseando WelcomePack retornado pelo Valhalla CRC: %v", err)
	}

	if wpInfo.FederateHandle != 1 {
		t.Errorf("FederateHandle atribuido esperado 1, obtido %d", wpInfo.FederateHandle)
	}
	if wpInfo.FederationName != "ExampleFederation" {
		t.Errorf("FederationName esperado ExampleFederation, obtido %s", wpInfo.FederationName)
	}

	// 3. Enviar SyncPointAchieved
	syncObj := wire.NewSyncPointAchievedObject("ReadyToRun", 1)
	syncPayload, err := wire.SerializePorticoControlMessage(syncObj)
	if err != nil {
		t.Fatalf("falha serializando SyncPointAchieved: %v", err)
	}

	syncHdr := wire.NewControlRequest(wire.MsgAchieveSyncPoint, 2, 1, 0, uint32(len(syncPayload)))
	if err := clientSession.SendPacket(syncHdr, syncPayload); err != nil {
		t.Fatalf("falha enviando SyncPointAchieved: %v", err)
	}

	syncResHdr, _, err := clientSession.ReadPacket(clientConn)
	if err != nil {
		t.Fatalf("falha lendo resposta Sync: %v", err)
	}
	if syncResHdr.CallType != wire.CallTypeControlResponseOK {
		t.Errorf("esperado ControlResponseOK no Sync, obtido %s", syncResHdr.CallType)
	}

	// 4. Enviar ResignFederation
	resignObj := wire.NewResignFederationObject("ExampleFederation", "Thor", "ieee1516e", "NO_ACTION")
	resignPayload, err := wire.SerializePorticoControlMessage(resignObj)
	if err != nil {
		t.Fatalf("falha serializando Resign: %v", err)
	}

	resignHdr := wire.NewControlRequest(wire.MsgResignFederation, 3, 1, 0, uint32(len(resignPayload)))
	if err := clientSession.SendPacket(resignHdr, resignPayload); err != nil {
		t.Fatalf("falha enviando Resign: %v", err)
	}

	resignResHdr, _, err := clientSession.ReadPacket(clientConn)
	if err != nil {
		t.Fatalf("falha lendo resposta Resign: %v", err)
	}
	if resignResHdr.CallType != wire.CallTypeControlResponseOK {
		t.Errorf("esperado ControlResponseOK no Resign, obtido %s", resignResHdr.CallType)
	}

	// Aguardar término gracioso do servidor
	select {
	case err := <-serverErrChan:
		if err != nil {
			t.Errorf("servidor encerrou com erro: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout aguardando servidor encerrar")
	}
}
