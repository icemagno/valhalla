package wire

import (
	"encoding/binary"
	"testing"
)

func TestJOS_GenericEncoder_RtiProbe(t *testing.T) {
	obj := NewRtiProbeObject()
	payload, err := SerializePorticoControlMessage(obj)
	if err != nil {
		t.Fatalf("falha ao serializar RtiProbe com motor generico: %v", err)
	}
	if len(payload) < 7 {
		t.Fatalf("payload do RtiProbe muito curto: %d bytes", len(payload))
	}

	// 1. Validar Magic (0xACED), Version (0x0005) e ManualMarshal=false
	if binary.BigEndian.Uint16(payload[0:2]) != 0xACED || binary.BigEndian.Uint16(payload[2:4]) != 0x0005 {
		t.Fatalf("cabecalho JOS invalido: %X", payload[:4])
	}
	if payload[4] != TC_BLOCKDATA || payload[5] != 1 || payload[6] != 0 {
		t.Fatalf("bloco manual marshal incorreto: %02X %02X %02X", payload[4], payload[5], payload[6])
	}

	// 2. Round-trip estrutural real via DeserializePorticoControlMessage
	dec, err := DeserializePorticoControlMessage(payload)
	if err != nil {
		t.Fatalf("falha ao desserializar RtiProbe: %v", err)
	}
	if dec.Class == nil || dec.Class.Name != ClassDescRtiProbe.Name {
		t.Fatalf("classe desserializada incorreta: got %+v, want %s", dec.Class, ClassDescRtiProbe.Name)
	}
	if dec.Class.Super == nil || dec.Class.Super.Name != ClassDescPorticoMessage.Name {
		t.Fatalf("superclasse esperada %s, obtida %+v", ClassDescPorticoMessage.Name, dec.Class.Super)
	}
}

func TestJOS_GenericEncoder_JoinFederation(t *testing.T) {
	fedName := "ExampleFederation"
	fedType := "ExampleFederateType"
	nodeName := "ValhallaNode"

	obj := NewJoinFederationObject(fedName, nodeName, fedType)
	payload, err := SerializePorticoControlMessage(obj)
	if err != nil {
		t.Fatalf("falha ao serializar JoinFederation com motor generico: %v", err)
	}

	dec, err := DeserializePorticoControlMessage(payload)
	if err != nil {
		t.Fatalf("falha ao desserializar JoinFederation: %v", err)
	}
	if dec.Class == nil || dec.Class.Name != ClassDescJoinFederation.Name {
		t.Fatalf("classe esperada %s, obtida %+v", ClassDescJoinFederation.Name, dec.Class)
	}
	if dec.Get("federationName") != fedName {
		t.Errorf("federationName = %v, want %q", dec.Get("federationName"), fedName)
	}
	if dec.Get("federateName") != nodeName {
		t.Errorf("federateName = %v, want %q", dec.Get("federateName"), nodeName)
	}
	if dec.Get("federateType") != fedType {
		t.Errorf("federateType = %v, want %q", dec.Get("federateType"), fedType)
	}
}

func TestJOS_GenericEncoder_CreateAndResign(t *testing.T) {
	createObj := NewCreateFederationObject("ValhallaExecution")
	createBuf, err := SerializePorticoControlMessage(createObj)
	if err != nil {
		t.Fatalf("falha ao serializar CreateFederation: %v", err)
	}
	createDec, err := DeserializePorticoControlMessage(createBuf)
	if err != nil {
		t.Fatalf("falha ao desserializar CreateFederation: %v", err)
	}
	if createDec.Class == nil || createDec.Class.Name != ClassDescCreateFederation.Name || createDec.Get("federationName") != "ValhallaExecution" {
		t.Fatalf("CreateFederation roundtrip divergente: %+v", createDec)
	}

	resignObj := NewResignFederationObject("ValhallaExecution", "ValhallaNode", "ieee1516e", "NO_ACTION")
	resignBuf, err := SerializePorticoControlMessage(resignObj)
	if err != nil {
		t.Fatalf("falha ao serializar ResignFederation: %v", err)
	}
	resignDec, err := DeserializePorticoControlMessage(resignBuf)
	if err != nil {
		t.Fatalf("falha ao desserializar ResignFederation: %v", err)
	}
	if resignDec.Class == nil || resignDec.Class.Name != ClassDescResignFederation.Name {
		t.Fatalf("ResignFederation classe incorreta: %+v", resignDec.Class)
	}
	if resignDec.Get("federationName") != "ValhallaExecution" || resignDec.Get("federateName") != "ValhallaNode" {
		t.Fatalf("ResignFederation campos divergentes: %+v", resignDec.Fields)
	}

	syncObj := NewSyncPointAchievedObject("ReadyToRun", 3)
	syncBuf, err := SerializePorticoControlMessage(syncObj)
	if err != nil {
		t.Fatalf("falha ao serializar SyncPointAchieved: %v", err)
	}
	syncDec, err := DeserializePorticoControlMessage(syncBuf)
	if err != nil {
		t.Fatalf("falha ao desserializar SyncPointAchieved: %v", err)
	}
	if syncDec.Class == nil || syncDec.Class.Name != ClassDescSyncPointAchieved.Name || syncDec.Get("label") != "ReadyToRun" {
		t.Fatalf("SyncPointAchieved roundtrip divergente: %+v", syncDec)
	}
}
