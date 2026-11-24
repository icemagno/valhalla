package wire

import (
	"testing"
)

func TestJOSDecoder_Roundtrip_RtiProbe(t *testing.T) {
	obj := NewRtiProbeObject()
	buf, err := SerializePorticoControlMessage(obj)
	if err != nil {
		t.Fatalf("falha ao serializar RtiProbe: %v", err)
	}

	decodedObj, err := DeserializePorticoControlMessage(buf)
	if err != nil {
		t.Fatalf("falha ao deserializar RtiProbe: %v", err)
	}

	if decodedObj.Class.Name != ClassDescRtiProbe.Name {
		t.Errorf("classe esperada %s, obtida %s", ClassDescRtiProbe.Name, decodedObj.Class.Name)
	}

	if decodedObj.Get("targetFederate") != int32(0) {
		t.Errorf("targetFederate esperado 0, obtido %v", decodedObj.Get("targetFederate"))
	}
}

func TestJOSDecoder_Roundtrip_JoinFederation(t *testing.T) {
	fedName := "ExampleFederation"
	nodeName := "ValhallaNode"
	nodeType := "ieee1516e"

	obj := NewJoinFederationObject(fedName, nodeName, nodeType)
	buf, err := SerializePorticoControlMessage(obj)
	if err != nil {
		t.Fatalf("falha ao serializar JoinFederation: %v", err)
	}

	decodedObj, err := DeserializePorticoControlMessage(buf)
	if err != nil {
		t.Fatalf("falha ao deserializar JoinFederation: %v", err)
	}

	if decodedObj.Class.Name != ClassDescJoinFederation.Name {
		t.Fatalf("classe esperada %s, obtida %s", ClassDescJoinFederation.Name, decodedObj.Class.Name)
	}

	if decodedObj.Get("federationName") != fedName {
		t.Errorf("federationName esperado %s, obtido %v", fedName, decodedObj.Get("federationName"))
	}
	if decodedObj.Get("federateName") != nodeName {
		t.Errorf("federateName esperado %s, obtido %v", nodeName, decodedObj.Get("federateName"))
	}
	if decodedObj.Get("federateType") != nodeType {
		t.Errorf("federateType esperado %s, obtido %v", nodeType, decodedObj.Get("federateType"))
	}
}

func TestJOSDecoder_Roundtrip_ResignFederationWithEnum(t *testing.T) {
	fedName := "ExampleFederation"
	nodeName := "ValhallaWarrior"
	nodeType := "ieee1516e"
	action := "NO_ACTION"

	obj := NewResignFederationObject(fedName, nodeName, nodeType, action)
	buf, err := SerializePorticoControlMessage(obj)
	if err != nil {
		t.Fatalf("falha ao serializar ResignFederation: %v", err)
	}

	decodedObj, err := DeserializePorticoControlMessage(buf)
	if err != nil {
		t.Fatalf("falha ao deserializar ResignFederation: %v", err)
	}

	if decodedObj.Class.Name != ClassDescResignFederation.Name {
		t.Fatalf("classe esperada %s, obtida %s", ClassDescResignFederation.Name, decodedObj.Class.Name)
	}

	enumVal := decodedObj.Get("resignAction")
	enumObj, ok := enumVal.(*JavaEnum)
	if !ok {
		t.Fatalf("resignAction esperado *JavaEnum, obtido %T (%v)", enumVal, enumVal)
	}
	if enumObj.Name != action {
		t.Errorf("constante enum esperada %s, obtida %s", action, enumObj.Name)
	}
	if enumObj.Class.Name != ClassDescJResignAction.Name {
		t.Errorf("classe enum esperada %s, obtida %s", ClassDescJResignAction.Name, enumObj.Class.Name)
	}
}

func TestJOSDecoder_Roundtrip_SyncPointAchieved(t *testing.T) {
	label := "ReadyToRun"
	obj := NewSyncPointAchievedObject(label, 7)
	buf, err := SerializePorticoControlMessage(obj)
	if err != nil {
		t.Fatalf("falha ao serializar SyncPointAchieved: %v", err)
	}

	decodedObj, err := DeserializePorticoControlMessage(buf)
	if err != nil {
		t.Fatalf("falha ao deserializar SyncPointAchieved: %v", err)
	}

	if decodedObj.Class.Name != ClassDescSyncPointAchieved.Name {
		t.Fatalf("classe esperada %s, obtida %s", ClassDescSyncPointAchieved.Name, decodedObj.Class.Name)
	}
	if decodedObj.Get("label") != label {
		t.Errorf("label esperado %s, obtido %v", label, decodedObj.Get("label"))
	}
}

func TestJOSDecoder_ParseWelcomePack(t *testing.T) {
	// Payload simulando bloco serializado do WelcomePack
	// "WelcomePack" + SUID (8b) + Flags (1b: 0x0C) + NumFields (2b: 0x0000) + TC_ENDBLOCKDATA (1b: 0x78) + SuperClass (1b: 0x70) + TC_BLOCKDATA (1b: 0x77) + BlockLen (1b: 0x41)
	prefix := []byte("WelcomePack\x03\xCC\x42\xB0\xDA\x80\x25\xD4\x0C\x00\x00\x78\x70\x77\x41")
	// 4 bytes fedHandle (1), 4 bytes federateHandle (42)
	handles := []byte{0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x2A}
	// UTF strings:
	fedName := []byte("\x00\x11ExampleFederation")
	fedNode := []byte("\x00\x0CValhallaNode")
	fedType := []byte("\x00\x09ieee1516e")

	var raw []byte
	raw = append(raw, prefix...)
	raw = append(raw, handles...)
	raw = append(raw, fedName...)
	raw = append(raw, fedNode...)
	raw = append(raw, fedType...)

	info, err := ParseWelcomePack(raw)
	if err != nil {
		t.Fatalf("falha ao parsear WelcomePack: %v", err)
	}

	if info.FederationHandle != 1 {
		t.Errorf("FederationHandle esperado 1, obtido %d", info.FederationHandle)
	}
	if info.FederateHandle != 42 {
		t.Errorf("FederateHandle esperado 42, obtido %d", info.FederateHandle)
	}
	if info.FederationName != "ExampleFederation" {
		t.Errorf("FederationName esperado ExampleFederation, obtido %s", info.FederationName)
	}
	if info.FederateName != "ValhallaNode" {
		t.Errorf("FederateName esperado ValhallaNode, obtido %s", info.FederateName)
	}
	if info.FederateType != "ieee1516e" {
		t.Errorf("FederateType esperado ieee1516e, obtido %s", info.FederateType)
	}
}

func TestJOSDecoder_InvalidHeaders(t *testing.T) {
	// 1. Dados muito curtos
	dec := NewJOSDecoder([]byte{0xAC, 0xED})
	if err := dec.ReadStreamHeader(); err == nil {
		t.Error("esperava erro para header truncado")
	}

	// 2. Magic incorreto
	dec2 := NewJOSDecoder([]byte{0x12, 0x34, 0x00, 0x05})
	if err := dec2.ReadStreamHeader(); err == nil {
		t.Error("esperava erro para STREAM_MAGIC incorreto")
	}
}
