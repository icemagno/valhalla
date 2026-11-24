package integration

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/icemagno/valhalla/pkg/portico/transport"
	"github.com/icemagno/valhalla/pkg/portico/wire"
)

func TestIntegration_LivePortico_HandshakeAndProbe(t *testing.T) {
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

	// 1. Enviar pacote RtiProbe com payload oficial no formato JOS
	probePayload := wire.NewRtiProbePayload()
	probeHdr := wire.NewControlRequest(wire.MsgRtiProbe, 1, 0, 0, uint32(len(probePayload)))

	t.Logf("Enviando RtiProbe (cabecalho 12 bytes + payload JOS %d bytes, bundle 0xCAFE)...", len(probePayload))
	if err := client.SendPacket(probeHdr, probePayload); err != nil {
		t.Fatalf("falha ao enviar RtiProbe: %v", err)
	}

	// Aguardar e ler resposta do Portico (timeout de 2 segundos)
	respHdr, respPayload, err := client.ReadPacketTimeout(2 * time.Second)
	if err != nil {
		t.Logf("Aviso na leitura de resposta do RtiProbe: %v", err)
	} else {
		t.Logf(">>> RESPOSTA RECEBIDA DO PORTICO PARA RtiProbe: Tipo: %s, Call: %s, ReqId: %d, PayloadLen: %d",
			respHdr.MessageType, respHdr.CallType, respHdr.RequestId, len(respPayload))
	}

	// 2. Enviar JoinFederation para a ExampleFederation ativa onde o Thor está aguardando
	joinPayload := wire.NewJoinFederationPayload("ExampleFederation", "ValhallaNode", "ExampleFederateType")
	joinHdr := wire.NewControlRequest(wire.MsgJoinFederation, 2, 0, 0, uint32(len(joinPayload)))

	t.Logf("Enviando JoinFederation na ExampleFederation (payload JOS %d bytes)...", len(joinPayload))
	if err := client.SendPacket(joinHdr, joinPayload); err != nil {
		t.Fatalf("falha ao enviar JoinFederation: %v", err)
	}

	// Aguardar resposta de ingresso (timeout de 2 segundos)
	joinRespHdr, joinRespPayload, err := client.ReadPacketTimeout(2 * time.Second)
	if err != nil {
		t.Logf("Aviso na leitura de resposta do JoinFederation: %v", err)
	} else {
		t.Logf(">>> RESPOSTA RECEBIDA DO PORTICO PARA JoinFederation: Tipo: %s, Call: %s, ReqId: %d, PayloadLen: %d",
			joinRespHdr.MessageType, joinRespHdr.CallType, joinRespHdr.RequestId, len(joinRespPayload))
	}

	time.Sleep(500 * time.Millisecond)
	t.Logf("Sessao finalizada com sucesso.")
}
