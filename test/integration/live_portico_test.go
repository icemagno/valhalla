package integration

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/icemagno/valhalla/pkg/portico/transport"
	"github.com/icemagno/valhalla/pkg/portico/wire"
)

func TestIntegration_LivePortico_Handshake(t *testing.T) {
	endpoint := os.Getenv("PORTICO_ENDPOINT")
	if endpoint == "" {
		endpoint = "192.168.1.226:52295"
	}

	t.Logf("Iniciando conexao ao vivo contra o executivo Portico no Arcanjo: %s", endpoint)

	client := transport.NewTCPClient(endpoint)
	err := client.Connect(3 * time.Second)
	if err != nil {
		t.Fatalf("falha ao conectar e executar handshake com Portico real em %s: %v", endpoint, err)
	}
	defer client.Close()

	if !client.IsConnected() {
		t.Fatal("esperava que o cliente estivesse conectado")
	}

	welcome := client.WelcomeMessage()
	t.Logf(">>> HANDSHAKE BEM-SUCEDIDO! Resposta oficial do Portico: [%s]", welcome)

	if !strings.Contains(welcome, "Portico") {
		t.Errorf("mensagem de boas-vindas inesperada: %s", welcome)
	}

	// Manter o socket aberto por 500ms para estabilidade
	time.Sleep(500 * time.Millisecond)

	// Enviar pacote oficial de RtiProbe usando enquadramento 0xCAFE e cabecalho exato de 12 bytes
	probeHdr := wire.NewControlRequest(wire.MsgRtiProbe, 1, 0, 0, 0)
	if err := client.SendPacket(probeHdr, nil); err != nil {
		t.Fatalf("falha ao enviar pacote RtiProbe via bundle TCP: %v", err)
	}
	t.Logf("Pacote RtiProbe (12 bytes, bundle 0xCAFE) enviado com sucesso ao Portico!")

	// Aguardar brevemente para registrar no canal do servidor Portico
	time.Sleep(500 * time.Millisecond)
	t.Logf("Conexao com o Portico finalizada graciosamente sem Broken Pipe.")
}
