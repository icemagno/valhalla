# Auditoria de Engenharia e Arquitetura — Valhalla (HLA-4 / IEEE 1516-2025 RTI)

**Data:** 2026-11-27  
**Escopo:** Auditoria completa e adversarial do repositório `valhalla` (`cmd/`, `pkg/`, `sdk/`, `include/`, `examples/`, `test/` e documentação `README.md`, `BENCHMARK.md`, `TECHNICAL-MEMORY.md`, `docs/*.md`).  
**Metodologia:** Leitura direta de código-fonte, análise estática (`go vet`), execução real da suíte de testes com detector de data races (`go test -race -count=1 ./...`), coleta de cobertura (`go test -cover ./...`) e execução empírica de benchmarks (`go test -bench=. -benchmem ./pkg/crc/core`).

---

## 1. Resumo Executivo e Evidências Empíricas

Embora o projeto compile em Go, passe em `go vet ./...` (exit 0) e passe em `go test -race -count=1 ./...` (exit 0), a auditoria em profundidade revelou **falhas estruturais graves**, **vulnerabilidades de DoS/panic**, **bypass criptográfico no Merkle Tree**, **código C++ que sequer compila**, **mentiras empíricas em benchmarks e documentação**, **SDKs que retornam resultados falsos ou ignoram parâmetros normativos**, **data races confirmados em código de produção** e **testes viciados, circulares ou que não testam o que seus nomes afirmam**.

### 1.1 Cobertura Real de Testes (`go test -cover ./...`)

| Pacote | Cobertura (Statements) | Diagnóstico |
| :--- | :---: | :--- |
| `cmd/valhalla-crc` | `0.0%` | Sem testes. |
| `cmd/valhalla-federate` | `0.0%` | Sem testes. |
| `pkg/cabi` | **`3.7%`** | `cabi_test.go` está escondido atrás da build tag `//go:build cabi_test`; o `go test ./...` padrão executa apenas 43 linhas de `cabi_unit_test.go`. |
| `pkg/crc/portico/wire/metamodel` | **`16.8%`** | Maioria dos serializadores/desserializadores JOS sem testes diretos. |
| `pkg/crc` | **`29.1%`** | Baixa cobertura unitária nos dispatchers Native/Portico/Web e sessions. |
| `pkg/lrc` | `59.6%` | Cobertura moderada, mas com testes negativos viciados (erros de setup ignorados). |
| `pkg/crc/portico/wire` | `61.0%` | Cobertura enganosa: vários testes usam apenas `bytes.Contains` ou `len > 0`. |
| `pkg/logger` | `61.1%` | Testes dependentes de `time.Sleep`. |
| `pkg/crc/core` | `62.4%` | Diversos métodos públicos usados apenas em testes. |
| `pkg/common` | `65.5%` | Testes predominantemente circulares (auto-roundtrip sem vetores independentes). |
| `pkg/crc/portico/transport` | `66.4%` | Goroutine vazada e porta fixa em teste. |
| `pkg/metamodel` | `71.4%` | Boa cobertura, porém duplicada em `test/resources_test.go`. |

### 1.2 Verificação Empírica de Benchmarks (`go test -run=^$ -bench=. -benchmem -benchtime=200ms ./pkg/crc/core`)

| Benchmark | Alegado (`README.md` / `BENCHMARK.md`) | Real Empírico (Intel i5-13400F) | Veredito |
| :--- | :--- | :--- | :--- |
| `BenchmarkBitset_Union` | `33.70 ns/op \| 0 B/op \| 0 allocs/op` | `37.39 ns/op \| 128 B/op \| 1 allocs/op` | **FALSO** (`b1.Clone()` aloca) |
| `BenchmarkInterestMatcher_GetAttributeSubscribers` | `275.70 ns/op \| 0 B/op \| 0 allocs/op` | `301.5 ns/op \| 656 B/op \| 2 allocs/op` | **FALSO** (2 alocações/op) |
| `BenchmarkMerkleTree_ComputeObjectRoot` | `870.00 ns/op \| 0 B/op \| 0 allocs/op` | **`8700 ns/op \| 328 B/op \| 5 allocs/op`** | **FALSO (10× mais lento + 5 allocs)** |
| `BenchmarkMerkleTree_GetGlobalRoot_Cached` | `30.10 ns/op \| 0 allocs/op` | **Não existe no repositório** | **INEXISTENTE** |
| `BenchmarkMorton_ContainsPoint` | Listado em `README.md:81` | **Não existe** (nome real: `BenchmarkSpatialRegionContainsPoint`) | **NOME ERRADO** |
| `TestBenchmark_EndToEnd_RealNetworkLatency` | Comando de reprodução em `BENCHMARK.md:153` | **Não existe** (roda 0 testes) | **COMANDO QUEBRADO** |

---

## 2. Achados Críticos (P0 — Segurança, DoS, Panics, Quebra de Build e Corrupção)

### 2.1 DoS / Panic Instantâneo em `JOSDecoder.ReadObject` (`TC_BLOCKDATALONG`)
- **Local:** `pkg/crc/portico/wire/jos_decoder.go:224-233`
- **Problema:** O caso `0x7A` (`TC_BLOCKDATALONG`) lê um `int32` (`iLen`) diretamente do stream e executa `buf := make([]byte, iLen)` **sem qualquer checagem de limite ou sinal** (embora `ReadBlockData` nas linhas 149-151 e `skipCustomData` nas linhas 514-516 validem `iLen < 0 || int(iLen) > maxJOSAllocation`).
- **Impacto:** Um peer malicioso enviando `iLen = -1` derruba o CRC instantaneamente com `panic: runtime error: makeslice: len out of range`; enviando `0x7FFFFFFF` (2 GiB) causa OOM imediato.

