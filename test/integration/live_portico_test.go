package integration

import (
	"fmt"
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

	// 1. Enviar JoinFederation usando o motor serializador JOS generico com nome dinamico unico
	fedName := "ExampleFederation"
	nodeName := fmt.Sprintf("Valhalla_%d", time.Now().UnixNano()%1000000)
	fedType := "ExampleFederateType"

	joinObj := wire.NewJoinFederationObject(fedName, nodeName, fedType)
	joinPayload, err := wire.SerializePorticoControlMessage(joinObj)
	if err != nil {
		t.Fatalf("falha ao serializar JoinFederation com motor generico: %v", err)
	}

	joinHdr := wire.NewControlRequest(wire.MsgJoinFederation, 1, 0, 0, uint32(len(joinPayload)))

	t.Logf("Enviando JoinFederation generico na %s como %s (payload %d bytes)...", fedName, nodeName, len(joinPayload))
	if err := client.SendPacket(joinHdr, joinPayload); err != nil {
		t.Fatalf("falha ao enviar JoinFederation: %v", err)
	}

	// Aguardar resposta oficial de ingresso (WelcomePack) do Portico
	joinRespHdr, joinRespPayload, err := client.ReadPacketTimeout(3 * time.Second)
	if err != nil {
		t.Fatalf("falha ao receber resposta de JoinFederation do Portico: %v", err)
	}

	t.Logf(">>> RESPOSTA RECEBIDA DO PORTICO PARA JoinFederation:")
	t.Logf("    Tipo Mensagem : %s", joinRespHdr.MessageType)
	t.Logf("    Tipo Chamada  : %s", joinRespHdr.CallType)
	t.Logf("    Request ID    : %d", joinRespHdr.RequestId)
	t.Logf("    Source Handle : %d", joinRespHdr.SourceHandle)
	t.Logf("    Target Handle : %d", joinRespHdr.TargetHandle)
	t.Logf("    Tamanho Pacote: %d bytes (WelcomePack oficial)", len(joinRespPayload))
	if len(joinRespPayload) > 64 {
		t.Logf("    Hex Inicial   : %X", joinRespPayload[:64])
	}

	if joinRespHdr.CallType != wire.CallTypeControlResponseOK {
		t.Fatalf("CRTI retornou erro na resposta: %s", joinRespHdr.CallType)
	}

	federateHandle := joinRespHdr.TargetHandle
	t.Logf("Federado %s ingressado com sucesso absoluto na federação viva (Handle %d)!", nodeName, federateHandle)

	// 2. Anunciar que alcançou o ponto de sincronização ReadyToRun
	// targetFederation deve ser 1 (ExampleFederation)
	syncObj := wire.NewSyncPointAchievedObject("ReadyToRun", 3)
	syncObj.Set("targetFederation", int32(1))
	syncObj.Set("sourceFederate", int32(3))

	syncPayload, err := wire.SerializePorticoControlMessage(syncObj)
	if err != nil {
		t.Fatalf("falha ao serializar SyncPointAchieved: %v", err)
	}

	syncHdr := wire.NewControlRequest(wire.MsgAchieveSyncPoint, 2, 1, 0, uint32(len(syncPayload)))
	syncHdr.SourceHandle = 3
	if err := client.SendPacket(syncHdr, syncPayload); err != nil {
		t.Fatalf("falha ao enviar SyncPointAchieved: %v", err)
	}
	t.Logf("SyncPointAchieved ('ReadyToRun') enviado com sucesso pelo federado %s (Federation 1).", nodeName)

	// Aguardar e processar a resposta oficial de SyncPointAchieved do CRTI
	syncRespHdr, syncRespPayload, err := client.ReadPacketTimeout(3 * time.Second)
	if err != nil {
		t.Logf("Aviso aguardando resposta de SyncPointAchieved: %v", err)
	} else {
		t.Logf(">>> RESPOSTA RECEBIDA DO PORTICO PARA SyncPointAchieved:")
		t.Logf("    Tipo Mensagem : %s", syncRespHdr.MessageType)
		t.Logf("    Tipo Chamada  : %s", syncRespHdr.CallType)
		t.Logf("    Request ID    : %d", syncRespHdr.RequestId)
		t.Logf("    Payload Bytes : %s", string(syncRespPayload))
	}

	// 3. Enviar ResignFederation canônico para saida graciosa e liberar o federado no CRTI
	resignObj := wire.NewResignFederationObject(fedName, nodeName, "ieee1516e", "NO_ACTION")
	resignPayload, err := wire.SerializePorticoControlMessage(resignObj)
	if err != nil {
		t.Fatalf("falha ao serializar ResignFederation: %v", err)
	}

	resignHdr := wire.NewControlRequest(wire.MsgResignFederation, 3, 0, 0, uint32(len(resignPayload)))
	resignHdr.SourceHandle = federateHandle
	if err := client.SendPacket(resignHdr, resignPayload); err != nil {
		t.Fatalf("falha ao enviar ResignFederation: %v", err)
	}
	t.Logf("ResignFederation enviado para saida graciosa do federado %s.", nodeName)

	// Aguardar e processar a resposta oficial de desligamento do CRTI
	resignRespHdr, _, err := client.ReadPacketTimeout(3 * time.Second)
	if err != nil {
		t.Logf("Aviso aguardando resposta do Resign: %v", err)
	} else {
		t.Logf(">>> RESPOSTA RECEBIDA DO PORTICO PARA ResignFederation:")
		t.Logf("    Tipo Mensagem : %s", resignRespHdr.MessageType)
		t.Logf("    Tipo Chamada  : %s", resignRespHdr.CallType)
	}

	time.Sleep(200 * time.Millisecond)
	t.Logf("Sessao finalizada com sucesso.")
}
