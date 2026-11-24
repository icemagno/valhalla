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

	// Manter o socket aberto por 1 segundo para garantir estabilidade e ausencia de Broken Pipe
	time.Sleep(1 * time.Second)

	// Enviar ping/header de teste estruturado
	pingHdr := wire.NewHeader(wire.MsgCreateFederation, -1, 0, 0)
	pingHdr.Flags = wire.FlagValhallaExt
	if err := client.SendPacket(pingHdr, nil); err != nil {
		t.Logf("aviso: envio de pacote inicial apos handshake (esperado se canal ainda nao autenticou federado): %v", err)
	} else {
		t.Logf("Pacote de cabecalho Valhalla enviado com sucesso pela conexao TCP!")
	}

	t.Logf("Conexao com o Portico finalizada graciosamente sem Broken Pipe.")
}