### 2.2 DoS de Alocação Ilimitada (OOM) em `lrc.Client.readLoop`
- **Local:** `pkg/lrc/client.go:316-320`
- **Problema:** `hdr.PayloadLen` (`uint32` não confiável, até 4 GiB) é passado diretamente para `make([]byte, hdr.PayloadLen)` sem validação de tamanho máximo de frame.
- **Impacto:** Um pacote corrompido ou servidor hostil causa OOM imediato no federado. O mesmo ocorre em `pkg/cabi/*.go` onde `fomCount`, `attrCount`, `paramCount` e `dimCount` não têm teto antes de `make(...)`.

### 2.3 Stack Overflow DoS por Recursão Ilimitada em `JOSDecoder.readClassDesc`
- **Local:** `pkg/crc/portico/wire/jos_decoder.go:257-357` (linha 351)
- **Problema:** `superCls, err := d.readClassDesc()` faz recursão sem verificar nem incrementar `d.depth`. Um stream malicioso com cadeia profunda de `TC_CLASSDESC` estoura a pilha da goroutine.

### 2.4 Bypass / Falsificação Criptográfica na Árvore de Merkle da Federação
- **Local:** `pkg/crc/models.go:395-408`, `pkg/crc/native_dispatch_object.go:138`, `pkg/crc/native_dispatch_ddm_omt.go:181`
- **Problema:**
  ```go
  if f.merkleAuditEnabled {
      if merkleRoot == [32]byte{} {
          // só calcula se o cliente mandou zero!
          merkleRoot = core.ComputeObjectMerkleRoot(attrMap)
      }
      obj.MerkleRoot = merkleRoot
      f.stateTree.UpdateObjectState(common.ObjectInstanceHandle(objHandle), merkleRoot)
  }
  ```
- **Impacto:** Qualquer cliente não confiável que preencha `hdr.MerkleRoot` com 32 bytes arbitrários não-nulos faz o CRC **pular o recálculo** e gravar cegamente o hash forjado na `FederationStateTree`. Isso anula a garantia de auditoria criptográfica do protocolo.

### 2.5 Código C++ do SDK Não Compila (`evokeMultipleCallbacks` e `NullFederateAmbassador.h`)
- **Locais:**
  - `sdk/cpp/src/ValhallaRTIambassador.cpp:708-733`
  - `sdk/cpp/include/rti1516e/NullFederateAmbassador.h:1-63` vs `sdk/cpp/include/rti1516e/FederateAmbassador.h:10-39`
- **Evidências:**
  1. Em `evokeMultipleCallbacks`, `double slice = step;` é declarada **dentro** do bloco `do { ... }` (L720) e usada na condição `} while (elapsed < maximumTime && slice > 0.0);` (L731) — **erro de compilação C++** (escopo inválido). Isso também prova que o binário `sdk/cpp/librti1516e.a` commitado no repositório está dessincronizado do código-fonte.
  2. `NullFederateAmbassador.h` não faz `#include <rti1516e/RTIexception.h>`, mas usa `throw(FederateInternalError)` em todos os métodos; declara 5 métodos que **não existem** na classe base `FederateAmbassador`; e adiciona `throw(...)` em overrides cujos métodos base não possuem especificação de exceção (mal-formado em C++).
  3. `include/valhalla_lrc.h:105, 115` usa `_Bool*` (C99), que é inválido em C++ padrão (MSVC / ISO C++ estrito).

### 2.6 Crash por `std::terminate()` em C++ devido a Mapeamento Incorreto de Exceções C-ABI
- **Locais:** `pkg/cabi/cabi.go:72-74`, `sdk/cpp/src/ValhallaRTIambassador.cpp:130-131, 148-149, 378-413`
- **Problema:**
  - `cabi.go:72` mapeia `common.ErrObjectInstanceNameInUse` para `VALHALLA_ERR_FEDERATE_NAME_IN_USE`. Em C++, `registerObjectInstance` chama `checkStatus()`, que lança `FederateNameAlreadyInUse` — exceção ausente na lista `throw(...)` de `registerObjectInstance`, disparando `std::unexpected()` -> **`std::terminate()`**.
  - O mesmo ocorre em `deleteObjectInstance`: `VALHALLA_ERR_NOT_OWNER` lança `AttributeNotOwned`, fora da especificação `throw(...)` da função -> **`std::terminate()`**.

### 2.7 Use-After-Free / Reuso de Ponteiro no C-ABI e Dangling Pointer no SDK Java
- **Locais:** `pkg/cabi/cabi.go:388-423`, `sdk/java/src/main/java/hla/rti1516e/ValhallaRtiAmbassador.java:22-53`
- **Problema:**
  - `valhalla_destroy_ambassador` remove o ID do registro e chama `C.free(amb)` imediatamente, mesmo que outra thread esteja dentro de `valhalla_evoke_callback` usando `ctx` e `userData`. Como `amb` é um `malloc(1)`, o alocador pode reutilizar o mesmo endereço imediatamente para outro ambassador.
  - Em Java, `ValhallaRtiAmbassador.close()` destrói `this.amb`, mas **nenhum método da classe verifica `destroyed.get()`** antes de chamar a biblioteca nativa. Uma chamada após `close()` pode operar silenciosamente sobre outro ambassador que herdou o mesmo endereço de memória.

### 2.8 `stateTree` de Merkle e Metadados Sujos após `Resign` / `Disconnect` no LRC
- **Local:** `pkg/lrc/rti_ambassador.go:60-76, 343-368`, `pkg/lrc/rti_amb_time_callbacks.go:55-71`
- **Problema:** `Disconnect()` não limpa `stateTree`, `lastDesyncErr`, `federationName`, `federateName`, `lookahead` nem `fedAmb`. `ResignFederationExecution()` zera **apenas** `federateID = 0`, deixando `federationID`, nomes, `lookahead` e `stateTree` intactos. Reentrar em outra federação herda hashes antigos e gera falsos `ErrStateDesyncDetected`.

### 2.9 Corrupção de Stream TCP no Cliente Portico (`TCPClient.SendPacket`)
- **Local:** `pkg/crc/portico/transport/tcp_client.go:183-204`
- **Problema:** `MagicTCPBundle` (`0xCAFE`) e `bundleLength` são escritos no `writer` (L187-194) **antes** de `hdr.MarshalTo` validar `PayloadLength <= 0x00FFFFFF` (L198). Se `MarshalTo` falhar, 8 bytes órfãos ficam no buffer e corrompem permanentemente o enquadramento TCP da sessão. Além disso, `Connect(timeout)` não aplica deadline durante `performHandshake()`, podendo travar indefinidamente segurando `c.stateMu.Lock()`.

