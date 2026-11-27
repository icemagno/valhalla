# Relatório de Auditoria Técnica e Arquitetural: Projeto Valhalla (HLA-RTI)

> **Data da Auditoria:** 26 de Novembro de 2026  
> **Escopo da Análise:** Engenharia de Software, Arquitetura de Sistemas Distribuídos, Padrões de Projeto, Desempenho, Concorrência, Tratamento de Erros e Verificação de Conformidade Documental/Normativa.  
> **Status:** Concluído com Apontamentos Críticos

---

## Sumário Executivo

A presente auditoria foi executada sobre a totalidade do código-fonte do **Projeto Valhalla** (Go, C-ABI via CGo, Java SDK, C++ SDK, Web/TypeScript, documentação técnica e suítes de teste). O objetivo foi identificar **anti-padrões**, **códigos mortos**, **erros não tratados**, **stubs/mocks ocultos**, **inconsistências/afirmações inverídicas na documentação (vaporware)**, **violações de padrões de projeto e delegação de responsabilidade**, **classes/funções 'God'** e **gargalos potenciais de desempenho**.

O projeto demonstra engenhosidade na concepção de estruturas de dados e no parsing de serialização Java limpa (*Clean-Room JOS*). Contudo, a auditoria revelou **discrepâncias severas entre as alegações da documentação oficial e a realidade do código**, presença de **stubs vazios em interfaces de produção**, **falhas estruturais de corrupção de dados no despacho cruzado entre dialetos** e **gargalos de concorrência e alocação** que comprometem a escalabilidade e confiabilidade do sistema.

---

## 1. Enganações, Mentiras e "Vaporware" (Documentação vs. Código Real)

