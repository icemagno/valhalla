# Auditoria Completa de Engenharia, Arquitetura e Testes — Project Valhalla (HLA-4 RTI)

> [!CAUTION]
> Esta auditoria foi conduzida de forma **empírica e adversarial** em toda a base de código (`pkg/`, `cmd/`, `sdk/`, `include/`, `examples/`, `test/`). Todos os achados abaixo foram verificados diretamente nos arquivos-fonte, na saída do compilador/vet (`go vet ./...`) e na execução da suíte com detector de corridas (`go test -race -count=1 -cover ./...`).

---

## 1. Sumário Executivo & Linha de Base Empírica

### Verificações Automatizadas Executadas

| Verificação | Resultado | Observação Crítica |
| :--- | :--- | :--- |
| `go vet ./...` | **FALHOU (exit 1)** | **17 violações** de `possible misuse of unsafe.Pointer` em [cabi_test_bridge.go](pkg/cabi/cabi_test_bridge.go#L201-L347) — e esse arquivo é **compilado na DLL/SO de produção**. |
| `go test -race -count=1 -cover ./...` | Passou (mas mascara falhas) | Cobertura de statements crítica: `pkg/crc` (**28.0%**), `pkg/crc/portico/wire/metamodel` (**0.0%**), `pkg/cabi` (**50.5%**), `pkg/lrc` (**57.7%**). Muitos testes passam por engolir erros ou não ter asserções. |
| `git ls-files` (higiene) | **Binários versionados** | [dist/lib/librti1516e.a](dist/lib/librti1516e.a), [sdk/cpp/librti1516e.a](sdk/cpp/librti1516e.a) e JARs em `portico-3.0-homolog/` estão comitados no Git. |
| Integridade de headers C | **Triplicados** | 3 cópias idênticas de `valhalla_lrc.h` ([include/](include/valhalla_lrc.h), [pkg/cabi/](pkg/cabi/valhalla_lrc.h), [sdk/c/include/](sdk/c/include/valhalla_lrc.h)) — alto risco de *drift*. |

### Distribuição de Severidade dos Achados

- **P0 — Crítico (Segurança, Crashes, Vazamentos, Mentiras de Protocolo, DoS):**
  - Código de teste (`cabi_test_bridge.go`) exportado na DLL/SO de produção + 17 violações `go vet`.
  - OOM DoS instantâneo sem limite de alocação no WebSocket (`uint64`), Native (`uint32`) e Portico JOS (`int32`/`uint32`), além de loop infinito em `NextPowerOfTwo` e `skipCustomData`.
  - Use-After-Free no SDK C++ (`createFederationExecution`), `std::terminate()` por violação de `throw(...)` em C++14, e vazamento permanente de handles nativos no SDK Java (`valhalla_destroy_ambassador` nunca é chamado).
  - Data races em produção (`Federate`, `NativeSession`, `PorticoSession`, `WebGateway` Ping, `TCPClient`, `logger.GetGlobal`) e deadlock garantido no callback `OnStateDesyncDetected`.
  - Serviços Portico inteiros (Ownership, Time Management, Save/Restore, fallback de controle) são **stubs falsos** que retornam `OK` sem executar o serviço real.
- **P1 — Alto (Arquitetura, Domínio, Conformidade HLA-4/SSZ):**
  - Invasão de domínio: `domain_service.go` serializa frames binários SSZ do protocolo Native, forçando `PorticoSession` e `WebSession` a **deserializar SSZ e re-serializar** em JOS/JSON a cada notificação.
  - Estado temporal triplicado e divergente entre Native, Portico e Web (federados Web nunca recebem TSO).
  - `pkg/metamodel` importa diretamente `pkg/crc/core`.
  - God files/objects (`native_dispatchers.go` 1.485 linhas, `rti_ambassador.go` 1.445 linhas, `payloads.go` 1.291 linhas, `cabi.go` 1.073 linhas, `FederationExecution`).
  - LRC fabrica callbacks de *Sync Point* localmente em vez de recebê-los do CRC; `computeFederationLeaf` não segue SSZ canônico.
- **P2 — Médio (Desempenho, Escalabilidade, Qualidade de Testes):**
  - Gargalos algorítmicos: `TimeManager` $O(N^2)$/$O(N^3)$ com iteração de map não-determinística; `OMTRegistry` $O(V^2)$ ao carregar FOM; `FederationStateTree` $O(N^2)$ em inserções; locks mantidos durante I/O de rede sem *write deadline*.
  - Suíte de testes com testes sem asserções, testes circulares sem *golden vectors*, testes que engolem erros de setup, `time.Sleep` generalizado e nomes de teste enganosos.

---

## 2. Stubs, Mentiras, Enganações e Trapaças (Vaporware)

### 2.1. CRC — Protocolo Portico (O badge "Portico 3.0 100% Wire-Compatible" no [README.md](README.md#L16) é falso)

1. **Ownership Management no Portico é um stub 100% falso** — [portico_dispatchers.go](pkg/crc/portico_dispatchers.go#L784-L800):
   - `MsgAttributeDivest` e `MsgQueryOwnership` **sequer deserializam o payload**, não checam `ctx.currentFed`, nunca chamam `OwnershipManager`, mas retornam `SendControlResponseOK` e logam `"divested attribute ownership"` / `"queried attribute ownership"`.
2. **Fallback do Portico responde `OK` para QUALQUER serviço não implementado** — [portico_handler.go](pkg/crc/portico_handler.go#L152-L157):
   - Qualquer `CallTypeControlRequest` não tratado (DDM `MsgCreateRegion`, `MsgModifyLookahead`, `MsgAttributeAcquire`, `MsgReserveObjectName`, `MsgUnpublishObjectClass`, `MsgUnsubscribeObjectClass`) recebe um `SuccessResponse` vazio fingindo sucesso.
3. **Time Management no Portico ignora completamente `core.TimeManager`** — [portico_dispatchers.go](pkg/crc/portico_dispatchers.go#L590-L690):
   - `MsgEnableTimeRegulation` e `MsgEnableTimeConstrained` apenas setam booleans em `Federate` e nunca registram no `TimeManager`.
   - `MsgTimeAdvanceRequest`, `MsgNextEventRequest` e `MsgFlushQueueRequest` caem no mesmo `case`, atribuem `fed.CurrentTime = requestedTime` diretamente e devolvem `TimeAdvanceGrant` imediato — **sem LBTS, sem sincronização com pares e sem drenagem TSO**.
4. **Save/Restore no Portico é falso e dá nil-pointer panic** — [portico_dispatchers.go](pkg/crc/portico_dispatchers.go#L694-L780):
   - `MsgSaveBegun` é um no-op. `MsgSaveComplete` e `MsgRestoreComplete` disparam `MsgFederationSaved`/`MsgFederationRestored` imediatamente no **primeiro** federado sem persistir nada nem aguardar os demais. Nenhum dos 5 cases checa `ctx.currentFed == nil` (panic imediato se chamado fora da federação).
5. **Timestamp descartado no `PorticoMessage` CustomWriter** — [portico_message.go](pkg/crc/portico/wire/metamodel/portico_message.go#L38-L43):
   - Extrai `timestamp`, faz `_ = t` e escreve 8 bytes zerados literalmente (`[]byte{0, 0, 0, 0, 0, 0, 0, 0}`). O `CustomReader` (L58-64) ignora os bytes de timestamp.
6. **`BuildErrorResponsePayload(errMsg string)` ignora `errMsg`** — [templates.go](pkg/crc/portico/wire/templates.go#L86-L93):
   - Descarta completamente a mensagem de erro e envia apenas um flag booleano de erro vazio.
7. **Handlers Portico retornam `OK` em payloads corrompidos** — [portico_dispatchers.go](pkg/crc/portico_dispatchers.go#L335-L424) e [L471-L493](pkg/crc/portico_dispatchers.go#L471-L493):
   - Em pub/sub e `MsgDeleteObject`, se a deserialização falhar ou `classHandle <= 0` / `objHandle <= 0` / `currentFed == nil`, o código cai no final do bloco e retorna `SendControlResponseOK`.
8. **Cheats de nomes hardcoded no motor do CRC** — [fom_service.go](pkg/crc/fom_service.go#L96-L102), [portico_dispatchers.go](pkg/crc/portico_dispatchers.go#L60-L82), [templates.go](pkg/crc/portico/wire/templates.go#L21):
   - Regras hardcoded para `"ExampleFederation"`, `"ValhallaAdvancedFed"`, `"ValhallaHomologFederation"`, `"Soda"`, `"TacticalEntity"`, `"Thor"`.

### 2.2. CRC — Web Gateway & WebSocket

9. **Web `timeAdvanceRequest` descarta TSO e ignora grants de outros federados** — [web_dispatchers.go](pkg/crc/web_dispatchers.go#L490-L497):
   - Filtra apenas `int32(gFed.Federate) == *ctx.federateID`, descartando `gFed.Messages` (TSO) e deixando todos os outros federados desbloqueados em starvation.
10. **WebSocket Ping responde com Frame de Texto (não Pong)** — [web_gateway.go](pkg/crc/web_gateway.go#L224-L228):
    - Ao receber `opcode == 9` (Ping), envia `writeWebSocketTextFrame` (`0x81`) em vez de Pong (`0x8A`), corrompendo o protocolo RFC 6455.

### 2.3. LRC, Common & CLI

11. **LRC fabrica callbacks de Sync Point localmente** — [rti_ambassador.go](pkg/lrc/rti_ambassador.go#L393-L416):
    - `RegisterFederationSynchronizationPoint` injeta `NativeMsgSyncPointRegSucceeded`/`Failed` artificialmente na fila local via `EnqueueCallback` em vez de receber callbacks reais do CRC (inclusive injeta falha mesmo quando a conexão já morreu).
12. **Backdoor de teste compilado em produção no LRC** — [rti_ambassador.go](pkg/lrc/rti_ambassador.go#L1640-L1644):
    - `CorruptLocalStateForTesting` é um método público exportado no pacote de produção `pkg/lrc`.
13. **Atalhos hardcoded que quebram o contrato HLA-4 no LRC** — [rti_ambassador.go](pkg/lrc/rti_ambassador.go#L305) (`FederateType: "native-hla4"`), [L355](pkg/lrc/rti_ambassador.go#L355) (`ResignAction: 1`), [L583/640](pkg/lrc/rti_ambassador.go#L583) (`Active: true`), [L780/836](pkg/lrc/rti_ambassador.go#L780) (`HasTime: false` — **não existe API pública no LRC para enviar UAV ou Interaction com timestamp TSO**, mesmo com payloads e Time Management presentes!).
14. **`computeFederationLeaf` viola SSZ canônico** — [merkle.go](pkg/common/merkle.go#L74-L81):
    - Empacota `handle` (4B) + `root` (32B) + 28B zeros em `[64]byte` e aplica SHA-256 direto, em vez de `HashPair(HashTreeRootUint32(handle), root)` como faz `ComputeAttributeLeaf`.
15. **`cmd/valhalla-federate` é um exemplo quebrado que finge funcionar** — [main.go](cmd/valhalla-federate/main.go#L114-L177):
    - Entra na federação sem criá-la, registra classe `1` sem publicar, envia atributos hardcoded `562`/`551` ignorando erros (`_ = amb.UpdateAttributeValues(...)`) e loga `"attributes updated"` mesmo falhando, envia `TimeAdvanceRequest` sem habilitar regulação/restrição e nunca aguarda `TimeAdvanceGrant`.

### 2.4. SDKs (C++, Java, Web/TS) & Exemplos

16. **C++ `evokeMultipleCallbacks` é um stub** — [ValhallaRTIambassador.cpp](sdk/cpp/src/ValhallaRTIambassador.cpp#L673-L679):
    - Ignora completamente `maximumTime` e chama `evokeCallback(minimumTime)` uma única vez.
17. **Web/TS `evokeCallback` é um stub literal** — [valhalla-hla.ts](sdk/web/src/valhalla-hla.ts#L285-L287):
    - `public async evokeCallback(timeoutSeconds = 0.05): Promise<boolean> { return true; }`.
18. **Parâmetros HLA 1516e silenciosamente ignorados nos SDKs C++ e Java**:
    - C++ ignora `callbackModel`, `federateType`, `additionalFomModules`, `resignAction`, `theUserSuppliedTag`; hardcoda `OrderType::RECEIVE` e `HLA_RELIABLE` nos callbacks ([ValhallaRTIambassador.cpp](sdk/cpp/src/ValhallaRTIambassador.cpp#L52-L68)).
    - Java ignora `callbackModel`, `resignAction`, `userSuppliedTag`, `minimumTime` em `evokeMultipleCallbacks`, e `name` em `RtiFactoryFactory.getRtiFactory(name)` ([ValhallaRtiAmbassador.java](sdk/java/src/main/java/hla/rti1516e/ValhallaRtiAmbassador.java#L149-L426)).
    - Ambos os SDKs (C++ e Java) só conectam **4 dos 9 callbacks** da C-ABI (interações, sync points e unpublish/unsubscribe inexistem nos SDKs!).
    - Porta padrão divergente: Java usa `127.0.0.1:4000`, C++ usa `127.0.0.1:52295`.
19. **Teste de Homologação Web mente sobre aprovação** — [examples/web-federate/index.html](examples/web-federate/index.html#L395-L407):
    - O Estágio 7 apenas faz `log(...)` dos valores de ownership sem nenhum `assert`, imprimindo `"ALL 11 VERIFICATION STAGES SUCCEEDED"` mesmo se ownership falhar.

---

## 3. Erros Não Tratados, Engolidos e Mascarados

| Local | Código / Problema | Impacto |
| :--- | :--- | :--- |
| [models.go](pkg/crc/models.go#L249-L251) | `f.timeMgr.UnregisterFederate(...)` descarta `[]TimeGrantEvent` | **Deadlock em Time Management:** quando um federado regulador sai/cai, federados restritos desbloqueados nunca recebem `TAG`. |
| [federation_manager.go](pkg/crc/federation_manager.go#L44), [fom_service.go](pkg/crc/fom_service.go#L98-L101) | `_ = IngestFOMIntoFederation(...)` | FOMs XML inválidos ou inexistentes falham silenciosamente na criação da federação. |
| [xml_parser.go](pkg/metamodel/xml_parser.go#L95) e [L201-L204](pkg/metamodel/xml_parser.go#L201-L204) | `_, _ = omt.RegisterDimension(...)` e `_ = err` em `AddParameter` | Erros de esquema FOM são engolidos silenciosamente. |
| [native_dispatchers.go](pkg/crc/native_dispatchers.go#L117) (+ L360, 845, 909, 987, 1169, 1410, 1447, 1484) | `MarshalBinary()` / `GetStatus()` com `_` | Payloads vazios/corrompidos enviados se serialização falhar. |
| [templates.go](pkg/crc/portico/wire/templates.go#L11-L137) | `if err != nil { return nil }` | Transmite respostas Portico de 0 bytes com status `OK`. |
| [portico_dispatchers.go](pkg/crc/portico_dispatchers.go#L216), [L482](pkg/crc/portico_dispatchers.go#L482), [L528](pkg/crc/portico_dispatchers.go#L528) | Mapeia **todo** erro para uma única string fixa | `ErrFederatesCurrentlyJoined` vira `FederationExecutionDoesNotExist`; `ErrNotAttributeOwner` vira `ObjectInstanceNotKnown`. |
| [web_dispatchers.go](pkg/crc/web_dispatchers.go#L86-L90) | `strings.Contains(err.Error(), "not found")` | `ErrFederationExecutionDoesNotExist` não contém `"not found"`, retornando falsamente `"FederatesCurrentlyJoined"`. |
| [rti_ambassador.go](pkg/lrc/rti_ambassador.go#L1229-L1313) | `dispatchCallback` descarta falhas de `UnmarshalBinary` | Callbacks corrompidos são silenciosamente descartados ou entregues como `string(cb.Payload)` (lixo binário como nome de sync point). |
| [java_util_hashmap.go](pkg/crc/portico/wire/metamodel/java_util_hashmap.go#L86-L89) (+ `hashset`, `register_object`, `welcome_pack`) | `if err == nil && b != TC_ENDBLOCKDATA {}` vazio e `_, _ = r.ReadByte()` | Consome e descarta bytes arbitrariamente do stream JOS, dessincronizando o decoder. |
| [ssz.go](pkg/common/ssz.go#L89-L154) | `Uint*FromBytes` / `GetOffset` retornam `0` em slice truncado | Mascara bugs de truncamento de buffer sem retornar erro. |
| [ValhallaRTIambassador.cpp](sdk/cpp/src/ValhallaRTIambassador.cpp#L91), [ValhallaRtiAmbassador.java](sdk/java/src/main/java/hla/rti1516e/ValhallaRtiAmbassador.java#L111) | Retorno de `valhalla_set_callbacks` ignorado | Falha de registro de callbacks passa despercebida. |
| [ValhallaRtiAmbassador.java](sdk/java/src/main/java/hla/rti1516e/ValhallaRtiAmbassador.java#L422) | `} catch (InterruptedException ignored) {}` | Engole interrupção da thread sem restaurar `Thread.currentThread().interrupt()`. |
| [examples/web-federate/index.html](examples/web-federate/index.html#L243-L291) | 3 blocos `catch (e) {}` vazios | Erros de runtime silenciados. |

---

## 4. Códigos Mortos e Duplicações

### 4.1. Símbolos Mortos (0 referências úteis no repositório)
- **Erros sentinela nunca retornados:** `ErrObjectInstanceNotFound`, `ErrInvalidProtocolPreamble` ([pkg/crc/errors.go](pkg/crc/errors.go#L14-L15)), `ErrNotPublished` ([interest_matcher.go](pkg/crc/core/interest_matcher.go#L11)), `ErrCoordinateOutOfBounds` ([morton.go](pkg/crc/core/morton.go#L26)), `ErrAttributeUnowned` ([ownership.go](pkg/crc/core/ownership.go#L13)), `ErrInvalidCallType` ([header.go](pkg/crc/portico/wire/header.go#L202)), `ErrSSZInvalidSize` ([ssz.go](pkg/common/ssz.go#L30)), `ErrNativeInvalidMagic` e `ErrNativeInvalidVersion` ([protocol.go](pkg/common/protocol.go#L184-L185) — `ReadNativeHeader` usa `fmt.Errorf` sem envolvê-los!), `ErrInvalidMerkleProof` ([merkle.go](pkg/common/merkle.go#L16)), `VALHALLA_ERR_ALREADY_OWNED` ([valhalla_lrc.h](pkg/cabi/valhalla_lrc.h#L17)).
- **Campos write-only ou nunca lidos:** `Federate.IsAdvancing`, `Federate.TargetTime` ([models.go](pkg/crc/models.go#L46-L47)); `TimeManager.defaultTimeout` ([time_manager.go](pkg/crc/core/time_manager.go#L105)); `OwnershipManager.divested` ([ownership.go](pkg/crc/core/ownership.go#L27) — aloca e deleta, mas nunca lê!); `RTIambassadorImpl.address/federationName/federateName/lookahead` ([rti_ambassador.go](pkg/lrc/rti_ambassador.go#L18-L23)); `XMLObjectClass.Sharing` e `XMLInteractionClass.Sharing` ([xml_parser.go](pkg/metamodel/xml_parser.go#L31)); `ambCounter` ([cabi.go](pkg/cabi/cabi.go#L354)); `isSynchronized`/`syncedCond`/`timeGranted`/`timeCond` ([cmd/valhalla-federate/main.go](cmd/valhalla-federate/main.go#L22-L25)).
- **Funções/tipos/constantes sem chamadores em produção:** `EnsureClassAndAttrInOMT`, `GetOrCreateFederation`, `BuildIntegerResponsePayload`, `NativeMsgSyncPointAchieved`, `NativeMsgConnect`, `NativeMsgDisconnect`, `NativeMsgGetInteractionClassHandle*`, `NativeMsgGetParameterHandle*`, `SuccessOKPayload`, `GetInteractionClassHandlePayload`, `GetParameterHandlePayload`, `SSZAttributeEntry`, `Client.MaxQueueCapacity`, `SetConfigAccessor`, `LevelNone`, `ParseLogLevel`, `TCPClient` inteiro (apenas usado em testes), `ModifyLookahead`/`TimeAdvanceRequestAvailable`/`NextMessageRequestAvailable` (nunca expostos em nenhum dispatcher), 7 classes de exceção no SDK Java, `CallbackModel`/`OrderType` no SDK Web.
- **Código matematicamente inalcançável / copy-paste duplicado:**
  - [ssz.go](pkg/common/ssz.go#L349-L351): `if count > byteLen/4` nunca pode ser verdadeiro devido à verificação da linha 343.
  - [web_dispatchers.go](pkg/crc/web_dispatchers.go#L202-L207) (e L227-232, L252-257): mesmo bloco `if len(attrs) == 0` colado **duas vezes seguidas** em 3 handlers.

---

## 5. Quebra de Padrões de Projeto, Classes 'God' e Invasão de Domínio

### 5.1. Invasão de Domínio Grave: Dupla Serialização no `domain_service.go`
- [domain_service.go](pkg/crc/domain_service.go) deveria ser a camada de serviço de domínio independente de protocolo, mas **serializa payloads binários SSZ do protocolo Native** e envia opcodes `NativeMsg*` para `fed.Session.SendNotification(...)`.
- Consequência: [portico_session.go](pkg/crc/portico_session.go#L125-L232) e [web_session.go](pkg/crc/web_session.go#L67-L180) são forçados a **fazer Unmarshal dos bytes SSZ de volta para structs Go e re-serializar** em JOS ou JSON em cada notificação de rede! Além disso, `domain_service.go` loga usando `logCRCNative` mesmo para tráfego Portico/Web.

### 5.2. Estado Triplicado e Divergente Entre Protocolos
- `Federate` ([models.go](pkg/crc/models.go#L41-L47)) duplica o estado de tempo (`CurrentTime`, `Lookahead`, `IsRegulating`, `IsConstrained`) que já existe em `core.TimeManager`.
  - **Native** atualiza ambos.
  - **Portico** atualiza apenas `Federate` (quebrando `TimeManager`).
  - **Web** atualiza apenas `TimeManager` (deixando `Federate.IsConstrained == false`, logo [domain_service.go](pkg/crc/domain_service.go#L144) **nunca enfileira mensagens TSO para federados Web**!).
- [models.go](pkg/crc/models.go#L348-L356): `RegisterObjectInstance` só registra ownership inicial dos atributos diretos da classe, ignorando atributos herdados de ancestrais (`cls.Ancestors`).
- Catch-up `DiscoverObject` ao se inscrever só existe no Native ([native_dispatchers.go](pkg/crc/native_dispatchers.go#L357-L371)), ausente em Portico e Web.

### 5.3. God Files & God Objects
- [native_dispatchers.go](pkg/crc/native_dispatchers.go) (**1.485 linhas**): repete boilerplate de header/cheque de federação ~30 vezes.
- [rti_ambassador.go](pkg/lrc/rti_ambassador.go) (**1.445 linhas**): mistura ciclo de vida, OMT, sync points, pub/sub, objetos, tempo, ownership, DDM, evocation/timer pool, dispatch e Merkle desync num único arquivo.
- [payloads.go](pkg/common/payloads.go) (**1.291 linhas**): 5 structs idênticas para string única, pares duplicados (`DeleteObject` ≡ `RemoveObject`, `Divest` ≡ `Acquire`, `GetAttr` ≡ `GetParam`).
- [cabi.go](pkg/cabi/cabi.go) (**1.073 linhas**): mistura trampolins C, mapeamento de erros, registry, marshaling e ~40 exports num só arquivo.
- `FederationExecution` ([models.go](pkg/crc/models.go)): God Object controlando federados, sync points, objetos, regiões DDM, OMT, Router, Merkle, TimeManager e OwnershipManager sob um único `sync.RWMutex`.
- [web_gateway.go](pkg/crc/web_gateway.go#L388-L570): embute um SPA HTML/CSS/JS de 183 linhas como string Go dentro do servidor de transporte WebSocket.

### 5.4. Outras Quebras de Fronteira e Abstração
- **Acoplamento invertido:** [pkg/metamodel/xml_parser.go](pkg/metamodel/xml_parser.go#L10) importa diretamente `pkg/crc/core` e muta `*core.OMTRegistry` concreto.
- **Design quebrado da C-ABI sem `void* userData`:** [valhalla_lrc.h](pkg/cabi/valhalla_lrc.h#L31-L51) não passa `void* userData` em nenhum dos 9 callbacks, forçando o SDK C++ a usar uma variável global `thread_local ValhallaRTIambassador* t_activeAmbassador` que quebra em callbacks multi-thread ou `HLA_IMMEDIATE`.
- **Tipo não-exportado em método exportado:** [pkg/lrc/client.go](pkg/lrc/client.go#L166) expõe `EnqueueCallback(item callbackItem)` com parâmetro privado.
- **Resolução de FOM duplicada e divergente:** [portico_handler.go](pkg/crc/portico_handler.go#L198-L216) reimplementa `resolveFOMPath` com apenas 2 caminhos vs 13 em [fom_service.go](pkg/crc/fom_service.go#L16-L54).

---

## 6. Segurança de Memória (FFI), Concorrência, DoS e Gargalos de Desempenho

### 6.1. FFI / C-ABI / C++ / Java — Bugs Críticos de Memória
1. **Arquivo de teste compilado na DLL/SO de produção** — [cabi_test_bridge.go](pkg/cabi/cabi_test_bridge.go#L1-L396): sem sufixo `_test.go` nem build tag, exporta 5 funções `goBridge_*` e estado global na biblioteca de produção, além de gerar **17 erros no `go vet`**.
2. **Use-After-Free no SDK C++ (`createFederationExecution`)** — [ValhallaRTIambassador.cpp](sdk/cpp/src/ValhallaRTIambassador.cpp#L209-L214): `utfFoms.push_back(...)` sem `reserve()` realoca o `std::vector` e invalida todos os ponteiros `c_str()` já armazenados em `fomPtrs` quando há >1 módulo FOM.
3. **`std::terminate()` em runtime no SDK C++14** — [ValhallaRTIambassador.cpp](sdk/cpp/src/ValhallaRTIambassador.cpp#L266-L305) e [L661-L671](sdk/cpp/src/ValhallaRTIambassador.cpp#L661-L671): `checkStatus()` lança exceções (`ObjectClassNotDefined`, `NotConnected`) que não constam na cláusula dinâmica `throw(...)` dos métodos -> aborto imediato do processo.
4. **Exceções C++ cruzando fronteira CGO (UB)** — [ValhallaRTIambassador.cpp](sdk/cpp/src/ValhallaRTIambassador.cpp#L34-L76): nenhum trampolim `c_on*` tem `try { ... } catch (...)`. Além disso, `ValhallaRTIambassador` viola a *Rule of Three/Five* (cópia gera double-free em `m_amb`).
5. **Use-After-Free / Race em `valhalla_destroy_ambassador`** — [cabi.go](pkg/cabi/cabi.go#L358-L401): `getContext` solta `registryMu.RLock()` antes do uso; `destroy` faz `Disconnect()` sob lock global e `C.free(amb)` concorrendo com chamadas ativas. Além disso, `C.malloc` em [cabi.go](pkg/cabi/cabi.go#L221-L230) e [L294-L303](pkg/cabi/cabi.go#L294-L303) não checa retorno `nil`.
6. **Vazamento permanente de memória nativa e goroutines no SDK Java** — [ValhallaRtiAmbassador.java](sdk/java/src/main/java/hla/rti1516e/ValhallaRtiAmbassador.java#L28): `valhalla_destroy_ambassador()` **nunca é chamado em nenhum lugar do SDK Java**. Adicionalmente, `new Memory(...)` por atributo não chama `mem.close()` após a chamada síncrona, e `deleteOnExit()` em [ValhallaNativeLoader.java](sdk/java/src/main/java/valhalla/client/ValhallaNativeLoader.java#L52-L53) vaza `.dll` no `%TEMP%` no Windows.
7. **Promessas penduradas para sempre no SDK Web** — [valhalla-hla.ts](sdk/web/src/valhalla-hla.ts#L137-L153): `ws.onclose` e `disconnect()` nunca rejeitam `pendingRequests`; `sendRequest` não tem timeout.
8. **XSS no Dashboard Web e Exemplo Web** — [web_gateway.go](pkg/crc/web_gateway.go#L565) e [examples/web-federate/index.html](examples/web-federate/index.html#L218): `entities[k].name` interpolado sem escape em `el.innerHTML`.

### 6.2. Corridas de Dados (Data Races) e Deadlocks
1. **`Federate` mutado sem lock** — [native_dispatchers.go](pkg/crc/native_dispatchers.go#L906-L1020) e [portico_dispatchers.go](pkg/crc/portico_dispatchers.go#L612-L677) escrevem `IsRegulating`/`IsConstrained`/`CurrentTime` sem lock enquanto [domain_service.go](pkg/crc/domain_service.go#L144) lê concorrentemente.
2. **`NativeSession` / `PorticoSession` IDs sem sincronização** — [native_session.go](pkg/crc/native_session.go#L37-L47), [portico_session.go](pkg/crc/portico_session.go#L33-L43).
3. **WebSocket Ping escreve no socket sem `session.mu`** — [web_gateway.go](pkg/crc/web_gateway.go#L225): corrida de dados e corrupção de framing TCP.
4. **`logger.GetGlobal()` sem sincronização** — [logger.go](pkg/logger/logger.go#L250-L259): leitura sem lock concorre com escrita em `InitGlobalLogger`.
5. **Deadlock garantido em `OnStateDesyncDetected`** — [rti_ambassador.go](pkg/lrc/rti_ambassador.go#L761-L813) e [L1574-L1626](pkg/lrc/rti_ambassador.go#L1574-L1626): segura `a.mu.RLock()` durante RPC de 5s **e** durante invocação do callback do usuário; se o callback chamar `Disconnect()` ou `ResignFederationExecution()` (`a.mu.Lock()`), trava permanentemente.
6. **Roteamento corrompido de RPC expirado no `Client`** — [client.go](pkg/lrc/client.go#L296-L321): resposta síncrona (`ReqID > 0`) que chega após timeout cai no ramo assíncrono e é enfileirada em `c.inQueue` como callback! Além disso, `reqID` (`uint16`) dá wraparound sem verificar colisão em `c.pending`.
7. **`TCPClient` race + nil panic** — [tcp_client.go](pkg/crc/portico/transport/tcp_client.go#L174-L279): acessa `c.conn`/`c.writer`/`c.reader` sem `stateMu` enquanto `Close()` os zera.

### 6.3. Vetores de DoS, Overflow e Não-Determinismo
1. **OOM DoS em 1 pacote (sem limite de tamanho):**
   - WebSocket: `make([]byte, payloadLen)` com `uint64` sem teto ([web_session.go](pkg/crc/web_session.go#L276-L290)).
   - Native CRC: `make([]byte, hdr.PayloadLen)` até 4 GiB sem checar `MaxNativePayloadLen` ([native_handler.go](pkg/crc/native_handler.go#L157-L159)).
   - Portico JOS: `int32`/`uint32` sem limite ou cheque `< 0` em `TC_BLOCKDATALONG`, `readArray`, `DeserializePorticoControlMessage`, `HashMap`, `HashSet`, `ArrayList` ([jos_decoder.go](pkg/crc/portico/wire/jos_decoder.go#L137-L665), [java_util_hashmap.go](pkg/crc/portico/wire/metamodel/java_util_hashmap.go#L69-L72)) + ausência de limite de recursão (stack overflow).
2. **Loops infinitos (100% CPU hang):**
   - [ssz.go](pkg/common/ssz.go#L657-L697): `NextPowerOfTwo` entra em loop infinito para `limit > 1<<62` (overflow de `p <<= 1` para negativo) e `int(limit)` trunca `uint64` em `Merkleize`.
   - [jos_decoder.go](pkg/crc/portico/wire/jos_decoder.go#L486-L492): `skipCustomData` faz `Seek` com `int32` negativo em `TC_BLOCKDATALONG`, voltando no stream em loop infinito.
3. **Overflow silencioso de `uint32` e `byte`/`uint16`:**
   - Todos os `MarshalBinary` dinâmicos em [payloads.go](pkg/common/payloads.go) e [ssz.go](pkg/common/ssz.go) somam `uint32` sem cheque de overflow; `WriteBlockData` ([jos_encoder.go](pkg/crc/portico/wire/jos_encoder.go#L73-L77)) trunca `len(data)` para `byte` (corrompe blocos > 255B).
4. **Envenenamento por `NaN`/`Inf`:**
   - [time_manager.go](pkg/crc/core/time_manager.go#L176-L297), [rti_ambassador.go](pkg/lrc/rti_ambassador.go#L862) e [cabi.go](pkg/cabi/cabi.go#L1157) não rejeitam `math.IsNaN`/`math.IsInf` (`NaN < 0` é `false`).
5. **Não-determinismo em serialização e estado:**
   - Iteração direta de Go maps em [time_manager.go](pkg/crc/core/time_manager.go#L391-L539), [java_util_hashmap.go](pkg/crc/portico/wire/metamodel/java_util_hashmap.go#L38-L47) e [fom_synthesizer.go](pkg/crc/portico/wire/fom_synthesizer.go#L43-L166).
   - Ordenação divergente entre [merkle.go](pkg/common/merkle.go#L34-L38) (`int` com sinal) e [ssz.go](pkg/common/ssz.go#L395-L399) (`uint32` sem sinal).

### 6.4. Gargalos de Desempenho & Vazamentos
- **Locks durante I/O de rede sem *write deadline*:** `NativeSession`, `PorticoSession`, `WebSession`, `lrc.Client.writePacket` e `domain_service.go` (broadcast síncrono sequencial na goroutine do publisher) — **1 cliente lento trava toda a federação**.
- **Complexidade $O(N^2)$ / $O(N^3)$:** `TimeManager.evaluateAdvancementsLocked` ($O(N^3)$ pior caso sob lock exclusivo), `OMTRegistry.AddObjectClass` (recalcula Euler Tour inteiro a cada inserção: $O(V^2)$), `FederationStateTree.rebuildTreeLocked` ($O(N^2)$ em inserções/remoções) e recálculo de Merkle K vezes por lote em `UpdateAttributeValues` ([rti_ambassador.go](pkg/lrc/rti_ambassador.go#L796-L798)) e sob `f.mu.Lock()` no CRC ([models.go](pkg/crc/models.go#L378-L383)).
- **Crescimento ilimitado de slices:** `InterestMatcher` ([interest_matcher.go](pkg/crc/core/interest_matcher.go#L96-L310)) faz `append` sem deduplicação em `fedObjPubs`, `fedObjSubs`, `objRegionSubs`.
- **Recompute quebrado por *value receiver*:** [morton.go](pkg/crc/core/morton.go#L204-L237) chama `r.RecomputeMortonBounds()` em cópia por valor descartada.
- **Vazamento de timers:** `time.After(...)` em cada `SendRequest` ([client.go](pkg/lrc/client.go#L140)) e dentro de loop em `queueLoop` ([L268](pkg/lrc/client.go#L268)).

---

## 7. Auditoria da Suíte de Testes e Benchmarks (46 arquivos auditados)

### 7.1. Testes Inúteis / Sem Asserções / Que Não Podem Falhar
- [pkg/logger/logger_test.go](pkg/logger/logger_test.go#L48-L68) (`TestLogger_ConcurrentLogging`): **0 asserções**.
- [pkg/crc/core/time_manager_test.go](pkg/crc/core/time_manager_test.go#L289-L316) (`TestTimeManager_ConcurrencyRaceFree`): **0 asserções**, engole todos os retornos.
- [test/c_test/main.c](test/c_test/main.c#L6-L22): não verifica o código de retorno de `valhalla_disconnect` e imprime `SUCCESS` incondicionalmente.
- [pkg/crc/portico/wire/register_object_test.go](pkg/crc/portico/wire/register_object_test.go#L7-L13): só verifica `len(payload) == 0`.
- [pkg/crc/portico/wire/jos_test.go](pkg/crc/portico/wire/jos_test.go#L9-L122): valida stream binário JOS apenas com `bytes.Contains(..., className)`.
- [pkg/crc/portico/wire/welcome_pack_fom_test.go](pkg/crc/portico/wire/welcome_pack_fom_test.go#L56-L64): após roundtrip, só verifica o nome da classe raiz e ignora todo o FOM interno.
- [pkg/crc/core/ownership_test.go](pkg/crc/core/ownership_test.go#L70-L71): chama `DeleteObjectOwnership` na última linha sem nenhuma asserção posterior.

### 7.2. Testes Circulares, Com Viés ou Trapaceando Asserções
- [pkg/lrc/rti_ambassador_test.go](pkg/lrc/rti_ambassador_test.go#L125-L173): transforma falhas reais de `EvokeCallback` e `TimeAdvanceGrant` em mero `t.Logf` — o teste **passa mesmo que TAG nunca chegue**.
- [pkg/lrc/rti_ambassador_test.go](pkg/lrc/rti_ambassador_test.go#L198-L385): engole erros de `Connect`/`Create`/`Join` em quase todos os subtestes negativos; se o setup falhar, `if err == nil` passa por motivo falso.
- [pkg/common/ssz_test.go](pkg/common/ssz_test.go#L300-L322) (`TestSSZIntegerOverflowAttack`): faz `_ = p.UnmarshalBinary(malicious)` — só checa `recover()` de panic; se aceitar `0xFFFFFFF0` sem erro, o teste passa!
- **Testes 100% circulares (zero *golden vectors* independentes):** [payloads_test.go](pkg/common/payloads_test.go#L10-L564) (além de engolir erros nas linhas 48-66), [ssz_test.go](pkg/common/ssz_test.go#L324-L350) (`TestSSZMerkleization`), [morton_test.go](pkg/crc/core/morton_test.go#L8-L29), [fom_synthesizer_test.go](pkg/crc/portico/wire/fom_synthesizer_test.go#L11-L118).
- [test/integration/cross_protocol_dispatch_test.go](test/integration/cross_protocol_dispatch_test.go#L167-L226): engole dezenas de erros, usa fallback hardcoded `objHandle = 1` e não valida conteúdo de atributos refletidos.
- [test/integration/live_portico_test.go](test/integration/live_portico_test.go#L15-L168): pulado por `t.Skip`, IP de LAN hardcoded (`192.168.1.226:52295`), rebaixa erros reais para `t.Logf`.

### 7.3. Testes e Benchmarks Enganosos, Flaky ou Mal Posicionados
- **Nomes mentirosos:**
  - `TestIntegration_TimeManagement_LockstepAndTSO` ([time_management_test.go](test/integration/time_management_test.go#L45-L156)): envia **zero mensagens TSO** e não testa lockstep bloqueado.
  - `TestServer_DedicatedNativePort_ZeroLatency` ([server_test.go](pkg/crc/server_test.go#L292-L371)): **nunca mede latência**.
  - `BenchmarkHashTreeRootByteList_1MB` ([ssz_test.go](pkg/common/ssz_test.go#L377-L384)): faz hash de apenas **42 bytes**, não 1 MB.
  - `BenchmarkMerkleTree_FederationGlobalRoot` ([merkle_tree_test.go](pkg/crc/core/merkle_tree_test.go#L117-L128)): só lê cache sob `RLock` sem mutar estado.
  - `TestEvokeCallback_ZeroHeapAllocsWhenCallbackReady` ([evoke_test.go](pkg/lrc/evoke_test.go#L36-L58)): usa `federateAmb: nil`, pulando unmarshaling e dispatch reais.
- **Flakiness generalizada:** dezenas de `time.Sleep(5ms..300ms)` em 15 arquivos de teste, leituras parciais com `conn.Read` em vez de `io.ReadFull`, corrida de setup em [callback_queue_test.go](pkg/lrc/callback_queue_test.go#L228-L236), risco de deadlock global em [logger_adversarial_test.go](pkg/logger/logger_adversarial_test.go#L34-L141).
- **Duplicação e má organização:** [test/resources_test.go](test/resources_test.go#L75-L173) duplica `xml_parser_test.go`; testes de integração TCP completos moram dentro do pacote unitário `pkg/lrc/`.
- **Lacunas graves de cobertura:** **0 testes automatizados** para `sdk/cpp/`, `sdk/web/`, `sdk/java/` (apenas script manual), `cmd/*`, `pkg/common/protocol.go`, `pkg/crc/portico/wire/metamodel/` (31 arquivos), `pkg/crc/fom_service.go`, `pkg/crc/native_handler.go`, `pkg/crc/web_gateway.go` (erros/frames malformados) e entrega TSO de ponta a ponta.

---

## 8. Plano de Remediação Priorizado

### Onda 1 — Bloqueadores Críticos (P0: Segurança, Crashes, FFI & DoS)
1. **Isolar `cabi_test_bridge.go`** com build tag `//go:build cabi_test` ou mover para `_test.go` e corrigir os 17 erros de `go vet` (`unsafe.Pointer`).
2. **Corrigir limites de alocação e loops infinitos:** impor `MaxPayloadLen` no WebSocket, Native CRC e JOS decoder; checar `iLen >= 0` em `skipCustomData`; corrigir overflow em `NextPowerOfTwo`/`Merkleize`; rejeitar `NaN`/`Inf` em todos os tempos/timeouts.
3. **Corrigir bugs de memória nos SDKs:** `utfFoms.reserve(...)`, `try/catch` nos trampolins C++ e remoção/correção das `throw(...)` em C++14; adicionar `close()`/`Cleaner` chamando `valhalla_destroy_ambassador` no SDK Java; ref-counting/lock em `cabi.go` e checagem de `C.malloc == nil`.
4. **Eliminar data races e deadlocks:** sincronizar `Federate`, `NativeSession`, `PorticoSession`, `WebGateway` Ping (e corrigir opcode para Pong `0x8A`), `TCPClient`, `logger.GetGlobal()`; liberar `a.mu.RLock()` antes de invocar `OnStateDesyncDetected`; tratar grants de `UnregisterFederate` em `Resign`.
5. **Remover mentiras/stubs no Portico e Web:** ou implementar de fato Ownership, Time Management (`core.TimeManager`) e Save/Restore no Portico + TSO/grants no Web, ou retornar erro explícito `NOT IMPLEMENTED` em vez de fingir `OK`.

### Onda 2 — Arquitetura e Domínio (P1)
6. **Eliminar dupla serialização:** fazer `domain_service.go` despachar eventos tipados de domínio para `Session` (interface por evento), deixando cada protocolo (Native SSZ, Portico JOS, Web JSON) serializar uma única vez.
7. **Unificar fonte da verdade de Time Management:** remover campos duplicados de tempo em `Federate` e delegar 100% a `core.TimeManager`.
8. **Desacoplar `pkg/metamodel` de `pkg/crc/core`** via interface, e adicionar `void* userData` à C-ABI (`valhalla_lrc.h` como fonte única gerada/referenciada sem 3 cópias manuais).
9. **Quebrar God Files:** fatiar `native_dispatchers.go`, `rti_ambassador.go`, `payloads.go` e `cabi.go` por grupo de serviços HLA (Federation, Declaration, Object, Time, Ownership, DDM).

### Onda 3 — Desempenho, Testes e Higiene (P2/P3)
10. **Desempenho:** adicionar *write deadlines* / filas assíncronas por sessão para evitar *head-of-line blocking*, batching de Merkle/Euler Tour, deduplicação em `InterestMatcher` e pointer receiver em `morton.go`.
11. **Refatorar suíte de testes:** adicionar *golden vectors* (SSZ/Eth2, Morton, JOS), remover `t.Logf` mascarando falhas, substituir `time.Sleep` por sincronização determinística, corrigir nomes mentirosos e criar testes automatizados para SDKs e caminhos adversariais.
12. **Limpeza de repositório:** remover binários `.a`/`.jar` versionados do Git e remover códigos mortos listados na Seção 4.
