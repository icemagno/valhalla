package transport

import (
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"github.com/icemagno/valhalla/pkg/portico/wire"
)

// mockPorticoServer simula fielmente a rotina do TcpServerTransport e TcpClientProxy do Portico
func mockPorticoServer(t *testing.T, ln net.Listener, welcomeText string) {
	conn, err := ln.Accept()
	if err != nil {
		return
	}
	defer conn.Close()

	// 1. Enviar magic de boas-vindas 0xBEEF
	if err := binary.Write(conn, binary.BigEndian, MagicWelcome); err != nil {
		t.Errorf("mock: erro enviando 0xBEEF: %v", err)
		return
	}

	// 2. Enviar tamanho da string de boas-vindas
	msgBytes := []byte(welcomeText)
	if err := binary.Write(conn, binary.BigEndian, int32(len(msgBytes))); err != nil {
		t.Errorf("mock: erro enviando tamanho: %v", err)
		return
	}

	// 3. Enviar texto de boas-vindas
	if _, err := conn.Write(msgBytes); err != nil {
		t.Errorf("mock: erro enviando texto: %v", err)
		return
	}

	// 4. Enviar sinal de prontidao 0xFEED
	if err := binary.Write(conn, binary.BigEndian, MagicReady); err != nil {
		t.Errorf("mock: erro enviando 0xFEED: %v", err)
		return
	}

	// 5. Ler resposta de prontidao do cliente (0xFEED)
	var clientReady int32
	if err := binary.Read(conn, binary.BigEndian, &clientReady); err != nil {
		t.Errorf("mock: erro lendo resposta do cliente: %v", err)
		return
	}
	if clientReady != MagicReady {
		t.Errorf("mock: resposta do cliente invalida: 0x%X", clientReady)
		return
	}

	// 6. Loop de eco/recepcao de bundles 0xCAFE
	for {
		var magic int32
		if err := binary.Read(conn, binary.BigEndian, &magic); err != nil {
			return // Conexao encerrada normalmente
		}
		if magic != MagicTCPBundle {
			t.Errorf("mock: magic de bundle inesperado: 0x%X", magic)
			return
		}

		var bundleLen int32
		if err := binary.Read(conn, binary.BigEndian, &bundleLen); err != nil {
			return
		}

		buf := make([]byte, bundleLen)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return
		}

		var hdr wire.Header
		if err := hdr.UnmarshalBinary(buf[:wire.HeaderLength]); err != nil {
			t.Errorf("mock: erro decodificando cabecalho: %v", err)
			return
		}

		// Responder de volta com uma confirmacao de sucesso (ControlResponseOK)
		respHdr := wire.Header{
			Flags:         0,
			PayloadLength: 0,
			FederationID:  hdr.FederationID,
			CallType:      wire.CallTypeControlResponseOK,
			MessageType:   hdr.MessageType,
			RequestId:     hdr.RequestId,
			SourceHandle:  0, // RTI handle
			TargetHandle:  hdr.SourceHandle,
		}

		var respBuf [wire.HeaderLength]byte
		_ = respHdr.MarshalTo(respBuf[:])

		_ = binary.Write(conn, binary.BigEndian, MagicTCPBundle)
		_ = binary.Write(conn, binary.BigEndian, int32(wire.HeaderLength))
		_, _ = conn.Write(respBuf[:])
	}
}

func TestTCPClient_HandshakeAndExchange(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("falha ao abrir listener local: %v", err)
	}
	defer ln.Close()

	welcomeStr := "Portico 2.2.0 (Valhalla Mock Server)"
	go mockPorticoServer(t, ln, welcomeStr)

	client := NewTCPClient(ln.Addr().String())
	err = client.Connect(2 * time.Second)
	if err != nil {
		t.Fatalf("falha ao conectar e executar handshake: %v", err)
	}
	defer client.Close()

	if !client.IsConnected() {
		t.Fatal("esperava que o cliente estivesse conectado")
	}

	if client.WelcomeMessage() != welcomeStr {
		t.Errorf("mensagem de boas-vindas incorreta: esperado %q, obtido %q", welcomeStr, client.WelcomeMessage())
	}

	// Testar envio de pacote (RtiProbe)
	reqHdr := wire.NewControlRequest(wire.MsgRtiProbe, 101, 10, 0, 0)
	if err := client.SendPacket(reqHdr, nil); err != nil {
		t.Fatalf("falha ao enviar pacote bundle: %v", err)
	}

	// Receber resposta de controle
	respHdr, payload, err := client.ReadPacket()
	if err != nil {
		t.Fatalf("falha ao receber resposta bundle: %v", err)
	}

	if respHdr.CallType != wire.CallTypeControlResponseOK {
		t.Errorf("CallType inesperado na resposta: %s", respHdr.CallType)
	}
	if respHdr.RequestId != 101 {
		t.Errorf("RequestId inesperado na resposta: esperado 101, obtido %d", respHdr.RequestId)
	}
	if len(payload) != 0 {
		t.Errorf("payload inesperado na resposta: %d bytes", len(payload))
	}
}

func TestTCPClient_ConnectionFailure(t *testing.T) {
	// Tentar conectar em porta fechada
	client := NewTCPClient("127.0.0.1:54321")
	err := client.Connect(200 * time.Millisecond)
	if err == nil {
		t.Fatal("esperava erro de conexao, mas passou")
	}
	if client.IsConnected() {
		t.Fatal("cliente nao deveria estar marcado como conectado")
	}
}
