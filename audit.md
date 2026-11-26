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

### 🔴 1.6. Distorções em Benchmarks e "P50 < 1 µs" Artificial
* **Alegações em Documentação ([BENCHMARK.md](BENCHMARK.md#L40-L45), [README.md](README.md#L58-L66)):**
  - Apresenta tabelas comparando Valhalla contra *Pitch pRTI 5.4*, *MÄK RTI 4.5*, *CERTI* e *Portico* com precisão de frações de nanosegundos (ex.: "OMT Class Lookup: 1.14 ns vs 48.20 ns no Pitch"). Produtos proprietários comerciais em C++ não disponibilizam microbenchmarks isolados de chamadas internas em nanosegundos.
  - No benchmark interno em [`test/benchmark/end_to_end_benchmark_test.go`](test/benchmark/end_to_end_benchmark_test.go#L117-L118):
    ```go
    elapsed := time.Duration(time.Now().UnixNano() - sentNano)
    ```
    No ambiente Windows, a resolução do temporizador do sistema possui granularidade de milissegundos (~0,5 a 1 ms). Iterações executadas dentro da mesma fatia de clock geram diferença `0 s` (`0.00 µs`), que foi documentada de forma enganosa como *"P50 (Median) : < 1 µs (microsecond resolution limit)"*.
  - A decomposição de overhead da Lei de Amdahl (0.18 µs de marshaling, 0.28 µs de bitset router) foi arbitrariamente decomposta no texto, sem instrumentação contínua de amostragem.

### 🔴 1.7. Ingestão Incompleta do RPR-FOM v2.0
* **Alegações em Documentação ([README.md](README.md#L49-L51), [TECHNICAL-MEMORY.md](TECHNICAL-MEMORY.md#L92-L96)):**
  > *"Full end-to-end integration and schema ingestion validated against official RPR-FOM v2.0 (SISO-STD-001-2015) schemas across all 14 modules."*
* **Realidade no Código:**
  O teste [`test/resources_test.go`](test/resources_test.go#L52-L60) não ingere o metamodelo nem instancia classes; ele apenas realiza um `xml.Unmarshal` básico verificando se o nó raiz é `<objectModel>`. Os testes do parser real ([`pkg/metamodel/xml_parser_test.go`](pkg/metamodel/xml_parser_test.go#L80)) ingerem somente 2 dos 14 módulos (`RPR-Base` e `RPR-Physical`).

### 🔴 1.8. Estrutura de Diretórios Fictícia no README
* O [README.md](README.md#L87-L91) indica os pacotes `pkg/euler/`, `pkg/fom/`, `pkg/morton/`, `pkg/vector/`, `pkg/ssz/` e `pkg/crypto/`. Nenhum deles existe na raiz de `pkg/` (foram fundidos em `pkg/crc/core` ou não existem).

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

### ⚠️ 3.4. Descarte Silencioso de Mensagens no Logger ([pkg/logger/logger.go](pkg/logger/logger.go#L251-L254))
Quando a fila assíncrona de 10.000 logs atinge saturação, eventos de severidade `ERROR` ou `CRITICAL` são descartados no bloco `default:`. Além disso, a função `Flush()` retorna imediatamente sem aguardar o esvaziamento caso a fila esteja cheia.

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

### ⚠️ 4.3. Colisão de Nomes de Classes na OMT ([pkg/metamodel/xml_parser.go](pkg/metamodel/xml_parser.go#L111))
O registro de OMT indexa as classes pelo seu nome simples (`omt.objectNameToClass[name]`). No padrão IEEE 1516 e no RPR-FOM, classes podem compartilhar nomes simples em sub-ramos distintos (ex.: `A.Sensor` e `B.Sensor`). No parser do Valhalla, a segunda classe registrada causará `ErrDuplicateClass` ou substituirá indevidamente a classe original.

### ⚠️ 4.4. Mapeamento Frágil de Erros via Strings na C-ABI ([pkg/cabi/cabi.go](pkg/cabi/cabi.go#L38-L66))
A conversão de erros Go para códigos de status da C-ABI é realizada através de busca textual com `strings.Contains(msg, ...)` em vez de erros tipados (`errors.Is` / `errors.As`). Pequenas alterações em textos descritivos quebram o lançamento de exceções nos SDKs C++ e Java.

### ⚠️ 4.5. Falta de Guarda de Ponteiros Nulos na C-ABI
Funções exportadas como `valhalla_connect`, `valhalla_create_federation`, etc., invocam `C.GoString(address)` e `C.GoString(fedName)` sem verificar se o ponteiro C é `NULL`. Se um ponteiro nulo for repassado, ocorre falha de segmentação (SIGSEGV) irreversível na JVM ou processo C++.

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

### 🐢 5.3. Latência Forçada de 50 ms no Multiplexador de Porta
Em [`pkg/crc/multiplexer.go`](pkg/crc/multiplexer.go#L30-L51), toda conexão que chega à porta `:52295` é submetida a um `Peek(4)` com timeout de 50 ms. Como o Portico opera em modo "Server-First" (aguardando mensagem do servidor), ele sempre incorre em **atraso forçado de 50 ms na conexão**. Adicionalmente, clientes nativos em redes de alta latência que demorarem mais de 50 ms para enviar o preâmbulo `VAL4` são erroneamente classificados como Portico.

### 🐢 5.4. Bind em Porta Privilegiada (`:1`) pelo Web Gateway
Em [`pkg/crc/server.go`](pkg/crc/server.go#L51-L53):
```go
if pNum, err := strconv.Atoi(parts[1]); err == nil {
    webAddr = fmt.Sprintf("%s:%d", parts[0], pNum+1)
}
```
Quando o servidor CRC é configurado para escutar em porta dinâmica atribuída pelo sistema operacional (`127.0.0.1:0`), `pNum + 1` resulta na tentativa de bind na porta `127.0.0.1:1`. Em ambientes Unix/Linux sem privilégios de root, a inicialização do Web Gateway falha silenciosamente.

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

