package crc

import (
	"net"
	"testing"
	"time"
)

func TestMultiplexer_DetectNativeVAL4(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	go func() {
		_, _ = client.Write([]byte("VAL4ExtraData"))
	}()

	proto, wrapped, err := DetectProtocol(server, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("erro ao detectar protocolo: %v", err)
	}

	if proto != ProtocolNative {
		t.Errorf("esperado ProtocolNative, obtido %s", proto)
	}

	// Verificar se os 4 bytes VAL4 foram descartados e restou "ExtraData"
	buf := make([]byte, 9)
	n, err := wrapped.Read(buf)
	if err != nil {
		t.Fatalf("erro lendo dados restantes: %v", err)
	}
	if string(buf[:n]) != "ExtraData" {
		t.Errorf("esperado 'ExtraData', obtido '%s'", string(buf[:n]))
	}
}

func TestMultiplexer_DetectPorticoPassive(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	// O cliente NÃO escreve nada (comportamento passivo do Portico/Thor)
	proto, _, err := DetectProtocol(server, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("erro ao detectar protocolo: %v", err)
	}

	if proto != ProtocolPortico {
		t.Errorf("esperado ProtocolPortico, obtido %s", proto)
	}
}