---

## 3. Mentiras, Enganações, Stubs, Mocks e Vaporware

### 3.1 `isAttributeOwnedByFederate` Mente nos SDKs C++ e Java
- **Locais:** `sdk/cpp/src/ValhallaRTIambassador.cpp:533-553`, `sdk/java/src/main/java/hla/rti1516e/ValhallaRtiAmbassador.java:365-371`
- **Evidência:** Ambos chamam `valhalla_query_ownership(..., &owner, &owned)`, **ignoram `owner`** e retornam apenas `owned` (que indica se *qualquer* federado possui o atributo, não o federado chamador). Se o Federado B for dono do atributo, o Federado A recebe `true`.

### 3.2 `BuildErrorResponsePayload` Descarta Silenciosamente a Mensagem de Erro no Fio (Contradiz `TECHNICAL-MEMORY.md:831`)
- **Locais:** `pkg/crc/portico/wire/templates.go:86-96`, `pkg/crc/portico/wire/metamodel/portico_response_message.go:13-24`
- **Evidência:** `templates.go:89` grava `resp.Set("exception", errMsg)`, mas `ClassDescResponseMessage.CustomWriter` só serializa 1 byte (`isError`) e nunca escreve `"exception"`. O payload de erro tem os exatos mesmos 83 bytes do payload de sucesso — bug mascarado pelo próprio teste `templates_test.go:37-40`.

### 3.3 Descasamento Sistemático de Campo (`"immediate"` vs `"isAsync"`) em 11 Mensagens Portico
- **Locais:** `pkg/crc/portico/wire/metamodel/portico_message.go:48` vs 11 arquivos (`portico_join_federation.go:26`, `portico_announce_sync_point.go:24`, `portico_federation_synchronized.go:22`, `portico_register_sync_point.go:23`, `portico_resign_federation.go:28`, `portico_rti_probe.go:20`, `portico_sync_point_achieved.go:22`, `portico_time_advance_grant.go:35`, `portico_register_object.go:284`, `portico_update_attributes.go:50`, `portico_interaction.go:50`).
- **Impacto:** O serializador lê `obj.Get("isAsync")`, mas os 11 construtores setam `"immediate"`. O byte 21 do cabeçalho Portico é **sempre serializado como `0x00`**.

### 3.4 Publicação *Write-Only* (Sem Validação Real de Publish — Violação HLA Cláusula 5/6)
- **Local:** `pkg/crc/core/interest_matcher.go:11, 31, 37, 40, 42`
- **Problema:** `objPublishers`, `interactionPublishers`, `fedObjPubs` e `fedInteractionPubs` são preenchidos e limpos, mas **nunca lidos**. `ErrNotPublished` nunca é usado. Qualquer federado registra objetos, atualiza atributos e envia interações sem nunca ter publicado.

### 3.5 Comentário Falso e Validação Quebrada de Privilégio de Exclusão
- **Local:** `pkg/crc/models.go:424-428`
- **Problema:** O comentário afirma *"Strict IEEE 1516-2025 standard validation: Only registering owner OR federate holding delete privilege can delete instance"*, mas o código faz apenas `if obj.OwnerID != federateID` e **jamais consulta `f.ownershipMgr`** para `HLAprivilegeToDeleteObjectHandle`.

### 3.6 Bypass de Ownership em Atributos Não-Possuídos / Divested
- **Local:** `pkg/crc/domain_service.go:132-138`
- **Problema:** Checa `if owner, isOwned := ...; isOwned && owner != senderID`. Se o atributo estiver sem dono (`!isOwned`), **qualquer federado** consegue atualizá-lo livremente.

### 3.7 DDM Quebrado na Prática: `CreateRegion` Ignora Dimensões e `SetRegionRangeBounds` Usa Handles `0..3`
- **Locais:** `pkg/crc/native_dispatch_ddm_omt.go:20-27`, `pkg/crc/models.go:477-496`, `pkg/crc/core/omt_registry.go:144`
- **Problema:** `CreateRegion` descarta `req.DimensionHandles`. `SetRegionRangeBounds` faz `switch dimHandle { case 1, 0: X; case 2: Y; case 3: Z }`, enquanto handles reais de dimensão na OMT começam em **`1001`**. Passar qualquer `DimensionHandle` legítimo da OMT silenciosamente não faz nada e retorna `nil`. Além disso, `DiscoverObject` e `RemoveObjectInstance` (`domain_service.go:18-42, 88-111`) ignoram assinaturas e regiões DDM, fazendo broadcast para **todos** os federados.

### 3.8 Parâmetros Normativos IEEE 1516e Ignorados Silenciosamente nos SDKs e LRC
- `ResignAction` hardcoded como `1` em `pkg/lrc/rti_ambassador.go:343-355` -> ignorado nos SDKs C++ (`ValhallaRTIambassador.cpp:278-287`) e Java (`ValhallaRtiAmbassador.java:257-260`).
- `FederateType` hardcoded como `"native-hla4"` em `pkg/lrc/rti_ambassador.go:304` -> ignorado junto com `additionalFomModules` no C++ (`ValhallaRTIambassador.cpp:258-276`).
- `CallbackModel` ignorado no `connect()` de C++ (`L157-181`), Java (`L173-181`) e Web (`valhalla-hla.ts:7-15`, enums exportados e nunca usados).
- `userSuppliedTag` ignorado em `attributeOwnershipAcquisition` (C++ `L508-531`, Java `L358-362`).
- `minimumTime` ignorado em Java `evokeMultipleCallbacks` (`L448-469`).
- **5 de 9 callbacks do C-ABI não ligados** nos SDKs C++ (`L100-107`) e Java (`L126-131`): `onReceiveInteraction`, `onAnnounceSynchronizationPoint`, `onFederationSynchronized`, `onSyncPointRegistrationSucceeded`, `onSyncPointRegistrationFailed` são deixados como `NULL`.
- `OrderType::RECEIVE` e `HLA_RELIABLE` hardcoded em todos os callbacks de C++ e Java.
- `hasTime`, `timestamp` e `merkleRoot` descartados (`_`) na saída de `PorticoSession` (`portico_session.go:345,357,377`) e `WebSession` (`web_session.go:59,180-182,250,265`), e na entrada de Portico (`portico_dispatchers.go:528,582`) e Web (`web_dispatchers.go:329,357`).

