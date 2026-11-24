package wire

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestJOS_RtiProbePayload(t *testing.T) {
	payload := NewRtiProbePayload()

	if len(payload) == 0 {
		t.Fatal("payload do RtiProbe gerado com 0 bytes")
	}

	// 1. Validar Magic (0xACED) e Version (0x0005)
	if binary.BigEndian.Uint16(payload[0:2]) != JOSMagic {
		t.Errorf("magic JOS incorreto: esperado 0xACED, obtido 0x%04X", binary.BigEndian.Uint16(payload[0:2]))
	}
	if binary.BigEndian.Uint16(payload[2:4]) != JOSVersion {
		t.Errorf("versao JOS incorreta: esperado 0x0005, obtido 0x%04X", binary.BigEndian.Uint16(payload[2:4]))
	}

	// 2. Validar Bloco com boolean false (ManualMarshal = false)
	if payload[4] != TC_BLOCKDATA || payload[5] != 1 || payload[6] != 0 {
		t.Errorf("bloco manual marshal incorreto: %02X %02X %02X", payload[4], payload[5], payload[6])
	}

	// 3. Validar se o nome da classe RtiProbe está presente
	if !bytes.Contains(payload, []byte(ClassRtiProbe)) {
		t.Errorf("classe %s nao encontrada no payload", ClassRtiProbe)
	}

	// 4. Validar se o nome da superclasse PorticoMessage está presente
	if !bytes.Contains(payload, []byte(ClassPorticoMessage)) {
		t.Errorf("classe %s nao encontrada no payload", ClassPorticoMessage)
	}
}

func TestJOS_JoinFederationPayload(t *testing.T) {
	fedName := "ExampleFederation"
	fedType := "ExampleFederateType"
	nodeName := "ValhallaNode"

	payload := NewJoinFederationPayload(fedName, nodeName, fedType)

	if len(payload) == 0 {
		t.Fatal("payload do JoinFederation gerado com 0 bytes")
	}

	// 1. Validar Magic e Versao
	if binary.BigEndian.Uint16(payload[0:2]) != JOSMagic || binary.BigEndian.Uint16(payload[2:4]) != JOSVersion {
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
	if !bytes.Contains(payload, []byte(ClassJoinFederation)) {
		t.Errorf("classe %s nao encontrada no payload", ClassJoinFederation)
	}
	if !bytes.Contains(payload, []byte(ClassArrayList)) {
		t.Errorf("classe %s nao encontrada no payload", ClassArrayList)
	}
}
