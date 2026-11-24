// Projeto Valhalla - Federado Nativo HLA-4 (IEEE 1516-2025)
// Desenvolvedor Líder: Carlos Magno Oliveira de Abreu (@icemagno / magno.mabreu@gmail.com)

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/icemagno/valhalla/pkg/lrc"
)

type FederateCallbackHandler struct {
	lrc.NullFederateAmbassador
	mu             sync.Mutex
	isSynchronized bool
	syncedCond     *sync.Cond
	timeGranted    float64
	timeCond       *sync.Cond
}

func NewCallbackHandler() *FederateCallbackHandler {
	h := &FederateCallbackHandler{}
	h.syncedCond = sync.NewCond(&h.mu)
	h.timeCond = sync.NewCond(&h.mu)
	return h
}

func (h *FederateCallbackHandler) FederationSynchronized(label string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.isSynchronized = true
	log.Printf("[Callback] >>> FEDERACAO TOTALMENTE SINCRONIZADA no ponto '%s'!", label)
	h.syncedCond.Broadcast()
}

func (h *FederateCallbackHandler) DiscoverObjectInstance(theObject int32, theObjectClass int32, objectName string) {
	log.Printf("[Callback] >>> OBJETO DESCOBERTO: handle=%d, class=%d, name='%s'", theObject, theObjectClass, objectName)
}

func (h *FederateCallbackHandler) ReflectAttributeValues(theObject int32, theAttributes map[int32][]byte, tag []byte) {
	log.Printf("[Callback] >>> ATRIBUTOS RECEBIDOS para o objeto %d: %d atributos atualizados", theObject, len(theAttributes))
	for k, v := range theAttributes {
		log.Printf("           -> Attr[%d] = %s", k, string(v))
	}
}

func (h *FederateCallbackHandler) ReceiveInteraction(interactionClass int32, theParameters map[int32][]byte, tag []byte) {
	log.Printf("[Callback] >>> INTERACAO RECEBIDA da classe %d: %d parametros", interactionClass, len(theParameters))
	for k, v := range theParameters {
		log.Printf("           -> Param[%d] = %s", k, string(v))
	}
}

func (h *FederateCallbackHandler) TimeAdvanceGrant(grantedTime float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.timeGranted = grantedTime
	log.Printf("[Callback] >>> TEMPO CONCEDIDO (TAG): %.2f", grantedTime)
	h.timeCond.Broadcast()
}

func main() {
	addr := flag.String("address", "127.0.0.1:52295", "Endereco do Servidor CRC (ip:porta)")
	fedName := flag.String("federation", "ExampleFederation", "Nome da Federacao")
	name := flag.String("name", fmt.Sprintf("Valhalla_%d", os.Getpid()), "Nome deste Federado")
	syncPoint := flag.String("sync", "ReadyToRun", "Ponto de sincronizacao inicial")
	flag.Parse()

	fmt.Println("====================================================================")
	fmt.Println("           PROJETO VALHALLA - FEDERADO NATIVO HLA-4 (LRC)")
	fmt.Println("====================================================================")
	fmt.Printf("Conectando ao CRC em: %s\n", *addr)
	fmt.Printf("Federacao: %s | Federado: %s (Nativo VAL4)\n\n", *fedName, *name)

	// Instanciar LRC e Handler de Callbacks
	handler := NewCallbackHandler()
	amb := lrc.NewRTIambassador()

	// 1. Conexão com o CRC
	log.Printf("[1/5] Conectando ao Central RTI Component (CRC)...")
	if err := amb.Connect(handler, *addr); err != nil {
		log.Fatalf("[-] Falha ao conectar ao CRC: %v", err)
	}
	defer amb.Disconnect()
	log.Printf("[+] Conexao nativa estabelecida com sucesso via preambulo VAL4!")

	// Goroutine em background para evocar callbacks assíncronos do RTI
	stopCallbacks := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopCallbacks:
				return
			default:
				_, _ = amb.EvokeCallback(100 * time.Millisecond)
			}
		}
	}()
	defer close(stopCallbacks)

	// Captura graciosa de Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 2. Ingresso na Federação
	log.Printf("[2/5] Ingressando na federacao '%s'...", *fedName)
	fedHandle, err := amb.JoinFederationExecution(*name, *fedName)
	if err != nil {
		log.Fatalf("[-] Falha ao ingressar na federacao: %v", err)
	}
	log.Printf("[+] Ingresso confirmado! FederateHandle atribuido: %d", fedHandle)

	// 3. Ponto de Sincronização
	if *syncPoint != "" {
		log.Printf("[3/5] Anunciando conquista do ponto de sincronizacao '%s'...", *syncPoint)
		_ = amb.RegisterFederationSynchronizationPoint(*syncPoint, nil)
		if err := amb.SynchronizationPointAchieved(*syncPoint); err != nil {
			log.Printf("[!] Aviso ao conquistar ponto: %v", err)
		} else {
			log.Printf("[+] Ponto '%s' conquistado com sucesso!", *syncPoint)
		}
	}

	// 4. Registro de Objeto FOM (Exemplo: Aircraft / Node 561)
	log.Printf("[4/5] Registrando objeto na simulacao...")
	objHandle, err := amb.RegisterObjectInstance(561, fmt.Sprintf("%s_Obj", *name))
	if err != nil {
		log.Printf("[!] Erro registrando objeto: %v", err)
	} else {
		log.Printf("[+] Objeto registrado com sucesso! ObjectHandle: %d", objHandle)
	}

	// 5. Loop de Simulação
	log.Printf("[5/5] Federado pronto e ativo no ciclo de simulacao!")
	fmt.Println(">>> Pressione Ctrl+C para encerrar o federado graciosamente <<<")

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	step := 1
	for {
		select {
		case <-sigChan:
			fmt.Println("\n[!] Sinal de encerramento recebido. Desingressando da federacao...")
			if err := amb.ResignFederationExecution(); err != nil {
				log.Printf("[-] Erro no resign: %v", err)
			} else {
				log.Printf("[+] Desingresso concluido com sucesso!")
			}
			return

		case <-ticker.C:
			// Publicar atualizacao de atributos
			if objHandle > 0 {
				x := 100.0 + float64(step)*10.5
				y := 200.0 + float64(step)*5.2
				attrs := map[int32][]byte{
					562: []byte(fmt.Sprintf("X:%.2f", x)),
					551: []byte(fmt.Sprintf("Y:%.2f", y)),
				}
				_ = amb.UpdateAttributeValues(objHandle, attrs, nil)
				log.Printf("[Simulacao #%d] Atributos do objeto %d atualizados (X=%.2f, Y=%.2f)", step, objHandle, x, y)
			}

			// Disparar interacao
			_ = amb.SendInteraction(525, map[int32][]byte{1: []byte(fmt.Sprintf("HeartbeatStep_%d", step))}, nil)

			// Solicitar avanço temporal
			logicalTime := float64(step) * 1.0
			_ = amb.TimeAdvanceRequest(logicalTime)

			step++
		}
	}
}
