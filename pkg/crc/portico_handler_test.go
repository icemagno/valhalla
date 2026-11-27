package crc

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/icemagno/valhalla/pkg/crc/portico/wire"
)

func TestPorticoHandler_FullLifecycle(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("falha ao criar listener: %v", err)
	}
	defer ln.Close()

	fedManager := NewFederationManager()

	// Iniciar handler do servidor em goroutine aceitando conexão TCP real
	serverErrChan := make(chan error, 1)
	go func() {
		serverConn, err := ln.Accept()
		if err != nil {
			serverErrChan <- err
			return
		}
		defer serverConn.Close()
		err = HandlePorticoConnection(serverConn, serverConn, fedManager)
		serverErrChan <- err
	}()

	clientConn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("falha ao conectar cliente: %v", err)
	}
	defer clientConn.Close()

	// Simular cliente Portico (ex: Thor)
	clientSession := NewPorticoSession(clientConn)

	readControlResponse := func() (wire.Header, []byte, error) {
		for {
			hdr, payload, err := clientSession.ReadPacket(clientConn)
			if err != nil {
				return hdr, nil, err
			}
			if hdr.CallType == wire.CallTypeControlResponseOK || hdr.CallType == wire.CallTypeControlResponseErr {
				return hdr, payload, nil
			}
			// Notificações assíncronas (ex: AnnounceSyncPoint, FederationSynchronized) são consumidas
		}
	}

	// 1. Handshake do cliente: ler 0xBEEF, enviar 0xFEED
	var welcomeMagic int32
	var welcomeLen int32
	if err := clientConn.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
		t.Fatalf("falha set deadline: %v", err)
	}

	buf := make([]byte, 8)
	if _, err := io.ReadFull(clientConn, buf); err != nil {
		t.Fatalf("falha lendo magic de boas-vindas: %v", err)
	}
	welcomeMagic = int32(wire.HeaderLength) // apenas variavel
	_ = welcomeMagic
	_ = welcomeLen

	msgBuf := make([]byte, len(PorticoWelcomeMsg))
	if _, err := io.ReadFull(clientConn, msgBuf); err != nil {
		t.Fatalf("falha lendo texto de boas-vindas: %v", err)
	}

	// Ler 0xFEED do servidor
	var srvFeed [4]byte
	if _, err := io.ReadFull(clientConn, srvFeed[:]); err != nil {
		t.Fatalf("falha lendo 0xFEED do servidor: %v", err)
	}

	// Responder com 0xFEED (65261)
	var ready [4]byte
	ready[0], ready[1], ready[2], ready[3] = 0x00, 0x00, 0xFE, 0xED
	if _, err := clientConn.Write(ready[:]); err != nil {
		t.Fatalf("falha enviando 0xFEED: %v", err)
	}

	// 2. Enviar CreateFederation
	createObj := wire.NewCreateFederationObject("ExampleFederation")
	createPayload, err := wire.SerializePorticoControlMessage(createObj)
	if err != nil {
		t.Fatalf("falha serializando CreateFederation: %v", err)
	}
	createHdr := wire.NewControlRequest(wire.MsgCreateFederation, 1, 0, 0, uint32(len(createPayload)))
	if err := clientSession.SendPacket(createHdr, createPayload); err != nil {
		t.Fatalf("falha enviando CreateFederation: %v", err)
	}
	cfHdr, cfPayload, err := readControlResponse()
	if err != nil {
		t.Fatalf("falha lendo resposta CreateFederation: %v", err)
	}
	if cfHdr.CallType != wire.CallTypeControlResponseOK || len(cfPayload) == 0 {
		t.Fatalf("esperado ControlResponseOK para CreateFederation, obtido %s", cfHdr.CallType)
	}

	// 3. Enviar JoinFederation
	joinObj := wire.NewJoinFederationObject("ExampleFederation", "Thor", "ieee1516e")
	joinPayload, err := wire.SerializePorticoControlMessage(joinObj)
	if err != nil {
		t.Fatalf("falha serializando Join: %v", err)
	}

	joinHdr := wire.NewControlRequest(wire.MsgJoinFederation, 2, 0, 0, uint32(len(joinPayload)))
	if err := clientSession.SendPacket(joinHdr, joinPayload); err != nil {
		t.Fatalf("falha enviando Join: %v", err)
	}

	// Ler resposta do Join (WelcomePack)
	resHdr, resPayload, err := readControlResponse()
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

	// 4. Registrar SyncPoint antes de conquistá-lo
	regSyncObj := wire.NewRegisterSyncPointObject("ReadyToRun", 1)
	regSyncPayload, err := wire.SerializePorticoControlMessage(regSyncObj)
	if err != nil {
		t.Fatalf("falha serializando RegisterSyncPoint: %v", err)
	}
	regSyncHdr := wire.NewControlRequest(wire.MsgRegisterSyncPoint, 3, 1, 0, uint32(len(regSyncPayload)))
	if err := clientSession.SendPacket(regSyncHdr, regSyncPayload); err != nil {
		t.Fatalf("falha enviando RegisterSyncPoint: %v", err)
	}
	regSyncResHdr, _, err := readControlResponse()
	if err != nil {
		t.Fatalf("falha lendo resposta RegisterSyncPoint: %v", err)
	}
	if regSyncResHdr.CallType != wire.CallTypeControlResponseOK {
		t.Fatalf("esperado ControlResponseOK para RegisterSyncPoint, obtido %s", regSyncResHdr.CallType)
	}

	// 5. Enviar SyncPointAchieved
	syncObj := wire.NewSyncPointAchievedObject("ReadyToRun", 1)
	syncPayload, err := wire.SerializePorticoControlMessage(syncObj)
	if err != nil {
		t.Fatalf("falha serializando SyncPointAchieved: %v", err)
	}

	syncHdr := wire.NewControlRequest(wire.MsgAchieveSyncPoint, 4, 1, 0, uint32(len(syncPayload)))
	if err := clientSession.SendPacket(syncHdr, syncPayload); err != nil {
		t.Fatalf("falha enviando SyncPointAchieved: %v", err)
	}

	syncResHdr, _, err := readControlResponse()
	if err != nil {
		t.Fatalf("falha lendo resposta Sync: %v", err)
	}
	if syncResHdr.CallType != wire.CallTypeControlResponseOK {
		t.Errorf("esperado ControlResponseOK no Sync, obtido %s", syncResHdr.CallType)
	}

	// 6. Enviar ResignFederation
	resignObj := wire.NewResignFederationObject("ExampleFederation", "Thor", "ieee1516e", "NO_ACTION")
	resignPayload, err := wire.SerializePorticoControlMessage(resignObj)
	if err != nil {
		t.Fatalf("falha serializando Resign: %v", err)
	}

	resignHdr := wire.NewControlRequest(wire.MsgResignFederation, 5, 1, 0, uint32(len(resignPayload)))
	if err := clientSession.SendPacket(resignHdr, resignPayload); err != nil {
		t.Fatalf("falha enviando Resign: %v", err)
	}

	resignResHdr, _, err := readControlResponse()
	if err != nil {
		t.Fatalf("falha lendo resposta Resign: %v", err)
	}
	if resignResHdr.CallType != wire.CallTypeControlResponseOK {
		t.Errorf("esperado ControlResponseOK no Resign, obtido %s", resignResHdr.CallType)
	}

	// Cliente desconecta fechando o socket
	_ = clientConn.Close()

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
