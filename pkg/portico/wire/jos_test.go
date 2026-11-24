package wire

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestJOS_GenericEncoder_RtiProbe(t *testing.T) {
	obj := NewRtiProbeObject()
	payload, err := SerializePorticoControlMessage(obj)
	if err != nil {
		t.Fatalf("falha ao serializar RtiProbe com motor generico: %v", err)
	}

	if len(payload) == 0 {
		t.Fatal("payload do RtiProbe gerado com 0 bytes")
	}

	// 1. Validar Magic (0xACED) e Version (0x0005)
	if binary.BigEndian.Uint16(payload[0:2]) != 0xACED {
		t.Errorf("magic JOS incorreto: esperado 0xACED, obtido 0x%04X", binary.BigEndian.Uint16(payload[0:2]))
	}
	if binary.BigEndian.Uint16(payload[2:4]) != 0x0005 {
		t.Errorf("versao JOS incorreta: esperado 0x0005, obtido 0x%04X", binary.BigEndian.Uint16(payload[2:4]))
	}

	// 2. Validar Bloco de 1 byte com false (ManualMarshal = false para controle Portico)
	if payload[4] != TC_BLOCKDATA || payload[5] != 1 || payload[6] != 0 {
		t.Errorf("bloco manual marshal incorreto: %02X %02X %02X", payload[4], payload[5], payload[6])
	}

	// 3. Validar se o nome da classe RtiProbe está presente
	if !bytes.Contains(payload, []byte(ClassDescRtiProbe.Name)) {
		t.Errorf("classe %s nao encontrada no payload", ClassDescRtiProbe.Name)
	}

	// 4. Validar se o nome da superclasse PorticoMessage está presente
	if !bytes.Contains(payload, []byte(ClassDescPorticoMessage.Name)) {
		t.Errorf("classe %s nao encontrada no payload", ClassDescPorticoMessage.Name)
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

	if len(payload) == 0 {
		t.Fatal("payload do JoinFederation gerado com 0 bytes")
	}

	// 1. Validar Magic e Versao
	if binary.BigEndian.Uint16(payload[0:2]) != 0xACED || binary.BigEndian.Uint16(payload[2:4]) != 0x0005 {
		t.Fatal("cabecalho de stream Java invalido")
	}

	// 2. Validar que contem os nomes parametrizados
	if !bytes.Contains(payload, []byte(fedName)) {
		t.Errorf("nome da federacao %q nao encontrado no payload", fedName)
	}
	if !bytes.Contains(payload, []byte(nodeName)) {
		t.Errorf("nome do federado %q nao encontrado no payload", nodeName)
	}
	if !bytes.Contains(payload, []byte(fedType)) {
		t.Errorf("tipo do federado %q nao encontrado no payload", fedType)
	}

	// 3. Validar presenca das classes
	if !bytes.Contains(payload, []byte(ClassDescJoinFederation.Name)) {
		t.Errorf("classe %s nao encontrada no payload", ClassDescJoinFederation.Name)
	}
	if !bytes.Contains(payload, []byte(ClassDescArrayList.Name)) {
		t.Errorf("classe %s nao encontrada no payload", ClassDescArrayList.Name)
	}
}

func TestJOS_GenericEncoder_CreateAndResign(t *testing.T) {
	// Validar que o mesmo motor generico serializa CreateFederation e ResignFederation
	createObj := NewCreateFederationObject("ValhallaExecution")
	createBuf, err := SerializePorticoControlMessage(createObj)
	if err != nil {
		t.Fatalf("falha ao serializar CreateFederation: %v", err)
	}
	if !bytes.Contains(createBuf, []byte(ClassDescCreateFederation.Name)) {
		t.Errorf("classe %s nao encontrada", ClassDescCreateFederation.Name)
	}

	resignObj := NewResignFederationObject("ValhallaExecution", "ValhallaNode", "ieee1516e", "NO_ACTION")
	resignBuf, err := SerializePorticoControlMessage(resignObj)
	if err != nil {
		t.Fatalf("falha ao serializar ResignFederation: %v", err)
	}
	if !bytes.Contains(resignBuf, []byte(ClassDescResignFederation.Name)) {
		t.Errorf("classe %s nao encontrada", ClassDescResignFederation.Name)
	}
	if !bytes.Contains(resignBuf, []byte(ClassDescJResignAction.Name)) {
		t.Errorf("classe enum %s nao encontrada", ClassDescJResignAction.Name)
	}
	if !bytes.Contains(resignBuf, []byte("NO_ACTION")) {
		t.Errorf("constante enum NO_ACTION nao encontrada")
	}

	// Validar SyncPointAchieved
	syncObj := NewSyncPointAchievedObject("ReadyToRun", 3)
	syncBuf, err := SerializePorticoControlMessage(syncObj)
	if err != nil {
		t.Fatalf("falha ao serializar SyncPointAchieved: %v", err)
	}
	if !bytes.Contains(syncBuf, []byte(ClassDescSyncPointAchieved.Name)) {
		t.Errorf("classe %s nao encontrada", ClassDescSyncPointAchieved.Name)
	}
	if !bytes.Contains(syncBuf, []byte("ReadyToRun")) {
		t.Errorf("label ReadyToRun nao encontrada")
	}
}