### 3.9 Gambiarras no Fio Portico (`ParseWelcomePack`, `ExtractErrorMessage`, `SendInteraction` com `ClassDescObject == nil`)
- `pkg/crc/portico/wire/jos_portico.go:59-116`: `ParseWelcomePack` usa `bytes.Index(payload, []byte("WelcomePack"))` + offset mágico hardcoded `+ 8 + 1 + 2 + 1 + 1 + 1 + 1` em vez de usar `JOSDecoder`.
- `pkg/crc/portico/wire/jos_decoder.go:677`: `DeserializePorticoControlMessage` caso `255` (`SendInteraction`) instancia `NewJavaObject(metamodel.ClassDescObject)`, sendo que `ClassDescObject` é **`nil`** (`java_lang_object.go:4`).
- `pkg/crc/portico/wire/fom_synthesizer.go`: hardcoda `version="IEEE1516e"`, `order="TIMESTAMP"`, `transport="RELIABLE"`, `sharing="NEITHER"` e deixa `datatypes` e `spaces` vazios.
- `pkg/crc/fom_service.go:117-123` e `pkg/crc/portico_dispatchers.go:61-83, 156-174`: nomes hardcoded de federações de demo/teste (`ExampleFederation`, `ValhallaAdvancedFed`, `ValhallaHomologFederation`) em código de produção do CRC.

### 3.10 Falsidades e Contradições na Documentação
1. **"SIMD Bitset" (`BENCHMARK.md:45`)**: `pkg/crc/core/bitset.go:88-117` é um loop `for` escalar puro em Go, sem nenhuma instrução SIMD ou assembly.
2. **"Zero-Copy C-ABI" (`README.md:25`, `docs/project-master-plan.md:6`)**: desmentido pelo próprio `TECHNICAL-MEMORY.md:548` e por `pkg/cabi/cabi.go:220-271`, que aloca via `C.malloc`/`C.CBytes` em cada callback.
3. **Comparação de Benchmark Injusta e Inconsistente (`BENCHMARK.md` vs `README.md`)**: compara latência **in-process em loopback único** contra números **multi-processo em LAN** de Gütlein et al. (2020), apresenta números contraditórios dentro do mesmo arquivo (`89.67 µs` / `11,146/s` vs `81.46 µs` / `14,660/s`) e possui uma cópia 100% duplicada em `docs/benchmark.md`.
4. **`docs/portico-wire-spec.md` errado apesar de "Certified & Audited"**: lista IDs de `MessageType` totalmente incorretos (`CREATE_FEDERATION = 1`, `REGISTER_OBJECT = 14`, `UPDATE_ATTRIBUTES = 16`), contradizendo `header_test.go:44` e `TECHNICAL-MEMORY.md:381` (`3`, `60`, `254`).
5. **Resolução Morton contraditória**: `~6 m` e "precisão milimétrica" (`docs/valhalla-core-technologies.md:44,138`, `docs/project-master-plan.md:179,187`) vs `~9.53 m` (`TECHNICAL-MEMORY.md:285,469`, `morton_test.go:78`).
6. **"Complete English Normalization" (`TECHNICAL-MEMORY.md:675`)**: falso — dezenas de arquivos de teste e comentários em `pkg/` permanecem em português.
7. **Exemplos enganosos (`examples/web-federate/index.html`, `cpp-federate`, `java-federate`)**: o web-federate omite a checagem de `afterAcquire` no Stage 7 (L419-421), atualiza coordenadas do radar a partir do array local em vez das reflexões RTI (L378, 394-395) e tem 3 `catch (e) {}` vazios; C++ e Java ignoram falha no Stage 10 (`destroyFederationExecution`) e ainda imprimem `ALL 11 VERIFICATION STAGES SUCCEEDED`.

---

## 4. Erros Não Tratados, Engolidos e Mapeamento Quebrado

### 4.1 Erros Engolidos em Caminhos Críticos do CRC e LRC
- **Erros de `MarshalBinary()` descartados em respostas Native** (envia payload vazio/corrompido com `SuccessOK`): `pkg/crc/native_dispatchers.go:115`, `native_dispatch_ddm_omt.go:28, 269, 306, 343`, `native_dispatch_object.go:307`, `pkg/lrc/rti_ambassador.go:355`.
- **Erros de enfileiramento TSO engolidos (`pkg/crc/domain_service.go:217-218, 291-292`)**: violação de lookahead (`ErrInvalidLogicalTime`) gera apenas log de warning e retorna `nil` — o emissor recebe `SuccessOK` enquanto o evento TSO é silenciosamente descartado.
- **Portico retornando `OK` em erros e mentindo sobre a causa (`pkg/crc/portico_dispatchers.go`)**:
  - `MsgDeleteObject` (L472-494), `MsgPublish/Subscribe*` (L336-425) e `MsgDestroyFederation` (L214-228) retornam `SendControlResponseOK` quando desserialização falha ou handle é inválido.
  - `MsgDestroyFederation` (L217) traduz **qualquer** erro (inclusive `ErrFederatesCurrentlyJoined`) para `"FederationExecutionDoesNotExist"`.
  - `MsgUpdateAttributes` (L529) traduz **qualquer** erro (inclusive `ErrNotAttributeOwner`) para `"ObjectInstanceNotKnown"`.
  - `web_dispatchers.go:113-116` traduz **qualquer** erro de `Join` para `"FederateNameAlreadyInUse"`.
