package integration_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc"
	"github.com/icemagno/valhalla/pkg/lrc"
	"github.com/icemagno/valhalla/pkg/crc/portico/transport"
	"github.com/icemagno/valhalla/pkg/crc/portico/wire"
	"github.com/icemagno/valhalla/pkg/crc/portico/wire/metamodel"
)

type mixedRecorderAmb struct {
	lrc.NullFederateAmbassador
	onReflect func(obj int32, attrs map[int32][]byte)
}

func (m *mixedRecorderAmb) ReflectAttributeValues(obj int32, attrs map[int32][]byte, tag []byte) {
	if m.onReflect != nil {
		m.onReflect(obj, attrs)
	}
}

func TestMixedFederation_ThorAndValhalla(t *testing.T) {
	// 1. Iniciar servidor soberano CRC do Valhalla
	server := crc.NewServer("127.0.0.1:0")
	if err := server.Start(); err != nil {
		t.Fatalf("falha ao iniciar servidor CRC: %v", err)
	}
	defer server.Stop()

	crcAddr := server.Addr().String()

	// 2. Conectar Federado Portico Legado (simulando o Thor em Java)
	clientThor := transport.NewTCPClient(crcAddr)
	if err := clientThor.Connect(3 * time.Second); err != nil {
		t.Fatalf("falha ao conectar cliente Thor legado ao CRC: %v", err)
	}
	defer clientThor.Close()

	if !clientThor.IsConnected() {
		t.Fatal("esperava que o cliente Thor estivesse conectado")
	}

	// 2.1 Thor envia CreateFederationExecution
	createObj := wire.NewCreateFederationObject("ExampleFederation")
	createPayload, err := wire.SerializePorticoControlMessage(createObj)
	if err != nil {
		t.Fatalf("falha ao serializar CreateFederation do Thor: %v", err)
	}
	createHdr := wire.NewControlRequest(wire.MsgCreateFederation, 1, 0, 0, uint32(len(createPayload)))
	if err := clientThor.SendPacket(createHdr, createPayload); err != nil {
		t.Fatalf("falha ao enviar CreateFederation do Thor: %v", err)
	}
	_, cfRespPayload, err := clientThor.ReadPacketTimeout(3 * time.Second)
	if err != nil {
		t.Fatalf("falha ao receber resposta de CreateFederation do Thor: %v", err)
	}
	if len(cfRespPayload) == 0 {
		t.Fatal("resposta de CreateFederation vazia")
	}

	// 2.2 Thor envia JoinFederation
	joinObj := wire.NewJoinFederationObject("ExampleFederation", "ThorLegado", "ExampleFederateType")
	joinPayload, err := wire.SerializePorticoControlMessage(joinObj)
	if err != nil {
		t.Fatalf("falha ao serializar JoinFederation do Thor: %v", err)
	}
	joinHdr := wire.NewControlRequest(wire.MsgJoinFederation, 2, 0, 0, uint32(len(joinPayload)))
	if err := clientThor.SendPacket(joinHdr, joinPayload); err != nil {
		t.Fatalf("falha ao enviar JoinFederation do Thor: %v", err)
	}

	// Ler resposta do Join do Thor (WelcomePack)
	_, welcomePayload, err := clientThor.ReadPacketTimeout(3 * time.Second)
	if err != nil {
		t.Fatalf("falha ao ler WelcomePack do Thor: %v", err)
	}
	wpInfo, err := wire.ParseWelcomePack(welcomePayload)
	if err != nil {
		t.Fatalf("falha ao interpretar WelcomePack do Thor: %v", err)
	}
	if wpInfo.FederateHandle != 1 {
		t.Errorf("esperava que Thor tivesse handle 1, obteve %d", wpInfo.FederateHandle)
	}

	// Verificar se o WelcomePack entregou o ObjectModel sintetizado do OMT
	wpObj, err := wire.DeserializePorticoControlMessage(welcomePayload)
	if err != nil {
		t.Fatalf("falha ao deserializar WelcomePack do Thor: %v", err)
	}
	results := wpObj.Get("results").(*metamodel.JavaObject)
	entries := results.Get("entries").(map[any]any)
	wpObjActual := entries["result"].(*metamodel.JavaObject)
	fomObj, ok := wpObjActual.Get("fom").(*metamodel.JavaObject)
	if !ok || fomObj == nil {
		t.Fatal("esperava que o WelcomePack entregue pelo Valhalla contivesse o ObjectModel sintetizado do OMT")
	}
	t.Logf("WelcomePack do Thor contem ObjectModel sintetizado com sucesso: version=%v, oclasses=%v",
		fomObj.Get("version"), fomObj.Get("oclasses"))

	// 3. Conectar Federado Nativo Valhalla (usando pkg/lrc sobre VAL4) com recorder real
	ambValhalla := lrc.NewRTIambassador()
	var valRecMu sync.Mutex
	var valReceivedAttr string
	fedAmb := &mixedRecorderAmb{
		onReflect: func(obj int32, attrs map[int32][]byte) {
			valRecMu.Lock()
			if v, ok := attrs[int32(common.HLAprivilegeToDeleteObjectHandle)]; ok {
				valReceivedAttr = string(v)
			}
			valRecMu.Unlock()
		},
	}

	if err := ambValhalla.Connect(fedAmb, crcAddr); err != nil {
		t.Fatalf("Valhalla Connect falhou: %v", err)
	}
	defer ambValhalla.Disconnect()

	valHandle, err := ambValhalla.JoinFederationExecution("ValhallaNativo", "ExampleFederation")
	if err != nil {
		t.Fatalf("Valhalla Join falhou: %v", err)
	}
	if valHandle != 2 {
		t.Errorf("esperava que Valhalla tivesse handle 2 (apos o Thor ter 1), obteve %d", valHandle)
	}

	// 4. Sincronização mista: Valhalla registra o ponto ReadyToRun
	if err := ambValhalla.RegisterFederationSynchronizationPoint("ReadyToRun", nil); err != nil {
		t.Fatalf("Valhalla RegisterFederationSynchronizationPoint falhou: %v", err)
	}

	// Thor conquista ReadyToRun
	syncObj := wire.NewSyncPointAchievedObject("ReadyToRun", wpInfo.FederateHandle)
	syncPayload, err := wire.SerializePorticoControlMessage(syncObj)
	if err != nil {
		t.Fatalf("falha ao serializar SyncPointAchieved do Thor: %v", err)
	}
	syncHdr := wire.NewControlRequest(wire.MsgAchieveSyncPoint, 2, uint16(wpInfo.FederateHandle), 0, uint32(len(syncPayload)))
	if err := clientThor.SendPacket(syncHdr, syncPayload); err != nil {
		t.Fatalf("falha ao enviar SyncPointAchieved do Thor: %v", err)
	}
	_, _, _ = clientThor.ReadPacketTimeout(1 * time.Second) // ControlResponseOK

	// Valhalla conquista ReadyToRun
	if err := ambValhalla.SynchronizationPointAchieved("ReadyToRun"); err != nil {
		t.Fatalf("Valhalla SynchronizationPointAchieved falhou: %v", err)
	}

	// 4.5. Pub/Sub bilateral conforme IEEE 1516-2025
	if err := ambValhalla.PublishObjectClassAttributes(int32(common.HLAobjectRootHandle), []int32{int32(common.HLAprivilegeToDeleteObjectHandle)}); err != nil {
		t.Fatalf("Valhalla Publish falhou: %v", err)
	}
	if err := ambValhalla.SubscribeObjectClassAttributes(int32(common.HLAobjectRootHandle), []int32{int32(common.HLAprivilegeToDeleteObjectHandle)}); err != nil {
		t.Fatalf("Valhalla Subscribe falhou: %v", err)
	}
	thorPubObj := metamodel.NewPublishObjectClass(int32(common.HLAobjectRootHandle), []any{metamodel.NewJavaInteger(int32(common.HLAprivilegeToDeleteObjectHandle))})
	thorPubPayload, _ := wire.SerializePorticoControlMessage(thorPubObj)
	if err := clientThor.SendPacket(wire.NewControlRequest(wire.MsgPublishObjectClass, 10, uint16(wpInfo.FederateHandle), 0, uint32(len(thorPubPayload))), thorPubPayload); err != nil {
		t.Fatalf("Thor Publish falhou: %v", err)
	}
	thorSubObj := metamodel.NewSubscribeObjectClass(int32(common.HLAobjectRootHandle), []any{metamodel.NewJavaInteger(int32(common.HLAprivilegeToDeleteObjectHandle))})
	thorSubPayload, _ := wire.SerializePorticoControlMessage(thorSubObj)
	if err := clientThor.SendPacket(wire.NewControlRequest(wire.MsgSubscribeObjectClass, 11, uint16(wpInfo.FederateHandle), 0, uint32(len(thorSubPayload))), thorSubPayload); err != nil {
		t.Fatalf("Thor Subscribe falhou: %v", err)
	}
	// Drain pending sync/pub/sub responses on Thor
	for {
		if _, _, err := clientThor.ReadPacketTimeout(50 * time.Millisecond); err != nil {
			break
		}
	}

	// 5. Valhalla registra objeto e atualiza atributos -> Thor DEVE receber MsgReflectAttributes
	objHandle, err := ambValhalla.RegisterObjectInstance(int32(common.HLAobjectRootHandle), "ValhallaUAV_01")
	if err != nil {
		t.Fatalf("Valhalla RegisterObjectInstance falhou: %v", err)
	}
	if objHandle != 1 {
		t.Errorf("esperava object handle 1, obteve %d", objHandle)
	}

	err = ambValhalla.UpdateAttributeValues(objHandle, map[int32][]byte{
		int32(common.HLAprivilegeToDeleteObjectHandle): []byte("Posicao:Lat-23.5_Long-46.6"),
	}, nil)
	if err != nil {
		t.Fatalf("Valhalla UpdateAttributeValues falhou: %v", err)
	}

	// Verify Thor receives ReflectAttributes from Valhalla
	thorGotReflect := false
	for i := 0; i < 5 && !thorGotReflect; i++ {
		h, _, rerr := clientThor.ReadPacketTimeout(1 * time.Second)
		if rerr != nil {
			break
		}
		if h.MessageType == wire.MsgReflectAttributes {
			thorGotReflect = true
		}
	}
	if !thorGotReflect {
		t.Fatalf("Thor did not receive MsgReflectAttributes from Valhalla")
	}

	// 5.1. Thor registra um objeto através do protocolo binário JOS Portico
	regPayload, err := wire.NewRegisterObjectMsg(int32(common.HLAobjectRootHandle), "ThorShip_01")
	if err != nil {
		t.Fatalf("falha ao serializar RegisterObject do Thor: %v", err)
	}
	regHdr := wire.NewControlRequest(wire.MsgRegisterObject, 3, uint16(wpInfo.FederateHandle), 0, uint32(len(regPayload)))
	if err := clientThor.SendPacket(regHdr, regPayload); err != nil {
		t.Fatalf("falha ao enviar RegisterObject do Thor: %v", err)
	}

	_, regRespPayload, err := clientThor.ReadPacketTimeout(2 * time.Second)
	if err != nil {
		t.Fatalf("falha ao receber resposta de RegisterObject do Thor: %v", err)
	}

	// Verificar se a resposta indica sucesso (isError == false)
	regObj, err := wire.DeserializePorticoControlMessage(regRespPayload)
	if err != nil {
		t.Fatalf("falha ao deserializar resposta do RegisterObject: %v", err)
	}
	if isErr, ok := regObj.Get("isError").(bool); ok && isErr {
		t.Fatalf("esperava sucesso no RegisterObject do Thor, obteve: %+v", regObj)
	}

	// 5.2. Thor atualiza atributos do objeto -> Valhalla DEVE receber a reflexão com o payload exato!
	uavPayload, err := wire.NewUpdateAttributesMsg(2, map[int32][]byte{
		int32(common.HLAprivilegeToDeleteObjectHandle): []byte("ThorShipCoords:Lat-12.0_Long-38.5"),
	}, nil)
	if err != nil {
		t.Fatalf("falha ao serializar UpdateAttributes do Thor: %v", err)
	}
	uavHdr := wire.NewControlRequest(wire.MsgUpdateAttributes, 4, uint16(wpInfo.FederateHandle), 0, uint32(len(uavPayload)))
	if err := clientThor.SendPacket(uavHdr, uavPayload); err != nil {
		t.Fatalf("falha ao enviar UpdateAttributes do Thor: %v", err)
	}
	_, _, _ = clientThor.ReadPacketTimeout(1 * time.Second)

	// Evocar callbacks no Valhalla até receber o payload exato do Thor
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, _ = ambValhalla.EvokeCallback(20 * time.Millisecond)
		valRecMu.Lock()
		got := valReceivedAttr
		valRecMu.Unlock()
		if got == "ThorShipCoords:Lat-12.0_Long-38.5" {
			break
		}
	}
	valRecMu.Lock()
	gotAttr := valReceivedAttr
	valRecMu.Unlock()
	if gotAttr != "ThorShipCoords:Lat-12.0_Long-38.5" {
		t.Fatalf("Valhalla did not receive Thor attribute reflection: got %q", gotAttr)
	}

	// 6. Desingresso limpo
	_ = ambValhalla.ResignFederationExecution()

	// Thor envia Resign
	resignObj := wire.NewResignFederationObject("ExampleFederation", "ThorLegado", "ieee1516e", "NO_ACTION")
	resignPayload, _ := wire.SerializePorticoControlMessage(resignObj)
	resignHdr := wire.NewControlRequest(wire.MsgResignFederation, 3, uint16(wpInfo.FederateHandle), 0, uint32(len(resignPayload)))
	_ = clientThor.SendPacket(resignHdr, resignPayload)
	_, _, _ = clientThor.ReadPacketTimeout(1 * time.Second)

	// Verificar integridade e encerramento
	time.Sleep(20 * time.Millisecond)
	fmt.Println(">>> ORQUESTRACAO MISTA CONCLUIDA COM 100% DE SUCESSO! <<<")
}
