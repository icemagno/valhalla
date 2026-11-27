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

func TestMultiplexer_AlienProtocolRejected(t *testing.T) {
	alienPayloads := [][]byte{
		[]byte("HTTP/1.1 200 OK\r\n"),
		[]byte("POST /api/v1 HTTP/1.1\r\n"),
		[]byte("SSH-2.0-OpenSSH_8.2\r\n"),
		{0xDE, 0xAD, 0xBE, 0xEF},
		{0x00, 0x00, 0x00, 0x01},
	}

	for _, payload := range alienPayloads {
		server, client := net.Pipe()

		go func(p []byte) {
			_, _ = client.Write(p)
			_ = client.Close()
		}(payload)

		proto, _, err := DetectProtocol(server, 20*time.Millisecond)
		_ = server.Close()

		if err == nil {
			t.Errorf("esperado erro ao detectar payload alienigena %q, mas obteve nil (proto=%s)", string(payload), proto)
		}
		if proto != ProtocolUnknown {
			t.Errorf("esperado ProtocolUnknown para payload alienigena %q, obtido %s", string(payload), proto)
		}
	}
}

func TestMultiplexer_PartialNativePreamble_NotClassifiedAsPortico(t *testing.T) {
	// Cliente envia "VAL" (3 bytes do preambulo nativo) e para/pausa
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	go func() {
		_, _ = client.Write([]byte{'V', 'A', 'L'})
		// Nao envia o '4' imediatamente
		time.Sleep(50 * time.Millisecond)
		_ = client.Close()
	}()

	proto, _, err := DetectProtocol(server, 10*time.Millisecond)
	if proto == ProtocolPortico {
		t.Fatalf("VULNERABILIDADE: preambulo nativo parcial ('VAL') foi erroneamente classificado como ProtocolPortico!")
	}
	if err == nil {
		t.Fatalf("esperado erro de preambulo incompleto, obteve sucesso com proto=%s", proto)
	}
}

func TestMultiplexer_SegmentedNativePreamble_Success(t *testing.T) {
	// Cliente nativo com alta latencia enviando preambulo fragmentado byte a byte
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	go func() {
		for _, b := range []byte{'V', 'A', 'L', '4'} {
			_, _ = client.Write([]byte{b})
			time.Sleep(5 * time.Millisecond)
		}
		_, _ = client.Write([]byte("PayloadData"))
	}()

	proto, wrapped, err := DetectProtocol(server, 15*time.Millisecond)
	if err != nil {
		t.Fatalf("falha ao detectar preambulo nativo fragmentado: %v", err)
	}
	if proto != ProtocolNative {
		t.Fatalf("esperado ProtocolNative para preambulo fragmentado 'VAL4', obtido %s", proto)
	}

	buf := make([]byte, 11)
	n, err := wrapped.Read(buf)
	if err != nil {
		t.Fatalf("erro lendo dados apos preambulo fragmentado: %v", err)
	}
	if string(buf[:n]) != "PayloadData" {
		t.Errorf("esperado 'PayloadData', obtido '%s'", string(buf[:n]))
	}
}