- **Todos os erros de `UnmarshalBinary` de callbacks silenciados no LRC (`pkg/lrc/rti_amb_time_callbacks.go:395-484`)**: blocos `if err == nil` descartam frames corrompidos sem log, métrica ou erro.
- **6 builders em `pkg/crc/portico/wire/templates.go`** (L11-14, 37-40, 91-94, 103-106, 115-118, 137-140) engolem erros de serialização e retornam `nil`.
- **Exceções silenciadas nos SDKs**: `catch (...) {}` vazios nos 4 trampolins C++ (`ValhallaRTIambassador.cpp:44-46, 59-60, 79-80, 89-90`); `valhalla-hla.ts:389-391` descarta callbacks acima de 4096 sem aviso; `valhalla-hla.ts:383-387` sem `try/catch` em `dispatchCallback` deixa Promises penduradas até timeout.
- **Mapeamento errado no C-ABI (`pkg/cabi/cabi.go:79-83`)**: mapeia tanto `ErrFederateAlreadyExecutionMember` quanto `ErrFederateNotExecutionMember` para `VALHALLA_ERR_NOT_MEMBER`.
- **Demais descartes `_ =` relevantes**: `pkg/crc/fom_service.go:61, 119, 122`, `pkg/crc/server.go:238-251` (`WebGateway.Start()` engolido), `pkg/crc/native_dispatch_time.go:44, 123` (`TimeManager.GetStatus()` engolido), `pkg/crc/portico/wire/metamodel/portico_register_object.go:129-132, 256-258`, `portico_welcome_pack.go:83,87,90`, `pkg/common/ssz.go:90-155` (decodificadores primitivos retornam `0` silenciosamente em slice curto).

---

## 5. Violações Arquiteturais, Classes/Structs "God" e Invasão de Domínio

### 5.1 `FederationExecution` + `domain_service.go`: God Struct e Vazamento de Transporte
- **Locais:** `pkg/crc/models.go` (439 linhas), `pkg/crc/domain_service.go` (343 linhas)
- **Problema:** `FederationExecution` concentra 16 campos (membros, sync points, objetos, regiões DDM, OMT, roteador, Merkle tree, TimeManager, OwnershipManager) e `domain_service.go` executa I/O de rede direto (`fed.Session.SendNotification`) além de serializar structs SSZ do protocolo Native.
- **Inversão de dependência quebrada:** `dispatchTimeGrants` está definida na camada de transporte Native (`pkg/crc/native_handler.go:98-120`), mas é chamada diretamente por `models.go:267`, `domain_service.go:220,294`, `server.go:306`, `portico_dispatchers.go:686` e `web_dispatchers.go:488`.

### 5.2 Vazamento do Protocolo Native para Dentro de `pkg/crc/core`
- **Local:** `pkg/crc/core/time_manager.go:48-56`
- **Problema:** `core.TSOMessage` armazena `MsgType uint8` (opcode Native) e `Payload []byte` (SSZ Native). O motor de tempo, que deveria ser agnóstico de protocolo, força todos os clientes Portico e Web a fazerem round-trip SSZ na entrega de eventos TSO.

### 5.3 `Server` God Struct e Mistura de Responsabilidades
- **Local:** `pkg/crc/server.go` (444+ linhas)
- **Problema:** Mistura 3 listeners TCP, registro de conexões, multiplexação de protocolo, ciclo de vida do `WebGateway` e o watchdog anti-stall de HLA Time Management (`checkStalledFederates` manipulando `fed.TimeManager().EvictStalledFederates` e despachando grants).

### 5.4 Dispatchers e SDKs Monolíticos + Deriva de Paridade
- `pkg/crc/portico_dispatchers.go` (771 linhas) e `pkg/crc/web_dispatchers.go` (448 linhas) misturam parsing de fio, resolução de arquivos FOM em disco, regras de domínio e montagem de respostas.
- `sdk/cpp/src/ValhallaRTIambassador.cpp` (686+ linhas), `sdk/java/.../ValhallaRtiAmbassador.java` (418+ linhas) e `sdk/web/src/valhalla-hla.ts` (375+ linhas) são God Classes monolíticas com forte deriva de paridade:
  - C-ABI não expõe 12 métodos já existentes em `pkg/lrc/interfaces.go` (Merkle proof/verify, queue status, `EvokeMultipleCallbacks`, `FlushQueueRequest`, `DisableTime*`).
  - SDKs C++ e Java não expõem **nenhum** método de Interaction Management nem de Synchronization Points, embora o C-ABI já os exporte.
  - Portas padrão inconsistentes entre SDKs: C++ (`52295`), Java (`4000`), Web (`52296`).
  - `sdk/c/include/valhalla_lrc.h:2` usa `#include "../../../include/valhalla_lrc.h"`, quebrando distribuição standalone do SDK C.
  - Binários compilados grandes (~11 MB+) commitados diretamente no controle de versão (`sdk/c/lib/*`, `sdk/cpp/librti1516e.a`, `sdk/java/src/main/resources/libs/*`, `examples/cpp-federate/*.{exe,dll}`).

### 5.5 Quebra de Encapsulamento e Estado Global Mutável
- `FederationExecution` e `OMTRegistry` (`GetObjectClass`, `GetObjectInstance`, `GetAllObjects`, `GetFederate`, `GetAllFederates`) retornam ponteiros crus para structs internos mutáveis após liberar `RWMutex`.
- `pkg/common/ssz.go:26`: `var Endian = binary.LittleEndian` é uma variável global exportada mutável.
- `pkg/logger/logger.go:171-189, 277-280`: goroutine global iniciada em `init()` sem shutdown; `configAccessor` sobrescreve silenciosamente `l.SetLevel()`.

---

## 6. Concorrência, Data Races, Gargalos de Desempenho e Vazamentos