### 🔴 1.1. Post-Quantum Cryptography (ML-KEM-768 / Kyber / HPKE / AES-256-GCM) — Inexistente
* **Alegações em Documentação ([README.md](README.md#L9), [TECHNICAL-MEMORY.md](TECHNICAL-MEMORY.md#L59-L66)):**
  > *"Handshake key establishment powered by ML-KEM-768 (Kyber / FIPS 203) via HPKE (RFC 9180), followed by hardware-accelerated AES-256-GCM (AES-NI) for high-frequency coordinate streaming with negligible overhead (< 2 µs per packet)."*
* **Realidade no Código:**
  Em todo o repositório, existem apenas menções puramente nominais/decorativas:
  - Constante `FlagNativePQC uint8 = 0x04` em [`pkg/common/protocol.go`](pkg/common/protocol.go#L100).
  - Um comentário em [`pkg/common/handles.go`](pkg/common/handles.go#L44).
  - **Não há imports de bibliotecas de criptografia pós-quântica**, nem implementação de HPKE (RFC 9180), nem geração de chaves Kyber/ML-KEM, nem cifra simétrica AES-256-GCM.
  - Toda a comunicação de rede trafega em **texto plano (*plaintext*)**.
  - A constante `FlagNativePQC` **nunca é avaliada ou verificada** em nenhuma parte do CRC ou do LRC.

### 🔴 1.2. Falsa "Serialização SSZ" (Simple Serialize do Ethereum 2.0)
* **Alegações em Documentação ([README.md](README.md#L42), [TECHNICAL-MEMORY.md](TECHNICAL-MEMORY.md#L53-L58)):**
  > *"Adoption of Simple Serialize (SSZ from Ethereum 2.0 consensus layer) replacing manual IEEE Encoding Helpers with contiguous linear binary layouts supporting zero-copy parsing."*
* **Realidade no Código:**
  A implementação em [`pkg/common/payloads.go`](pkg/common/payloads.go#L26-L49) é uma **serialização binária proprietária ad-hoc**, usando `binary.BigEndian` com prefixos de comprimento de 2 bytes (`uint16`). A norma canônica do SSZ (Ethereum 2.0) estipula inteiros em *Little-Endian*, offsets fixos de 4 bytes na frente para tipos variáveis/strings e heap dinâmica ao final. O protocolo **não é SSZ**.

> ✅ **STATUS DE REMEDIAÇÃO (26/11/2026): CORRIGIDO E APROVADO** (commits `552a97c`, `ba6a8a8`, `bc1981a`)  
> Implementado SSZ canônico em `pkg/common/ssz.go` e `pkg/common/payloads.go` com Little-Endian unificado (incluindo `NativeHeader`, `RegisterObject` e `merkle_tree.go`), `List[Container]` determinístico com chaves ordenadas, validação estrita de booleanos (`0x00`/`0x01`), rejeição de trailing bytes e de `firstOffset == 0`, proteções anti-DoS (OOM e overflow `uint32`) e merkleização SSZ esparsa com *zero-hashes* pré-computados ($O(N + \log_2(\text{limit}))$). Validado com testes adversariais, benchmarks e `go test -race`.

### 🔴 1.3. Árvores Merkle e Auditoria Criptográfica (MLS / Inclusão) — Código Inoperante
* **Alegações em Documentação ([README.md](README.md#L42), [TECHNICAL-MEMORY.md](TECHNICAL-MEMORY.md#L55-L58)):**
  > *"Instant Desync Detection: federates compare the 32-byte Merkle root hash each cycle; any 1-bit state divergence is flagged immediately. Multi-Level Security (MLS) via compact Merkle Inclusion Proofs."*
* **Realidade no Código:**
  - O cliente ([`pkg/lrc/client.go`](pkg/lrc/client.go#L143-L180)) **descarta e ignora solenemente** o campo `MerkleRoot` de 32 bytes do cabeçalho `NativeHeader`. O LRC nunca compara o hash de estado nem verifica desync.
  - A função `GenerateProof` em [`pkg/crc/core/merkle_tree.go`](pkg/crc/core/merkle_tree.go#L178) **só é chamada no teste unitário**. Não há mensagem na rede para solicitar ou validar provas Merkle, nem qualquer exposição na C-ABI ou nos SDKs Java, C++ e Web.

> ✅ **STATUS DE REMEDIAÇÃO (26/11/2026): CORRIGIDO E APROVADO** (commit `ca8e127`)  
> Implementada a validação ponta-a-ponta de integridade e detecção de desync no LRC (`pkg/lrc/client.go` e `pkg/lrc/rti_ambassador.go`): árvore local de estado mantida no federado (`common.FederationStateTree`) com sincronização do ciclo de vida completo (registro, descoberta, atualização, atualização espacial e remoção) e comparada a cada reflexão e atualização de atributos contra o `MerkleRoot` de 32 bytes recebido no `NativeHeader`, com notificação imediata via `DesyncAwareFederateAmbassador` e `LastDesyncDetected()`. Implementado o protocolo de rede para requisição (`NativeMsgGetMerkleProof`) e resposta (`NativeMsgGetMerkleProofResp`) de provas criptográficas de inclusão SSZ, validadas matematicamente no cliente contra a raiz de estado confiável (`expectedRoot`) com checagem estrita de convergência de caminho (`idx == 0`) e limite de profundidade (<= 64 chunks) via `VerifyMerkleProof()`. Coberto por testes adversariais contra falsos positivos, forjamento de provas e corrupção de estado em `test/integration/merkle_desync_test.go` e `go test -race`.

### 🔴 1.4. DDM 3D com Curvas de Morton — Falsa Filtragem Bitwise
* **Alegações em Documentação ([README.md](README.md#L43), [TECHNICAL-MEMORY.md](TECHNICAL-MEMORY.md#L281-L285)):**
  > *"3D Morton Space-Filling Curves mapping ECEF coordinates into 64-bit Morton codes for bitwise spatial containment checks in < 2 ns bypassing hash tables and pointer chases."*
* **Realidade no Código:**
  O método de teste de contenção espacial em [`pkg/crc/core/morton.go`](pkg/crc/core/morton.go#L130-L134):
  ```go
  func (r SpatialRegion) ContainsMorton(code uint64) bool {
      x, y, z := DecodeMorton3D(code)
      return r.ContainsPoint(x, y, z)
  }
  ```
  Ele **decodifica o código Morton de volta para $(X, Y, Z)$** através de operações de deslocamento e realiza a comparação geométrica tradicional de bounding box (`MinX <= x && x <= MaxX`). Não existe busca por intervalos Z-order, nem contenção bitwise na curva. A conversão para Morton é um overhead inútil no caminho de execução.

> ✅ **STATUS DE REMEDIAÇÃO (26/11/2026): CORRIGIDO E APROVADO** (commit `a7db266`)  
> Refatorado `ContainsMorton` em `pkg/crc/core/morton.go` e integrado ao ciclo de vida em `pkg/crc/models.go` para operar diretamente no espaço Morton sem decodificação (`Compact1By2` eliminado), utilizando limites dilatados pré-computados (`Part1By2` monotônico) e rejeição rápida por prefixo comum de octree (`PrefixMask`). Validado com 200.000+ pontos em `TestMortonBitwiseEquivalence`, 0 alocações e `go test -race`.

### 🔴 1.5. "High-speed UDP Framing" — Inexistente
* **Alegações em Documentação ([README.md](README.md#L38)):**
  > *"Modern federates stream via zero-copy SSZ and high-speed UDP framing."*
* **Realidade no Código:**
  **Não existe uma única linha de código UDP** em todo o repositório. Toda a comunicação do servidor e dos clientes opera exclusivamente sobre conexões TCP e WebSockets.

> ✅ **STATUS DE REMEDIAÇÃO (27/11/2026): CORRIGIDO E APROVADO** (commit `292be6c`)  
> Todas as alegações de UDP framing em `README.md:38` e `docs/architectural-deliberations.md:282` foram corrigidas para refletir estritamente a realidade atual do código (TCP de alta vazão com preâmbulo VAL4 e cabeçalho linear de 56 bytes + WebSockets IEEE 1516-2025). O suporte a streaming não-confiável via UDP framing foi formal e explicitamente reclassificado como proposta futura de Roadmap.

### 🔴 1.6. Distorções em Benchmarks e "P50 < 1 µs" Artificial
* **Alegações em Documentação ([BENCHMARK.md](BENCHMARK.md#L40-L45), [README.md](README.md#L58-L66)):**
  - Apresenta tabelas comparando Valhalla contra *Pitch pRTI 5.4*, *MÄK RTI 4.5*, *CERTI* e *Portico* com precisão de frações de nanosegundos (ex.: "OMT Class Lookup: 1.14 ns vs 48.20 ns no Pitch"). Produtos proprietários comerciais em C++ não disponibilizam microbenchmarks isolados de chamadas internas em nanosegundos.
  - No benchmark interno em [`test/benchmark/end_to_end_benchmark_test.go`](test/benchmark/end_to_end_benchmark_test.go#L117-L118):
    ```go
    elapsed := time.Duration(time.Now().UnixNano() - sentNano)
    ```
    No ambiente Windows, a resolução do temporizador do sistema possui granularidade de milissegundos (~0,5 a 1 ms). Iterações executadas dentro da mesma fatia de clock geram diferença `0 s` (`0.00 µs`), que foi documentada de forma enganosa como *"P50 (Median) : < 1 µs (microsecond resolution limit)"*.
  - A decomposição de overhead da Lei de Amdahl (0.18 µs de marshaling, 0.28 µs de bitset router) foi arbitrariamente decomposta no texto, sem instrumentação contínua de amostragem.

> ✅ **STATUS DE REMEDIAÇÃO (27/11/2026): CORRIGIDO E APROVADO** (commits `292be6c`, `3edf7b6`)  
> Removidas as comparações fabricadas em nanosegundos contra softwares comerciais proprietários fechados (Pitch pRTI, MÄK RTI) do `README.md`, reestruturando a apresentação em duas camadas estritamente separadas: Camada 1 para Rede Real Fim-a-Fim (com baselines acadêmicos publicados de Gütlein et al., WSC 2020) e Camada 2 para Microbenchmarks Algorítmicos em Memória do Go (`testing.B`). A decomposição da Lei de Amdahl em `BENCHMARK.md` e `docs/benchmark.md` foi formalmente classificada como modelo de referência arquitetural teórico. No benchmark de rede em `test/benchmark/end_to_end_benchmark_test.go`, a quantização grosseira do relógio de interrupção do Windows que causava o falso "P50 < 1 µs" (leitura de 0s) foi eliminada através de temporizadores de alta precisão divididos por build tags: `timer_windows_test.go` (`//go:build windows`) com `QueryPerformanceCounter` (resolução de hardware de 100 ns) e `timer_other_test.go` (`//go:build !windows`) com `CLOCK_MONOTONIC`. Validado empiricamente no Windows com P50 real de ~72–81 µs (sem artefatos de 0 µs), compilação cruzada 100% limpa em Linux (`GOOS=linux`) e macOS (`GOOS=darwin`), e `go test -race ./...` com 0 data races.

### 🔴 1.7. Ingestão Incompleta do RPR-FOM v2.0
* **Alegações em Documentação ([README.md](README.md#L49-L51), [TECHNICAL-MEMORY.md](TECHNICAL-MEMORY.md#L92-L96)):**
  > *"Full end-to-end integration and schema ingestion validated against official RPR-FOM v2.0 (SISO-STD-001-2015) schemas across all 14 modules."*
* **Realidade no Código:**
  O teste [`test/resources_test.go`](test/resources_test.go#L52-L60) não ingere o metamodelo nem instancia classes; ele apenas realiza um `xml.Unmarshal` básico verificando se o nó raiz é `<objectModel>`. Os testes do parser real ([`pkg/metamodel/xml_parser_test.go`](pkg/metamodel/xml_parser_test.go#L80)) ingerem somente 2 dos 14 módulos (`RPR-Base` e `RPR-Physical`).

> ✅ **STATUS DE REMEDIAÇÃO (27/11/2026): CORRIGIDO E APROVADO** (commit `4c96732`)  
> Implementada suíte completa de ingestão e compilação ponta-a-ponta de todos os 14 módulos oficiais do RPR-FOM v2.0 (SISO-STD-001-2015) em `pkg/metamodel/xml_parser_test.go` (`TestXMLParser_All14_RPR_FOM_Modules`) e `test/resources_test.go` (`TestResources_RPR_FOM_FullMetamodelIngestion`). O parser OMT compilou incrementalmente os 14 módulos (Foundation, Enumerations, Base, Physical, Aggregate, Communication, DER, Logistics, Minefield, SE, SIMAN, Switches, UA e Warfare) em um único `core.OMTRegistry` consistente, gerando 128 intervalos de Euler Tour para classes de objetos e 158 para classes de interação, comprovando herança $O(1)$ entre módulos (`Aircraft` $\to$ `Platform` $\to$ `PhysicalEntity` $\to$ `BaseEntity` $\to$ `HLAobjectRoot`). Validada a instanciação ponta-a-ponta de federação tática real (`RPR_Tactical_Simulation`) via `crc.FederationManager.CreateFederationWithFOMs` carregando a totalidade dos módulos em disco através do `FOM-Service` com resolução de caminhos multinível em `ResolveFOMPath`. Validado com 100% PASS sob `go test -race ./...` e 0 data races.

### 🔴 1.8. Estrutura de Diretórios Fictícia no README
* O [README.md](README.md#L87-L91) indica os pacotes `pkg/euler/`, `pkg/fom/`, `pkg/morton/`, `pkg/vector/`, `pkg/ssz/` e `pkg/crypto/`. Nenhum deles existe na raiz de `pkg/` (foram fundidos em `pkg/crc/core` ou não existem).

> ✅ **STATUS DE REMEDIAÇÃO (27/11/2026): CORRIGIDO E APROVADO** (commit `b6bacf0`)  
> Substituída a árvore fictícia em `README.md` pela estrutura real e canônica do repositório, removendo todos os diretórios inexistentes (`pkg/euler/`, `pkg/fom/`, `pkg/morton/`, `pkg/vector/`) e eliminando a menção indevida a PQC em `pkg/common/`. Conferido 100% contra o sistema de arquivos real.

---

## 2. Stubs e Mocks Silenciosos no Código de Produção

### ⚠️ 2.1. Callbacks Fantasmas na C-ABI ([pkg/cabi/cabi.go](pkg/cabi/cabi.go#L78-L82), [L160-L161](pkg/cabi/cabi.go#L160-L161))
Na camada C-ABI via CGo utilizada pela DLL/SO nativa e consumida pelos SDKs C, C++ e Java:
```go
func (f *cFedAmbassador) SynchronizationPointRegistrationSucceeded(label string)              {}
func (f *cFedAmbassador) SynchronizationPointRegistrationFailed(label string, reason string) {}
func (f *cFedAmbassador) AnnounceSynchronizationPoint(label string, tag []byte)              {}
func (f *cFedAmbassador) FederationSynchronized(label string)                                {}
func (f *cFedAmbassador) ReceiveInteraction(interactionClass int32, theParameters map[int32][]byte, tag []byte) {}
```
**Impacto Grave:** Qualquer aplicação desenvolvida em C, C++ ou Java que aguarde o recebimento de uma **Interação HLA** (`receiveInteraction`) ou **anúncio de Ponto de Sincronização** terá o evento descartado em silêncio. As funções são *stubs* vazios.

> ✅ **STATUS DE REMEDIAÇÃO (26/11/2026): CORRIGIDO E APROVADO** (commits `1b21db2`, `fc1969b`)  
> Implementados todos os callbacks da C-ABI em `pkg/cabi/valhalla_lrc.h`, `pkg/cabi/cabi.go`, `include/valhalla_lrc.h` e `sdk/c/include/valhalla_lrc.h` (`onReceiveInteraction`, `onAnnounceSynchronizationPoint`, `onFederationSynchronized`, `onSyncPointRegistrationSucceeded`, `onSyncPointRegistrationFailed`), com marshaling determinístico (chaves ordenadas), conformidade estrita às regras de ponteiros CGo (`C.CBytes` + `defer C.free` para buffers e tags em memória C, sem armazenar ponteiros Go não pinados em memória C, verificado com `cgocheck2`), eliminação de data races em `ambContext.cb` via `sync.RWMutex` (`cbMu`), sincronização binária completa do layout da struct `ValhallaCallbacks` com os SDKs Java (`IValhallaLrc.java`) e C++ (`ValhallaRTIambassador.cpp` com `std::memset(&cbs, 0, sizeof(cbs))`), implementação das APIs C-ABI para gerenciamento de interações e pontos de sincronização com guardas contra ponteiros nulos (`VALHALLA_ERR_INVALID_ARG`), e propagação canônica do motivo real de falha de sincronização (`SyncPointFailurePayload`). Validado com testes unitários/adversariais em `pkg/cabi/cabi_test.go`, compilação Maven no SDK Java e `go test -race ./...`.

### ⚠️ 2.2. Motor Anti-Stall e Heartbeat Inativos ([pkg/crc/core/time_manager.go](pkg/crc/core/time_manager.go#L551))
* A funcionalidade documentada como *"Anti-Stall Resilience Engine"* implementa os métodos `EvictStalledFederates(timeout)` e `Heartbeat(handle)`.
* **Nenhum componente do servidor (CRC, server, handlers)** chama esses métodos em tempo de execução. Não existe ticker, temporizador nem monitor em background. O código é estático e morto em produção, sendo executado apenas dentro de testes unitários isolados.

> ✅ **STATUS DE REMEDIAÇÃO (26/11/2026): CORRIGIDO E APROVADO** (commit `8726ea5`)  
> Ativado o motor Anti-Stall em background no servidor CRC (`pkg/crc/server.go`) via goroutine `antiStallMonitorLoop` gerenciada por `sync.WaitGroup` e `<-s.stopChan`, executando varreduras periódicas em todas as federações (`GetAllFederations()`), aplicando `EvictStalledFederates(timeout)` e despachando concessões pendentes (`dispatchTimeGrants`). Protegido contra falsa quarentena de nós em espera de avanço (`State != TimeStateGranted`) e com renovação de `LastHeartbeat` no momento da concessão em `pkg/crc/core/time_manager.go`. Implementado o protocolo de vitalidade implícito (renovado em qualquer mensagem de rede em `native_handler.go` e `portico_handler.go`) e explícito (`NativeMsgHeartbeat uint8 = 70`), exposto no LRC (`RTIambassador.Heartbeat()`), C-ABI (`valhalla_heartbeat`), headers C e SDK Java (`IValhallaLrc.java`). Validado com testes de integração ponta-a-ponta (`test/integration/anti_stall_test.go`), C-ABI (`pkg/cabi/cabi_test.go`) e `go test -race ./...`.

---

## 3. Erros Não Tratados e Supressão em Massa

### ⚠️ 3.1. 377 Ocorrências de Supressão Explícita (`_ = `)
O código silencia sistematicamente erros críticos de I/O de rede e integridade:
- **155 supressões** em [`pkg/crc/native_handler.go`](pkg/crc/native_handler.go)
- **82 supressões** em [`pkg/crc/portico_handler.go`](pkg/crc/portico_handler.go)
- **17 supressões** em [`pkg/crc/web_gateway.go`](pkg/crc/web_gateway.go)

### ⚠️ 3.2. Falhas Críticas de Transporte Ignoradas
- Instruções como `_ = session.SendMessage(...)` e `_ = f.Session.SendNotification(...)` ignoram completamente falhas de socket (quebra de pipe, timeout, desconexão abrupta).
- Em [`native_handler.go`](pkg/crc/native_handler.go#L621), se a atualização de atributos falhar (`_ = currentFed.UpdateObjectAttributes(...)`), o erro é ignorado e o servidor envia resposta de sucesso (`NativeMsgSuccessOK`) para o cliente.

### ⚠️ 3.3. Descarte Silencioso de Callbacks no LRC ([pkg/lrc/client.go](pkg/lrc/client.go#L175-L179))
```go
select {
case c.callbacks <- callbackItem{MsgType: hdr.MsgType, Payload: payload}:
default:
    // Queue saturated, drop to prevent memory ballooning
}
```
Se a taxa de geração de eventos ultrapassar o buffer de 1.024 mensagens antes que o federado invoque `EvokeCallback`, **mensagens da simulação são descartadas sem erro nem aviso**, rompendo a garantia de entrega confiável exigida pelo padrão IEEE 1516.

> ✅ **STATUS DE REMEDIAÇÃO (26/11/2026): CORRIGIDO E APROVADO** (commits `9bb73d2`, `db39373`)  
> Implementada fila elástica dinâmica no cliente LRC (`pkg/lrc/client.go`) através de um pipeline com `inQueue` (4.096), `queueLoop` em background com expansão de slice e teto configurável (`DefaultMaxCallbackQueueCapacity = 262.144`) drenando em estrita ordem FIFO para `callbacks` (1.024). Eliminado o descarte silencioso e garantida a drenagem limpa no encerramento de rede. Ao atingir o teto de segurança, a saturação é sinalizada atomicamente e `EvokeCallback()` retorna explicitamente `ErrCallbackQueueSaturated`. Expostos métodos de inspeção e controle: `GetCallbackQueueStatus()`, `ResetCallbackQueueSaturation()` e `SetCallbackQueueCapacity()`. Validação empírica via `pkg/lrc/callback_queue_test.go`, teste de integração ponta-a-ponta (`test/integration/callback_queue_test.go`) com rajada de 2.500 interações sem perda de mensagens, e `go test -race ./...` com 100% PASS e 0 data races.

### ⚠️ 3.4. Descarte Silencioso de Mensagens no Logger ([pkg/logger/logger.go](pkg/logger/logger.go#L251-L254))
Quando a fila assíncrona de 10.000 logs atinge saturação, eventos de severidade `ERROR` ou `CRITICAL` são descartados no bloco `default:`. Além disso, a função `Flush()` retorna imediatamente sem aguardar o esvaziamento caso a fila esteja cheia.

> ✅ **STATUS DE REMEDIAÇÃO (26/11/2026): CORRIGIDO E APROVADO** (commit `5a68331`)  
> Implementado fallback síncrono imediato para eventos de severidade `LevelError` e `LevelCritical` sob saturação de fila, protegido por mutex granular de escritor (`l.outMu`), garantindo entrega ponta-a-ponta e eliminação de descartes silenciosos de falhas. Descartes de mensagens não-críticas (`TRACE`, `DEBUG`, `INFO`, `WARN`) sob saturação são monitorados e contabilizados atomicamente (`atomic.AddUint64(&droppedLogCount, 1)`), com consulta via `DroppedCount()` e reset via `ResetDroppedCount()`. Corrigida a função `Flush()`, removendo o descarte precoce em `default:` e assegurando bloqueio real até a drenagem completa das mensagens pela sentinela assíncrona; adicionada a função `FlushTimeout(timeout)`. Validação via suíte adversarial dedicada (`pkg/logger/logger_adversarial_test.go`) e `go test -race ./...` com 100% PASS e 0 data races.

---

## 4. Violações Arquiteturais, Anti-Padrões e Falhas de Delegação

### 🔴 4.1. "God Handlers" e Violação do SRP (Single Responsibility Principle)
- [`pkg/crc/native_handler.go`](pkg/crc/native_handler.go) centraliza **1.294 linhas em uma única função** (`HandleNativeConnection`), com um `switch` contendo 37 casos responsáveis simultaneamente por:
  - Leitura e enquadramento de rede;
  - Deserialização de payloads;
  - Regras de negócio de membros, federação e objetos;
  - Decodificação de coordenadas e regras de DDM;
  - Políticas de governança de posse;
  - Roteamento e fan-out para outros federados;
  - Formatação e transmissão de pacotes de erro.
- [`pkg/crc/portico_handler.go`](pkg/crc/portico_handler.go) (849 linhas) e [`pkg/crc/web_gateway.go`](pkg/crc/web_gateway.go) (995 linhas) reproduzem a mesma falha estrutural.
- **Triplicação da Lógica de Domínio:** A lógica de validação de membros, busca de instâncias de objetos, filtragem por DDM e checagem de propriedade está repetida em 3 arquivos diferentes, em vez de ser delegada a um serviço central do domínio de federação.

### 🔴 4.2. Corrupção de Protocolo no Despacho Cruzado entre Dialetos
A alegação de *"multiplexação sem gateways intermediários"* oculta um defeito grave de invasão de domínio entre protocolos:
1. **Nativo $\to$ Portico:** Em [`native_handler.go`](pkg/crc/native_handler.go#L597), quando um federado nativo exclui um objeto, o servidor executa:
   ```go
   _ = f.Session.SendNotification(NativeMsgRemoveObjectInstance, removePayload)
   ```
   No manipulador de sessão do Portico ([`portico_session.go`](pkg/crc/portico_session.go#L128)), o tipo `NativeMsgRemoveObjectInstance` (código numérico 47) é convertido diretamente para `wire.MessageType(47)`. No Portico, **o tipo de mensagem 47 não existe** (a exclusão no Portico é 64, `MsgDeleteObject`), e o payload enviado é a estrutura binária em Go, e não o Java Object Serialization. O cliente Portico rejeita ou falha.
2. **Nativo $\to$ Portico (Reflexão de Atributos):** Em [`native_handler.go`](pkg/crc/native_handler.go#L662), a mensagem `NativeMsgReflectAttributes` (código 43) é repassada para clientes Portico. No Portico, **43 corresponde a `MsgAchieveSyncPoint`**.
3. **Portico $\to$ Web:** Em [`portico_handler.go`](pkg/crc/portico_handler.go#L470), notificações Portico enviam o código 254 com payload binário Java diretamente para clientes Web, que esperam JSON formatado e código nativo 43.

> ✅ **STATUS DE REMEDIAÇÃO (26/11/2026): CORRIGIDO E APROVADO** (commit `e78c33d`)  
> Implementada a camada de tradução automática e transparente em `PorticoSession.SendNotification` (`pkg/crc/portico_session.go`), interceptando mensagens canônicas `common.NativeMsg*` provenientes de nós Nativo ou Web e convertendo-as para os correspondentes oficiais do Portico: `NativeMsgDiscoverObject` (41) $\to$ `MsgDiscoverObject` (61), `NativeMsgRemoveObjectInstance` (47) $\to$ `MsgDeleteObject` (64), `NativeMsgReflectAttributes` (43) $\to$ `MsgReflectAttributes` (254), `NativeMsgReceiveInteraction` (45) $\to$ `MsgReceiveInteraction` (255), `NativeMsgAnnounceSyncPoint` (9) $\to$ `MsgAnnounceSyncPoint` (42), `NativeMsgFederationSynchronized` (12) $\to$ `MsgFederationSynchronized` (44), `NativeMsgTimeAdvanceGrant` (67) $\to$ `MsgTimeAdvanceGrant` (126), com tratamento e retorno explícito de erros. Corrigido o fan-out em `pkg/crc/portico_handler.go`, enviando JOS para `ProtocolPortico` e `common.NativeMsg*` canônico para `ProtocolNative` e `ProtocolWeb`. Estendida a sessão Web (`pkg/crc/web_session.go`) com suporte a `receiveInteraction` e decodificação resiliente de pontos de sincronização. Criados os metamodelos e construtores JOS oficiais `NewDeleteObject`, `NewReceiveInteraction` (`ClassDescReceiveInteraction`) e boxing automático em `NewJavaHashSet`. Validado com suíte de integração de ponta a ponta (`test/integration/cross_protocol_dispatch_test.go`) cobrindo todas as travessias entre dialetos e `go test -race ./...` com 100% PASS e 0 data races.

### ⚠️ 4.3. Colisão de Nomes de Classes na OMT ([pkg/metamodel/xml_parser.go](pkg/metamodel/xml_parser.go#L111))
O registro de OMT indexa as classes pelo seu nome simples (`omt.objectNameToClass[name]`). No padrão IEEE 1516 e no RPR-FOM, classes podem compartilhar nomes simples em sub-ramos distintos (ex.: `A.Sensor` e `B.Sensor`). No parser do Valhalla, a segunda classe registrada causará `ErrDuplicateClass` ou substituirá indevidamente a classe original.

> ✅ **STATUS DE REMEDIAÇÃO (27/11/2026): CORRIGIDO E APROVADO** (commit `f299929`)  
> Implementada indexação canônica por FQCN na OMT e resolução hierárquica por pai no parser XML. Classes e interações em `pkg/crc/core/omt_registry.go` agora armazenam `QualifiedName` (FQCN completo, ex.: `HLAobjectRoot.Platform.Sensor` vs. `HLAobjectRoot.Environment.Sensor`) com índices especializados `objectFQCNToClass` / `interactionFQCNToClass` e `objectSimpleNameToHandles` / `interactionSimpleNameToHandles`. Nomes curtos idênticos em ramos distintos coexistem com handles independentes e intervalos de Euler Tour isolados, enquanto duplicatas verdadeiras sob o mesmo pai continuam retornando `ErrDuplicateClass` e `ErrDuplicateInteraction`. A resolução suporta busca por FQCN canônico, nome qualificado relativo ou nome simples não-ambíguo via `ResolveObjectClass`, `ResolveInteractionClass`, `GetObjectClassByFQCN` e `GetChildObjectClass`. No parser XML (`pkg/metamodel/xml_parser.go`), a busca global plana por nome simples foi substituída por `GetChildObjectClass(parentHandle, name)` e `GetChildInteractionClass(parentHandle, name)`, eliminando fusões indevidas de atributos/parâmetros entre ramos distintos e preservando extensões modulares multi-FOM. Validado com 100% PASS sob `go test -race ./...` e 0 data races.

### ⚠️ 4.4. Mapeamento Frágil de Erros via Strings na C-ABI ([pkg/cabi/cabi.go](pkg/cabi/cabi.go#L38-L66))
A conversão de erros Go para códigos de status da C-ABI é realizada através de busca textual com `strings.Contains(msg, ...)` em vez de erros tipados (`errors.Is` / `errors.As`). Pequenas alterações em textos descritivos quebram o lançamento de exceções nos SDKs C++ e Java.

> ✅ **STATUS DE REMEDIAÇÃO (27/11/2026): CORRIGIDO E APROVADO** (commit `61f2151`)  
> Eliminação completa de correspondência textual via strings (`strings.Contains`) na conversão de erros para a C-ABI em `pkg/cabi/cabi.go`. Implementada resolução de erros canônicos remotos via `common.AsCanonicalError` no LRC (`pkg/lrc/rti_ambassador.go`), e mapeamento 100% tipado com `errors.Is` abrangendo todos os sentinelas normativos de `lrc` e `common`. Conforme nota do auditor, os sentinelas de OMT foram centralizados em `pkg/common`, desacoplando totalmente a camada cliente C-ABI do pacote `pkg/crc/core`. Validado por `TestCABI_TypedErrorMapping` com 30 casos de teste cobrindo erros diretos e encapsulados (`%w`).

### ⚠️ 4.5. Falta de Guarda de Ponteiros Nulos na C-ABI
Funções exportadas como `valhalla_connect`, `valhalla_create_federation`, etc., invocam `C.GoString(address)` e `C.GoString(fedName)` sem verificar se o ponteiro C é `NULL`. Se um ponteiro nulo for repassado, ocorre falha de segmentação (SIGSEGV) irreversível na JVM ou processo C++.

> ✅ **STATUS DE REMEDIAÇÃO (27/11/2026): CORRIGIDO E APROVADO** (commits `1b21db2`, `fc1969b`, `61f2151`)  
> Adicionadas guardas exaustivas contra ponteiros nulos (`NULL`) em todas as funções exportadas pela C-ABI (`pkg/cabi/cabi.go`): validação de handle de embaixador (`amb == nil`), strings C (`address == nil`, `fedName == nil`, etc.), ponteiros de saída (`outHandle == nil`, `outOwner == nil`, `outIsOwned == nil`, `outRegionHandle == nil`, `outEvoked == nil`) e contadores negativos / arrays nulos (`count < 0`, `tagLen < 0`, `timeoutSeconds < 0`), retornando `VALHALLA_ERR_INVALID_ARG` determinístico e blindando clientes C/C++/JNA contra falhas de segmentação (SIGSEGV). Validado por `TestCABI_NullPointerSafety` sob `go test -race` e `GOEXPERIMENT=cgocheck2`.

---

## 5. Gargalos de Desempenho e Concorrência

### 🐢 5.1. Recálculo Global da Árvore Merkle a Cada Update
Em [`native_handler.go`](pkg/crc/native_handler.go#L623), a cada mensagem `UpdateAttributes` recebida, o CRC invoca:
```go
globalRoot := currentFed.StateTree().GetGlobalRoot()
```
O método `GetGlobalRoot()` adquire lock exclusivo, aloca slices na heap com todos os identificadores de entidades, executa ordenação com `sort.Ints()`, calcula SHA-256 de todas as entidades e realiza redução binária completa. Sob tráfego de simulação em tempo real (milhares de entidades a 60–100 Hz), esse padrão causa **severa contenção de locks e degradação de throughput**.

### 🐢 5.2. Alocações Desnecessárias na Heap em `EvokeCallback`
Em [`pkg/lrc/rti_ambassador.go`](pkg/lrc/rti_ambassador.go#L1002):
```go
if timeout > 0 {
    timer = time.After(timeout)
}
```
A chamada a `time.After` em loops rápidos aloca um novo canal e estrutura de timer a cada iteração, retida na memória até o término do timeout. Sob chamadas frequentes de bombeamento de eventos, causa **pressão excessiva no Garbage Collector (GC)**. O padrão correto requer `time.NewTimer` reaproveitado ou zerado com `Reset()`.

> ✅ **STATUS DE REMEDIAÇÃO (26/11/2026): CORRIGIDO E APROVADO** (commit `4830cfa`)  
> Implementado gerenciamento de timers via `sync.Pool` com `acquireTimer(d)` e `releaseTimer(t)` executando `Stop()`, drenagem de canal `t.C` e `Reset(d)` seguro, eliminando completamente as alocações de `time.After`. Implementados dois caminhos de fast-path em `EvokeCallback`: (1) caminho não-bloqueante para `timeout <= 0` com `default:`, eliminando o bug de bloqueio infinito em `EvokeCallback(0)` quando a fila está vazia; (2) caminho de despacho imediato quando já existem eventos no buffer, consumindo sem qualquer interação com timers. Resultados empíricos de benchmark: latência reduzida de 225,0 ns/op para 39,98 ns (~5.6x mais rápido), alocações de heap reduzidas de 248 B/op (3 allocs/op) para **estritos 0 B/op e 0 allocs/op**. Validado via `pkg/lrc/evoke_test.go` e `pkg/lrc/evoke_benchmark_test.go` sob `go test -race ./...` com 100% PASS e 0 data races.

### 🐢 5.3. Latência Forçada de 50 ms no Multiplexador de Porta
Em [`pkg/crc/multiplexer.go`](pkg/crc/multiplexer.go#L30-L51), toda conexão que chega à porta `:52295` é submetida a um `Peek(4)` com timeout de 50 ms. Como o Portico opera em modo "Server-First" (aguardando mensagem do servidor), ele sempre incorre em **atraso forçado de 50 ms na conexão**. Adicionalmente, clientes nativos em redes de alta latência que demorarem mais de 50 ms para enviar o preâmbulo `VAL4` são erroneamente classificados como Portico.

> ✅ **STATUS DE REMEDIAÇÃO (27/11/2026): CORRIGIDO E APROVADO** (commit `c0a1238`)  
> Implementada classificação estrita no multiplexador de dialeto em `pkg/crc/multiplexer.go` e listeners dedicados com latência zero em `pkg/crc/server.go`. Tráfego ativo não identificado (ex.: requisições HTTP ou lixo binário) é categorizado como `ProtocolUnknown` e rejeitado com `ErrUnknownProtocol`, eliminando o fallback incorreto para o deserializador Portico JOS. Preâmbulos nativos `"VAL4"` parciais/fragmentados em trânsito são reconhecidos por correspondência de prefixo com janela de tolerância (`graceTimeout`), impedindo má-classificação sob latência ou fragmentação TCP. Adicionados os métodos thread-safe `SetPorticoAddr()` / `PorticoAddr()` e `SetNativeAddr()` / `NativeAddr()` no CRC Server, permitindo conexões diretas aos dialetos sem qualquer inspeção ou atraso (0 ms de espera de handshake), mantendo a porta unificada multiplexada retrocompatível com `SetPeekTimeout()` e `PeekTimeout()` configuráveis (default otimizado para 15 ms). Validado por suíte de testes unitários e adversariais em `pkg/crc/multiplexer_test.go` e `pkg/crc/server_test.go` (`TestMultiplexer_AlienProtocolRejected`, `TestMultiplexer_PartialNativePreamble_NotClassifiedAsPortico`, `TestMultiplexer_SegmentedNativePreamble_Success`, `TestServer_DedicatedPorticoPort_ZeroLatency`, `TestServer_DedicatedNativePort_ZeroLatency`, `TestServer_ConfigurablePeekTimeout`) e `go test -race ./...` com 100% PASS e 0 data races.

### 🐢 5.4. Bind em Porta Privilegiada (`:1`) pelo Web Gateway
Em [`pkg/crc/server.go`](pkg/crc/server.go#L51-L53):
```go
if pNum, err := strconv.Atoi(parts[1]); err == nil {
    webAddr = fmt.Sprintf("%s:%d", parts[0], pNum+1)
}
```
Quando o servidor CRC é configurado para escutar em porta dinâmica atribuída pelo sistema operacional (`127.0.0.1:0`), `pNum + 1` resulta na tentativa de bind na porta `127.0.0.1:1`. Em ambientes Unix/Linux sem privilégios de root, a inicialização do Web Gateway falha silenciosamente.

> ✅ **STATUS DE REMEDIAÇÃO (27/11/2026): CORRIGIDO E APROVADO** (commit `a456026`)  
> Implementada derivação segura de portas para o Web Gateway do CRC em `pkg/crc/server.go` utilizando `net.SplitHostPort` e `net.JoinHostPort`, garantindo suporte completo a IPv4, IPv6 (`[::1]:0`) e interfaces genéricas (`:0`). Quando portas dinâmicas (`pNum == 0`) ou no teto de representação (`pNum >= 65535`) são solicitadas, o Web Gateway agora permanece em porta efêmera (`:0`), instruindo o sistema operacional a atribuir uma porta dinâmica desprivilegiada (>1024), eliminando qualquer tentativa de bind na porta privilegiada `:1`. Implementado fallback automático para `:0` em `s.webGateway.Start()` caso a porta derivada colida e adicionados os métodos thread-safe `Server.WebAddr()` e `Server.WebGateway()`. Removidos os workarounds de instâncias duplicadas de WebGateway nos testes de integração. Validado com testes unitários em `pkg/crc/server_test.go` (`TestServer_WebGateway_EphemeralPortNeverBindsToPort1`, `TestServer_WebGateway_StartWithEphemeralPort_EndToEnd` via HTTP `/api/v1/status`), testes ponta-a-ponta (`TestHLA4_WebProfile_11Stages`, `TestCrossProtocol_PorticoToWeb_ReflectAttributes`) e `go test -race ./...` com 100% PASS e 0 data races.

---

## 📊 Matriz de Risco e Prioridade de Correção

| ID | Categoria | Descrição do Problema | Severidade | Impacto no Sistema |
| :---: | :--- | :--- | :---: | :--- |
| **01** | **Conformidade / Verdade** | PQC (ML-KEM / Kyber / HPKE / AES-GCM) inexistente | **CRÍTICO** | Risco de conformidade e propaganda técnica enganosa |
| **02** | **Conformidade / Verdade** | Serialização SSZ e UDP framing inexistentes | **ALTO** | Incompatibilidade com as normas técnicas declaradas |
| **03** | **Conformidade / Verdade** | Merkle Desync e DDM Morton bitwise inoperantes | **ALTO** | O cliente não valida Merkle; Morton decodifica para AABB |
| **04** | **Corrupção de Dados** | Despacho multi-dialeto cruzado envia mensagens incompatíveis | **CRÍTICO** | Clientes Portico e Web quebram ao receber eventos de nós nativos |
| **05** | **Stubs Silenciosos** | Callbacks de Interação e Sincronização vazios na C-ABI | **CRÍTICO** | Federados C/C++/Java não recebem interações nem sync points |
| **06** | **Tratamento de Erros** | 377 supressões (`_ = `) e descarte silencioso de callbacks | **ALTO** | Perda silenciosa de pacotes e desvio de estado simulado |
| **07** | **Arquitetura** | God Handlers (> 1.000 linhas) e triplicação de lógica | **MÉDIO** | Dívida técnica severa, alto custo de manutenção |
| **08** | **Desempenho** | Recálculo síncrono SHA-256 de todas as entidades por update | **ALTO** | Congelamento de CPU e contenção de mutex sob alta frequência |

---

## 🛠️ Plano Recomendado de Remediação

1. **Ajuste Documental:** Remover imediatamente afirmações de ML-KEM-768, HPKE, AES-256-GCM, SSZ e UDP framing do `README.md` e `BENCHMARK.md`, reclassificando-as explicitamente como *Roadmap / Proposta Futura*.
2. **Correção dos Callbacks C-ABI:** Implementar a passagem de parâmetros e invocação dos ponteiros de função para `ReceiveInteraction`, `AnnounceSynchronizationPoint` e `FederationSynchronized` em [`pkg/cabi/cabi.go`](pkg/cabi/cabi.go).
3. **Barramento Unificado de Eventos no CRC:** Criar um módulo desacoplado de despacho (`EventDispatcher`) que traduza o evento canônico de simulação para a linguagem/formato nativo de cada sessão de destino antes da transmissão.
4. **Desmembramento dos God Handlers:** Quebrar as funções monolíticas de rede em processadores de comando especializados (`handlers/command pattern`), isolando a camada de transporte da camada de domínio.
5. **Cálculo Sob Demanda da Árvore Merkle:** Desacoplar `GetGlobalRoot()` do loop de recepção de atributos, tornando-o assíncrono ou ativado somente sob demanda expressa de auditoria.
6. **Eliminação de `time.After` e Supressões de Erro:** Substituir timers efêmeros por instâncias gerenciadas em `EvokeCallback`, além de adicionar tratamento adequado ou instrumentação de log para todos os retornos de erro de rede.

---

## ✅ Histórico de Remediações Verificadas

| Item | Descrição | Commits | Status |
| :---: | :--- | :--- | :---: |
| **1.4** | DDM 3D com Curvas de Morton (filtragem bitwise zero-decode + prefixo de octree) | `a7db266` | **✅ CORRIGIDO** |
| **1.2** | Serialização e Merkleização SSZ (Ethereum 2.0) canônica, determinística e anti-DoS | `552a97c`, `ba6a8a8`, `bc1981a` | **✅ CORRIGIDO** |
| **1.3** | Árvores Merkle e Validação de Desync no LRC + Protocolo de Provas de Inclusão | `ca8e127` | **✅ CORRIGIDO** |
| **2.1** | C-ABI: Implementação completa de Callbacks, Funções Ausentes, Proteções CGo e Sincronização de ABI | `1b21db2`, `fc1969b` | **✅ CORRIGIDO** |
| **2.2** | Motor Anti-Stall em Background no CRC e Protocolo de Heartbeat Implícito e Explícito | `8726ea5` | **✅ CORRIGIDO** |
| **3.3** | Fila Elástica e Eliminação de Descarte Silencioso de Callbacks no LRC com Alerta de Saturação | `9bb73d2`, `db39373` | **✅ CORRIGIDO** |
| **3.4** | Fallback Síncrono de Logs ERROR/CRITICAL sob Saturação e Bloqueio Efetivo de Flush | `5a68331` | **✅ CORRIGIDO** |
| **4.2** | Despacho Cruzado entre Dialetos (Nativo VAL4 ↔ Portico JOS ↔ Web JSON-WS) sem Corrupção | `e78c33d`, `cc1a47c` | **✅ CORRIGIDO** |
| **5.2** | Eliminação de Alocações em EvokeCallback via sync.Pool de Timers e Fast-Paths Não-Bloqueantes | `4830cfa` | **✅ CORRIGIDO** |
| **5.3** | Eliminação de Latência Forçada de 50 ms no Multiplexador e Classificação Estrita de Dialetos | `c0a1238` | **✅ CORRIGIDO** |
| **5.4** | Eliminação do Bind Indevido na Porta Privilegiada :1 pelo Web Gateway sob Portas Efêmeras | `a456026` | **✅ CORRIGIDO** |
| **1.5** | Correção de Alegações de UDP Framing (TCP Linear + WebSockets; UDP movido para Roadmap) | `292be6c` | **✅ CORRIGIDO** |
| **1.6** | Temporizadores de Alta Precisão Multiplataforma (QPC / CLOCK_MONOTONIC) e Remoção de P50 < 1 µs | `3edf7b6` | **✅ CORRIGIDO** |
| **1.7** | Ingestão e Validação Modular Ponta-a-Ponta dos 14 Módulos Normativos do SISO RPR-FOM v2.0 | `4c96732` | **✅ CORRIGIDO** |
| **4.3** | Indexação Canônica FQCN na OMT e Resolução Hierárquica por Pai no Parser XML (Zero Colisão) | `f299929` | **✅ CORRIGIDO** |
| **1.8** | Alinhamento da Estrutura de Diretórios no `README.md` com a Árvore Real do Repositório | `b6bacf0` | **✅ CORRIGIDO** |
| **4.4** | Mapeamento 100% Tipado via `errors.Is` na C-ABI e Centralização de Sentinelas em `pkg/common` | `61f2151` | **✅ CORRIGIDO** |
| **4.5** | Guardas Exaustivas contra Ponteiros `NULL` e Limites Negativos nas Funções Exportadas da C-ABI | `1b21db2`, `fc1969b`, `61f2151` | **✅ CORRIGIDO** |

---

## 🔴 Itens Pendentes de Remediação (5 de 23)

| Item | Categoria | Descrição | Severidade | Situação Atual |
| :---: | :--- | :--- | :---: | :--- |
| **1.1** | **Vaporware / Segurança** | Post-Quantum Cryptography (ML-KEM-768 / Kyber / HPKE / AES-256-GCM) | **CRÍTICO** | Inexistente no código; implementar ou reclassificar como Roadmap na documentação. |
| **3.1** | **Tratamento de Erros** | Supressão em massa de erros (`_ = `) | **ALTO** | Centenas de supressões em `native_handler.go`, `portico_handler.go` e `web_gateway.go`. |
| **3.2** | **Tratamento de Erros** | Falhas críticas de transporte e `UpdateObjectAttributes` ignoradas | **ALTO** | Erros de `SendMessage`, `SendNotification` e `UpdateObjectAttributes` ainda descartados. |
| **4.1** | **Arquitetura / SRP** | "God Handlers" (> 1.000 linhas) e triplicação da lógica de domínio | **MÉDIO** | Handlers de transporte monolíticos e com lógica de federação duplicada. |
| **5.1** | **Desempenho** | Recálculo global da árvore Merkle (`GetGlobalRoot()`) a cada `UpdateAttributes` | **ALTO** | `native_handler.go` ainda invoca `GetGlobalRoot()` sincronamente em cada atualização. |


