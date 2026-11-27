package integration

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/icemagno/valhalla/pkg/crc/portico/transport"
	"github.com/icemagno/valhalla/pkg/crc/portico/wire"
)

func TestIntegration_LivePortico_HandshakeAndProbe(t *testing.T) {
	endpoint := os.Getenv("VALHALLA_LIVE_PORTICO_ADDR")
	if endpoint == "" {
		endpoint = os.Getenv("PORTICO_ENDPOINT")
	}
	if os.Getenv("PORTICO_LIVE_TEST") != "true" || endpoint == "" {
		t.Skip("Skipping live Portico test (set PORTICO_LIVE_TEST=true and VALHALLA_LIVE_PORTICO_ADDR=host:port to run)")
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

	t.Logf("Federado %s ingressou com sucesso (WelcomePack recebido, TargetHandle %d)!", nodeName, joinRespHdr.TargetHandle)

	// Inspecionar o objeto deserializado do WelcomePack
	wpObj, wpErr := wire.DeserializePorticoControlMessage(joinRespPayload)
	if wpErr != nil {
		t.Fatalf("falha ao deserializar WelcomePack: %v", wpErr)
	}
	t.Logf("WelcomePack Class: %s (Flags: 0x%02X)", wpObj.Class.Name, wpObj.Class.Flags)
	if idxWP := bytes.Index(joinRespPayload, []byte("WelcomePack")); idxWP != -1 {
		end := idxWP + 80
		if end > len(joinRespPayload) {
			end = len(joinRespPayload)
		}
		t.Logf("WelcomePack at index %d. Next %d bytes HEX: %X", idxWP, end-idxWP, joinRespPayload[idxWP:end])
	}

	wpInfo, err := wire.ParseWelcomePack(joinRespPayload)
	if err != nil {
		t.Fatalf("falha ao analisar WelcomePack: %v", err)
	}
	t.Logf("Federado %s ingressado com sucesso absoluto! FedHandle=%d, FederateHandle=%d (%s)",
		wpInfo.FederateName, wpInfo.FederationHandle, wpInfo.FederateHandle, wpInfo.FederateType)

	// 2. Anunciar que alcançou o ponto de sincronização ReadyToRun com o handle REAL
	syncObj := wire.NewSyncPointAchievedObject("ReadyToRun", wpInfo.FederateHandle)
	syncObj.Set("targetFederation", wpInfo.FederationHandle)
	syncObj.Set("sourceFederate", wpInfo.FederateHandle)

	syncPayload, err := wire.SerializePorticoControlMessage(syncObj)
	if err != nil {
		t.Fatalf("falha ao serializar SyncPointAchieved: %v", err)
	}

	syncHdr := wire.NewControlRequest(wire.MsgAchieveSyncPoint, 2, uint16(wpInfo.FederateHandle), 0, uint32(len(syncPayload)))
	syncHdr.SourceHandle = uint16(wpInfo.FederateHandle)
	if err := client.SendPacket(syncHdr, syncPayload); err != nil {
		t.Fatalf("falha ao enviar SyncPointAchieved: %v", err)
	}
	t.Logf("SyncPointAchieved ('ReadyToRun') enviado com sucesso pelo federado %s (Handle %d).", nodeName, wpInfo.FederateHandle)

	// Aguardar e processar a resposta oficial de SyncPointAchieved do CRTI
	syncRespHdr, syncRespPayload, err := client.ReadPacketTimeout(3 * time.Second)
	if err != nil {
		t.Fatalf("falha aguardando resposta de SyncPointAchieved: %v", err)
	}
	t.Logf(">>> RESPOSTA RECEBIDA DO PORTICO PARA SyncPointAchieved: %s / %s (req=%d, %d bytes)",
		syncRespHdr.MessageType, syncRespHdr.CallType, syncRespHdr.RequestId, len(syncRespPayload))

	// 3. Enviar ResignFederation canônico para saida graciosa e liberar o federado no CRTI
	resignObj := wire.NewResignFederationObject(fedName, nodeName, "ieee1516e", "NO_ACTION")
	resignPayload, err := wire.SerializePorticoControlMessage(resignObj)
	if err != nil {
		t.Fatalf("falha ao serializar ResignFederation: %v", err)
	}

	resignHdr := wire.NewControlRequest(wire.MsgResignFederation, 3, uint16(wpInfo.FederateHandle), 0, uint32(len(resignPayload)))
	resignHdr.SourceHandle = uint16(wpInfo.FederateHandle)
	if err := client.SendPacket(resignHdr, resignPayload); err != nil {
		t.Fatalf("falha ao enviar ResignFederation: %v", err)
	}
	t.Logf("ResignFederation enviado para saida graciosa do federado %s.", nodeName)

	// Aguardar e processar a resposta oficial de desligamento do CRTI
	resignRespHdr, _, err := client.ReadPacketTimeout(3 * time.Second)
	if err != nil {
		t.Fatalf("falha aguardando resposta do Resign: %v", err)
	}
	t.Logf(">>> RESPOSTA RECEBIDA DO PORTICO PARA ResignFederation: %s / %s", resignRespHdr.MessageType, resignRespHdr.CallType)

	time.Sleep(200 * time.Millisecond)
	t.Logf("Sessao finalizada com sucesso.")
}