### 6.1 Data Races Confirmados em Código de Produção
1. **`PorticoSession.id` e `fedHandle` (`pkg/crc/portico_session.go`)**: escritos com `atomic.StoreInt32` (L40, L44), mas lidos **sem atomic** nas linhas 94, 101, 117, 122 e 251.
2. **`OMTRegistry` iteração vs escrita concorrente (`pkg/crc/models.go:361-372` vs `pkg/crc/portico_dispatchers.go:136`)**: `RegisterObjectInstance` itera `cls.Attributes` sem lock após `GetObjectClass` liberar `omt.mu`, enquanto `ingestPorticoFOMModules` (no Join) escreve em `class.Attributes` -> **risco de fatal panic `concurrent map iteration and map write`**.
3. **`Server.peekTimeout` (`pkg/crc/server.go:141` vs `346`)**: lido sem mutex em `handleConnection` enquanto escrito sob `s.mu` em `SetPeekTimeout`.
4. **`ObjectInstance` escape de ponteiro (`pkg/crc/models.go:445-462` vs `381-412`)**: leitores acessam `obj.Attributes`/`MerkleRoot` sem lock enquanto `UpdateObjectAttributes` os muta.
5. **`pkg/logger/logger.go:113, 180-186, 310-315`**: cada instância de `valhallaLogger` usa seu próprio `outMu`, causando race quando dois loggers compartilham o mesmo `io.Writer` (ex.: `os.Stdout` ou `bytes.Buffer`).
6. **`SpatialRegion.ContainsMorton` (`pkg/crc/core/morton.go:206-209`)**: muta `r.boundsComputed` e `r.partMin*` durante leitura.
7. **`sdk/cpp/src/ValhallaRTIambassador.h:9, 337-339`**: inclui `<mutex>`, mas não possui mutex protegendo `m_connected` nem `m_fedAmb`.

### 6.2 I/O Bloqueante sob Locks e Head-of-Line Blocking
- **`RTIambassadorImpl.mu` segurado durante I/O de rede de até 5s (`pkg/lrc/rti_ambassador.go`, `rti_amb_*.go`)**: `a.mu.Lock()` (Join, Resign, Enable/Disable Time) e `a.mu.RLock()` (quase todos os demais métodos) ficam presos durante `SendRequest(..., 5*time.Second)`, bloqueando `EvokeCallback`, `UpdateAttributeValues` e `Disconnect`.
- **`FederationManager.mu.Lock()` segurado durante `filepath.WalkDir` + leitura de disco + XML parsing (`pkg/crc/federation_manager.go:29-53`)**: trava criação/busca de todas as federações no CRC.
- **Ausência de write deadline em `WebSession` (`pkg/crc/web_session.go:189, 205, 314-332`)**: fan-out síncrono na goroutine do emissor e no watchdog anti-stall. Um único cliente WebSocket lento ou travado congela emissores, o watchdog e `WebGateway.Stop()` (que chama `s.Close()` segurando `g.mu.Lock()`).
- **Ausência de read deadline em handshakes**: `server.go:474` (`acceptDedicatedNativeLoop` `br.Peek(4)`) e `portico_handler.go:186` (`performPorticoHandshake`) — vulnerável a Slowloris.

### 6.3 Gargalos Algorítmicos, Alocações no Hot Path e Vazamentos de Memória
- **`TimeManager.evaluateAdvancementsLocked` (`pkg/crc/core/time_manager.go:404-565`)**: complexidade pior caso **$O(N^3)$** sob `mu.Lock()`, alocando e ordenando `handles` a cada avaliação.
- **`OMTRegistry` (`pkg/crc/core/omt_registry.go:246, 304`)**: **$O(N^2)$** ao carregar FOMs (recalcula DFS da árvore inteira a cada inserção de classe).
- **`OwnershipManager` (`pkg/crc/core/ownership.go:105-132`)**: `DeleteObjectOwnership` e `RemoveFederate` varrem os mapas inteiros da federação sob lock.
- **Hot path de `UpdateAndRouteAttributes` (`pkg/crc/domain_service.go:141-235`)**: múltiplas alocações de mapas/slices/bitsets por update + iteração **não-determinística** em `for subID := range targetSubs` (L198).
- **Double-write syscalls sem buffer**: `native_session.go:70-77` e `web_session.go:327-330` fazem duas chamadas `Write()` separadas (header + payload) direto no `net.Conn`.
- **Polling com `Thread.sleep(5)` e relógio não-monotônico** em Java `evokeMultipleCallbacks` (`ValhallaRtiAmbassador.java:448-469`) e **`time.Sleep(10ms)` com `sync.Cond` morto** em `cmd/valhalla-federate/main.go:25-84`.
- **Vazamentos de memória / crescimento ilimitado**:
  - `InterestMatcher`: `fedInteractionPubs`/`fedInteractionSubs` crescem sem deduplicação (`interest_matcher.go:194, 235`); assinaturas passivas (`active=false`) nunca são removidas; `SubscribeObjectClassWithRegion` sem deduplicação nem unsubscribe.
  - `FederationExecution`: `regions` sem `DeleteRegion` nem limpeza no `Resign`; `syncPoints` nunca expurgados.
  - `TimeManager.tsoQueues`: sem limite de tamanho (OOM por flood de eventos futuros).
  - Double-join na mesma conexão (`native_dispatchers.go:101-113`, `portico_dispatchers.go:141-153`, `web_dispatchers.go:113-121`) sobrescreve `ctx.federateID` sem dar resign, deixando federados fantasmas permanentes.
  - `pkg/lrc/client.go:253-258`: `buffer = buffer[1:]` retém backing array sob tráfego contínuo.

### 6.4 Não-Determinismo e Overflows
- **Não-determinismo no `ClassDescHashMap.CustomWriter` (`pkg/crc/portico/wire/metamodel/java_util_hashmap.go:63` + `types.go:133-137`)**: ordena chaves usando `fmt.Sprintf("%T:%v", k, k)`, que chama `JavaObject.String()` — este itera `o.Fields` (mapa Go) em ordem aleatória.
- **Overflows de `uint32` sem checagem** nos métodos `MarshalBinary()` em `pkg/common/payloads*.go` e `ssz.go`, além de `(limit + 31) / 32` em `HashTreeRootByteList` (`ssz.go:762`) e truncamento `uint16(len(s))` em `jos_encoder.go:163`.
- **Java `RangeBounds`**: trunca `long` para `int` de 32 bits em `ValhallaRtiAmbassador.java:387`.

