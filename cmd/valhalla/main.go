package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/icemagno/valhalla/pkg/crc/portico/transport"
	"github.com/icemagno/valhalla/pkg/crc/portico/wire"
)

func main() {
	host := flag.String("host", "127.0.0.1", "Endereco IP do executivo Portico CRTI")
	port := flag.Int("port", 52295, "Porta TCP do executivo Portico CRTI")
	fedName := flag.String("fed", "ExampleFederation", "Nome da federacao alvo")
	fedType := flag.String("type", "ieee1516e", "Tipo do federado (ex: ieee1516e)")
	defaultName := fmt.Sprintf("Valhalla_%d", time.Now().UnixNano()%100000)
	nodeName := flag.String("name", defaultName, "Nome deste federado")
	syncPoint := flag.String("sync", "ReadyToRun", "Ponto de sincronizacao inicial")
	flag.Parse()

	addr := fmt.Sprintf("%s:%d", *host, *port)
	fmt.Printf("====================================================================\n")
	fmt.Printf("           PROJETO VALHALLA - FEDERADO HLA (IEEE 1516e)            \n")
	fmt.Printf("====================================================================\n")
	fmt.Printf("Conectando ao Portico CRTI em: %s\n", addr)
	fmt.Printf("Federacao: %s | Federado: %s (%s)\n\n", *fedName, *nodeName, *fedType)

	client := transport.NewTCPClient(addr)

	// 1. Conexao e Handshake TCP (0xBEEF / 0xFEED)
	fmt.Printf("[1/4] Realizando Conexao e Handshake de Transporte TCP...\n")
	if err := client.Connect(5 * time.Second); err != nil {
		fmt.Printf("[-] Falha ao conectar ao CRTI: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[+] Conexao estabelecida: %s\n", client.WelcomeMessage())

	var assignedFederateHandle int32 = -1
	var assignedFederationHandle int32 = -1

	// Interceptar Ctrl+C para saida graciosa (Resign)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	cleanup := func() {
		fmt.Printf("\n[!] Solicitando desligamento gracioso (ResignFederation)...\n")
		resignObj := wire.NewResignFederationObject(*fedName, *nodeName, *fedType, "NO_ACTION")
		if assignedFederateHandle > 0 {
			resignObj.Set("sourceFederate", assignedFederateHandle)
		}
		resignPayload, err := wire.SerializePorticoControlMessage(resignObj)
		if err == nil {
			srcHandle := uint16(1)
			if assignedFederateHandle > 0 {
				srcHandle = uint16(assignedFederateHandle)
			}
			resignHdr := wire.NewControlRequest(wire.MsgResignFederation, 999, srcHandle, 0, uint32(len(resignPayload)))
			resignHdr.SourceHandle = srcHandle
			_ = client.SendPacket(resignHdr, resignPayload)
			respHdr, _, err := client.ReadPacketTimeout(2 * time.Second)
			if err == nil {
				fmt.Printf("[+] Resign confirmado pelo CRTI: %s / %s\n", respHdr.MessageType, respHdr.CallType)
			}
		}
		client.Close()
		fmt.Printf("[+] Conexao encerrada. Sessao Valhalla finalizada.\n")
	}

	// 2. Enviar JoinFederation
	fmt.Printf("\n[2/4] Enviando JoinFederation na %s...\n", *fedName)
	joinObj := wire.NewJoinFederationObject(*fedName, *nodeName, *fedType)
	joinPayload, err := wire.SerializePorticoControlMessage(joinObj)
	if err != nil {
		fmt.Printf("[-] Erro ao serializar JoinFederation: %v\n", err)
		client.Close()
		os.Exit(1)
	}

	joinHdr := wire.NewControlRequest(wire.MsgJoinFederation, 1, 0, 0, uint32(len(joinPayload)))
	if err := client.SendPacket(joinHdr, joinPayload); err != nil {
		fmt.Printf("[-] Erro ao enviar JoinFederation: %v\n", err)
		client.Close()
		os.Exit(1)
	}

	joinRespHdr, joinRespPayload, err := client.ReadPacketTimeout(5 * time.Second)
	if err != nil {
		fmt.Printf("[-] Erro aguardando WelcomePack: %v\n", err)
		client.Close()
		os.Exit(1)
	}

	if joinRespHdr.CallType != wire.CallTypeControlResponseOK {
		errMsg := wire.ExtractErrorMessage(joinRespPayload)
		fmt.Printf("[-] CRTI rejeitou ingresso: %s\n", joinRespHdr.CallType)
		fmt.Printf("[-] Detalhes do Portico: %s\n", errMsg)
		client.Close()
		os.Exit(1)
	}

	fmt.Printf("[+] INGRESSO BEM-SUCEDIDO! WelcomePack recebido (%d bytes).\n", len(joinRespPayload))

	wpInfo, err := wire.ParseWelcomePack(joinRespPayload)
	if err != nil {
		fmt.Printf("[!] Aviso ao processar WelcomePack: %v\n", err)
	} else {
		assignedFederationHandle = wpInfo.FederationHandle
		assignedFederateHandle = wpInfo.FederateHandle
		fmt.Printf("[+] WelcomePack validado: FedHandle=%d, FederateHandle=%d (%s)\n",
			assignedFederationHandle, assignedFederateHandle, wpInfo.FederateType)
	}

	// 3. Enviar SyncPointAchieved
	if *syncPoint != "" {
		srcHandle := assignedFederateHandle
		if srcHandle <= 0 {
			srcHandle = 1
		}
		targetFed := assignedFederationHandle
		if targetFed <= 0 {
			targetFed = 1
		}
		fmt.Printf("\n[3/4] Anunciando conquista do ponto de sincronizacao '%s' (Handle %d)...\n", *syncPoint, srcHandle)
		syncObj := wire.NewSyncPointAchievedObject(*syncPoint, srcHandle)
		syncObj.Set("targetFederation", targetFed)
		syncObj.Set("sourceFederate", srcHandle)

		syncPayload, err := wire.SerializePorticoControlMessage(syncObj)
		if err != nil {
			fmt.Printf("[-] Erro ao serializar SyncPointAchieved: %v\n", err)
			cleanup()
			os.Exit(1)
		}

		syncHdr := wire.NewControlRequest(wire.MsgAchieveSyncPoint, 2, uint16(srcHandle), 0, uint32(len(syncPayload)))
		syncHdr.SourceHandle = uint16(srcHandle)
		if err := client.SendPacket(syncHdr, syncPayload); err != nil {
			fmt.Printf("[-] Erro ao enviar SyncPointAchieved: %v\n", err)
			cleanup()
			os.Exit(1)
		}

		syncRespHdr, _, err := client.ReadPacketTimeout(3 * time.Second)
		if err != nil {
			fmt.Printf("[-] Erro aguardando resposta do SyncPoint: %v\n", err)
		} else if syncRespHdr.CallType == wire.CallTypeControlResponseOK {
			fmt.Printf("[+] Ponto de sincronizacao '%s' CONQUISTADO com sucesso!\n", *syncPoint)
		} else {
			fmt.Printf("[-] CRTI retornou aviso no SyncPoint: %s\n", syncRespHdr.CallType)
		}
	}

	// 4. Loop de escuta de notificacoes da simulacao
	fmt.Printf("\n[4/4] Valhalla ativo na federacao! Aguardando o Thor iniciar...\n")
	fmt.Printf(">>> PRESSIONE ENTER NO TERMINAL DO THOR PARA SINCRONIZAR A FEDERACAO! <<<\n")
	fmt.Printf("(Pressione Ctrl+C a qualquer momento para sair graciosamente)\n\n")

	done := make(chan struct{})

	go func() {
		for {
			hdr, payload, err := client.ReadPacketTimeout(1 * time.Second)
			if err != nil {
				var netErr net.Error
				if errors.As(err, &netErr) && netErr.Timeout() {
					// Timeout normal do polling periodico
					select {
					case <-done:
						return
					default:
						continue
					}
				}

				// Socket fechado pelo servidor, EOF ou ECONNRESET
				select {
				case <-done:
					return
				default:
					fmt.Printf("\n[-] Conexao com o Portico CRTI foi perdida ou encerrada pelo servidor: %v\n", err)
					os.Exit(1)
				}
			}

			fmt.Printf("[PACOTE RECEBIDO] Msg: %-22s | Call: %-18s | Len: %5d bytes\n",
				hdr.MessageType, hdr.CallType, len(payload))

			switch hdr.MessageType {
			case wire.MsgSyncPointFederation:
				fmt.Printf(">>> FEDERACAO SINCRONIZADA! Todos os federados alcancaram o ponto!\n")
			case wire.MsgDiscoverObject:
				fmt.Printf(">>> NOVO OBJETO DESCOBERTO na simulacao (Thor registrou objeto)!\n")
			case wire.MsgReflectAttributes:
				fmt.Printf(">>> ATRIBUTOS ATUALIZADOS recebidos da simulacao!\n")
			case wire.MsgReceiveInteraction:
				fmt.Printf(">>> INTERACAO RECEBIDA da simulacao!\n")
			case wire.MsgTimeAdvanceGrant:
				fmt.Printf(">>> AVANCO TEMPORAL CONCEDIDO (TAG)!\n")
			}
		}
	}()

	select {
	case <-sigChan:
		close(done)
		cleanup()
	}
}
