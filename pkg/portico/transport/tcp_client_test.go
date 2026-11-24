package transport

import (
	"encoding/binary"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/icemagno/valhalla/pkg/portico/wire"
)

// mockPorticoServer simula o comportamento exato do TcpClientProxy.java do Portico
func startMockPorticoServer(t *testing.T) (net.Listener, <-chan error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("falha ao criar mock server: %v", err)
	}

	errChan := make(chan error, 1)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			errChan <- err
			return
		}
		defer conn.Close()

		// 1. Enviar 0xBEEF
		if err := binary.Write(conn, binary.BigEndian, MagicWelcome); err != nil {
			errChan <- err
			return
		}

		// 2. Enviar tamanho e mensagem de boas-vindas
		welcome := "Portico Router (3.0.0): Your ID 999"
		buf := []byte(welcome)
		if err := binary.Write(conn, binary.BigEndian, int32(len(buf))); err != nil {
			errChan <- err
			return
		}
		if _, err := conn.Write(buf); err != nil {
			errChan <- err
			return
		}

		// 3. Enviar 0xFEED
		if err := binary.Write(conn, binary.BigEndian, MagicReady); err != nil {
			errChan <- err
			return
		}

		// 4. Aguardar o cliente responder 0xFEED
		var clientReady int32
		if err := binary.Read(conn, binary.BigEndian, &clientReady); err != nil {
			errChan <- err
			return
		}

		if clientReady != MagicReady {
			errChan <- ErrInvalidReadyCode
			return
		}

		// Sincronizacao bem sucedida! Agora aguardar um pacote
		hdrBuf := make([]byte, wire.HeaderSize)
		if _, err := conn.Read(hdrBuf); err != nil {
			errChan <- err
			return
		}

		errChan <- nil
	}()

	return ln, errChan
}

func TestTCPClient_MockHandshake_Success(t *testing.T) {
	ln, errChan := startMockPorticoServer(t)
	defer ln.Close()

	client := NewTCPClient(ln.Addr().String())
	err := client.Connect(2 * time.Second)
	if err != nil {
		t.Fatalf("erro ao conectar e executar handshake: %v", err)
	}
	defer client.Close()

	if !client.IsConnected() {
		t.Fatal("cliente deveria estar marcado como conectado")
	}

	expectedPrefix := "Portico Router (3.0.0)"
	if !strings.HasPrefix(client.WelcomeMessage(), expectedPrefix) {
		t.Fatalf("mensagem de boas-vindas inesperada: %s (esperava prefixo %s)", client.WelcomeMessage(), expectedPrefix)
	}
	t.Logf("Mensagem de boas-vindas recebida: %s", client.WelcomeMessage())

	// Testar envio de pacote pelo cliente
	hdr := wire.NewHeader(wire.MsgCreateFederation, 1, -1, 0)
	if err := client.SendPacket(hdr, nil); err != nil {
		t.Fatalf("falha ao enviar pacote: %v", err)
	}

	// Aguardar confirmacao do servidor mock
	select {
	case err := <-errChan:
		if err != nil {
			t.Fatalf("erro no servidor mock: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout aguardando servidor mock")
	}
}