---

## 7. Códigos Mortos e Duplicação Severa

### 7.1 Símbolos Mortos / Write-Only Confirmados
- **Campos write-only:** `FederationStateTree.entityRoots` (`pkg/common/merkle.go:51`), `TimeManager.defaultTimeout` (`pkg/crc/core/time_manager.go:106`), `OwnershipManager.divested` (`pkg/crc/core/ownership.go:27`), `InterestMatcher.{objPublishers, interactionPublishers, fedObjPubs, fedInteractionPubs}` (`interest_matcher.go:31-42`), `isSynchronized` e `syncedCond`/`timeCond` (`cmd/valhalla-federate/main.go:24-33`).
- **Erros/Tipos/Funções nunca usados em produção:** `SSZAttributeEntry` (`ssz.go:381`), `ErrSSZInvalidSize` (`ssz.go:31`), `ErrInvalidCallType` (`header.go:202`), `ExtractErrorMessage` (`jos_decoder.go:579`), `ErrObjectInstanceNotFound` (`pkg/crc/errors.go:14`), `ErrInvalidProtocolPreamble` (`pkg/crc/errors.go:15`), `ErrNotPublished` (`interest_matcher.go:11`), `ErrAttributeUnowned` (`ownership.go:13`), `ErrCoordinateOutOfBounds` (`morton.go:26`), `VALHALLA_ERR_ALREADY_OWNED` (`include/valhalla_lrc.h:17`), `EnsureClassAndAttrInOMT` (`fom_service.go:127`), `GetOrCreateFederation` (`federation_manager.go:57`), `DomainEventSink.SendTimeAdvanceGrant` (implementado nas 3 sessions e nunca chamado!), `ModifyLookahead` / `TimeAdvanceRequestAvailable` / `NextMessageRequestAvailable` (`time_manager.go:250,275,285`), drain de callbacks pós-fechamento inacessível em `EvokeCallback` (`pkg/lrc/rti_amb_time_callbacks.go:285-295, 331-342`).
- **Comentários órfãos** de refatoração: `pkg/common/payloads.go:488`, `pkg/common/payloads_object_time.go:419`.

### 7.2 Duplicação Severa
- **Carregamento triplo de FOMs canônicos:** `IngestCanonicalDefaults` (`fom_service.go:112-124`) + re-parsing duplicado em `portico_dispatchers.go:61-83` e triplicado no Join (`156-174`).
- **~230 linhas duplicadas** entre `Encode/Size/DecodeSSZAttributeList` e `Encode/Size/DecodeSSZParameterList` (`pkg/common/ssz.go:389-625`).
- **Serialização duplicada** em `PorticoSession` e `WebSession` (caminho SSZ vs `DomainEventSink`), em `portico_register_object.go` (cabeçalho de 22 bytes duplicado 2×), em `pkg/cabi/cabi.go` (`ReflectAttributeValues` vs `ReceiveInteraction`) e nos 3 loops de `Accept()` em `server.go`.

---

## 8. Auditoria da Suíte de Testes (Inúteis, Circulares, Viesados e Frágeis)

### 8.1 Testes Inúteis, Cosméticos ou Circulares
1. **`test/c_test/main.c:6-27` (Enganação):** Vendido na documentação como *"Native C verification test federate"* que valida ciclo de vida completo, mas só chama `create` -> `disconnect` (sem nunca conectar!) -> `destroy`.
2. **`test/resources_test.go:24-69`:** Testa apenas `os.ReadFile` + `encoding/xml` da biblioteca padrão Go em uma struct local; zero código de produção exercitado.
3. **`pkg/common/payloads_test.go:10-584`:** 577 linhas de auto-roundtrip circular (`MarshalBinary` -> `UnmarshalBinary` -> `DeepEqual`) usando o próprio código sob teste; apenas 1 de ~24 structs tem vetor de bytes dourado independente.
4. **`pkg/common/ssz_adversarial_test.go:8-28`:** Asserções vazias (`if p < 1`, `root != [32]byte{}`). Uma implementação quebrada retornando `1` sempre passa.
5. **`pkg/crc/portico/wire/jos_test.go:9-122`:** Verifica apenas os 4 bytes de magic JOS e `bytes.Contains(payload, []byte("ClassName"))`.
6. **`pkg/crc/portico/wire/templates_test.go:32-40, 119-125`:** Checa apenas `len > 0` ou `len == 83`, mascarando o bug real de `BuildErrorResponsePayload` que descarta a mensagem de erro.

### 8.2 Testes Viesados, Maquiados ou Que Não Testam o Que Prometem
1. **`test/integration/time_management_test.go:45-156` (`TestIntegration_TimeManagement_LockstepAndTSO`):** Não testa **nem lockstep blocking nem TSO**. Define `getReceivedAttrs()`, mas nunca chama, nunca registra objeto e nunca envia evento TSO. Ambos federados avançam com `t=0, lookahead=1.0 -> TAR(1.0)`, concedido imediatamente sem bloqueio LBTS.
2. **`test/integration/mixed_federation_test.go:16-199` (`TestMixedFederation_ThorAndValhalla`):** Troca **zero dados** entre Thor e Valhalla. Nenhum lado assina nada; `ambValhalla` usa `NullFederateAmbassador`; ambos enviam update para handle arbitrário `562` para o vazio e descartam respostas com `_, _, _ =`.
3. **`pkg/lrc/evoke_test.go:36-58` e `pkg/lrc/evoke_benchmark_test.go:8-53`:** Medem "zero allocs" com **`fedAmb == nil`** e payload lixo (`[]byte("test")`), logo nenhum unmarshal/dispatch real acontece — testa apenas um no-op de canal.
4. **`pkg/lrc/rti_ambassador_test.go:185-381` (`TestRTIambassador_NegativeScenarios_StrictConformity`):** Ignora erros de `Connect`, `CreateFederationExecution` e `JoinFederationExecution` em 10 subtestes e só checa `if err == nil`. Se o setup falhar, todos os testes negativos passam falsamente com `ErrNotConnected`.
5. **`test/integration/cross_protocol_dispatch_test.go:110-317`:** Nunca valida o conteúdo/payload das reflexões (nem no lado Portico nem no JSON Web), apenas o tipo da mensagem, e descarta dezenas de erros com `_ =`.
6. **`test/integration/web_gateway_test.go:18-323`:** Teste de federado único que nunca verifica callbacks assíncronos (`callAction` descarta frames não correspondentes) e relaxa asserção de erro na L250 (`errCode != "ObjectInstanceNotKnown" && errCode != "UpdateAttributesFailed"`).
7. **`pkg/crc/core/time_manager_test.go:289-334` (`TestTimeManager_ConcurrencyRaceFree`):** Ignora todos os erros de `TimeAdvanceRequest` (`_, _ =`), fazendo 19 de 20 iterações falharem com `ErrInTimeAdvancingState`, e só checa `st.CurrentTime < 0`.
8. **`pkg/cabi/cabi_test.go:1`:** Oculto atrás de `//go:build cabi_test` (deixando `pkg/cabi` em 3.7% de cobertura no `go test ./...`), cheio de `time.Sleep(50ms)` e descartando status codes de `CABI_EvokeCallback`.

### 8.3 Testes Redundantes, Frágeis (`time.Sleep`) e com Bugs
- **Duplicação de testes:** `test/resources_test.go:75-201` duplica `pkg/metamodel/xml_parser_test.go:114-211`; `templates_test.go:41-65` duplica `register_object_test.go:9-36`; `jos_test.go` duplica `jos_decoder_test.go` com asserções fracas.
- **Fragilidade por `time.Sleep` e relógio de parede:** presente em `server_test.go`, `multiplexer_test.go`, `logger_adversarial_test.go`, `lrc/callback_queue_test.go`, `ddm_and_ownership_test.go`, `rti_ambassador_test.go`, `integration/callback_queue_test.go`, `merkle_desync_test.go`, `anti_stall_test.go`, `JavaFederateTest.java`.
- **Leituras TCP parciais com `conn.Read` em vez de `io.ReadFull`:** `portico_handler_test.go`, `server_test.go`, `error_handling_test.go`, `lrc/callback_queue_test.go`.
- **Benchmark viciado (`pkg/crc/core/merkle_tree_test.go:133-147`):** `BenchmarkMerkleTree_UpdateAndGetRoot_1000Entities` reenvia repetidamente o mesmo `dummyRoot` idêntico sem mutar folhas.
- **`test/integration/live_portico_test.go`:** `t.Skip` padrão, IP de LAN hardcoded (`192.168.1.226:52295`), slice panic potencial (`joinRespPayload[idxWP:idxWP+80]`) e erros rebaixados para `t.Logf`.

---

## 9. Plano de Remediação Priorizado

### Fase 1 — Estancamento de Segurança, Panics e Quebras de Build (Imediato)
1. **DoS / Panics:** Adicionar validação de teto em `jos_decoder.go:224` (`TC_BLOCKDATALONG`), limite de profundidade em `readClassDesc`, teto de `PayloadLen` em `pkg/lrc/client.go:316` e limites de contagem no C-ABI.
2. **Merkle Forgery:** Remover a confiança cega em `hdr.MerkleRoot` (`pkg/crc/models.go:395-408`) — o CRC deve sempre computar o root autoritativo e validar/rejeitar divergência se o cliente enviar um hash.
3. **SDK C++ & Java:** Corrigir escopo de `slice` em `ValhallaRTIambassador.cpp:720-731`, corrigir `NullFederateAmbassador.h` e `_Bool` -> `bool` em `valhalla_lrc.h`, corrigir `isAttributeOwnedByFederate` em C++ e Java para comparar `owner == m_federateHandle`, corrigir mapeamento de exceções para eliminar `std::terminate()` e adicionar checagem `destroyed` em Java + refcounting/handle table segura em `pkg/cabi/cabi.go`.
4. **Data Races & Deadlocks:** Corrigir leituras não-atômicas em `PorticoSession`, proteger iteração de `OMTRegistry` e `ObjectInstance`, adicionar write deadline em `WebSession` e read deadline nos handshakes, e remover locks de `RTIambassadorImpl.mu` durante `SendRequest`.

### Fase 2 — Integridade Normativa HLA-4 e Wire Portico
1. Corrigir `"immediate"` -> `"isAsync"` nas 11 mensagens Portico, `ClassDescSendInteraction` em `jos_decoder.go:677`, e serialização de `exception` em `portico_response_message.go`.
2. Implementar enforcement real de Publish (`interest_matcher.go`), Delete Privilege (`ownershipMgr`), Ownership em atributos unowned, DDM com handles OMT reais (`1001+`) e filtragem em `DiscoverObject`/`RemoveObjectInstance`.
3. Limpar `stateTree` e metadados no `Resign`/`Disconnect` do LRC e impedir double-join órfão no CRC.
4. Ligar os 5 callbacks faltantes nos SDKs C++ e Java e propagar erros reais em vez de retornar `OK` falso nos dispatchers Portico/Native/Web.

### Fase 3 — Refatoração Arquitetural, Desempenho e Honestidade da Suíte de Testes
1. Desacoplar `FederationExecution` de I/O de rede e remover `core.TSOMessage` acoplado ao opcode/SSZ Native; unificar uso de `DomainEventSink` (incluindo `SendTimeAdvanceGrant`).
2. Reescrever testes maquiados (`time_management_test.go`, `mixed_federation_test.go`, `evoke_test.go`, `rti_ambassador_test.go`, `payloads_test.go`, `test/c_test/main.c`) com vetores independentes, verificação real de payloads e sincronização determinística (eliminando `time.Sleep`).
3. Remover binários commitados, código morto e corrigir `README.md`, `BENCHMARK.md` e `docs/portico-wire-spec.md` para refletir estritamente os números empíricos e capacidades reais do sistema.
