# Especificação Técnica do Protocolo de Rede Portico (Portico Wire Specification)

> **Projeto:** Projeto Valhalla  
> **Status:** Homologado e Auditado Diretamente do Código-Fonte do The Portico Project (Portico 2.x / 3.x - Apache 2.0 / CDDL)  
> **Arquivos de Referência:** `Header.java`, `BitHelpers.java`, `TcpChannel.java`, `Bundler.java`, `TcpClientProxy.java`, `MessageHelpers.java`

---

## 1. Visão Geral da Arquitetura de Rede do Portico

O Portico utiliza uma arquitetura baseada em pipeline de mensagens (`MessageSink`) onde todo evento de simulação (criação de federação, ingresso, publicação, registro de objeto, atualização de atributos e avanço de tempo) é encapsulado em uma instância de `PorticoMessage`.

Na camada de rede física, as mensagens trafegam através de dois canais principais:
1. **Canal de Controle (TCP Unicast):** Utilizado para conexões ponto a ponto confiáveis, handshakes de ciclo de vida (`Connect`, `Join`, `Resign`, `SyncPoints`) e respostas síncronas (`SUCCESS_RESP`, `ERROR_RESP`).
2. **Canal de Dados (UDP Multicast / TCP Mesh):** Utilizado para a distribuição em alta frequência de atualizações de atributos (`UpdateAttributes`) e interações transitórias (`SendInteraction`).

---

## 2. Enquadramento de Transporte TCP (*TCP Bundler Framing*)

No canal TCP (`TcpChannel.java` e `Bundler.java`), os dados nunca são enviados soltos no stream. Todos os pacotes trafegam empacotados em estruturas de bundle prefixadas pelo marcador mágico `0xCAFE`:

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                 Bundle Magic: 0x0000CAFE (int32)              | 4 bytes
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                 Bundle Length: N bytes (int32)                | 4 bytes
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|       Message 1: Header (12 bytes) + Payload (L1 bytes)       | 12 + L1 bytes
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|      [Opcional] Message 2: Header (12 bytes) + Payload...     | ...
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

---

## 3. Estrutura Binária Oficial do Cabeçalho de Mensagem (*Portico Header.java*)

Cada mensagem individual dentro de um bundle possui um cabeçalho fixo estrito de **12 bytes (96 bits)** em **Network Byte Order (Big-Endian)**:

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|     Flags     |                 Payload Length                |  Linha 1 (Bytes 0-3)
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
| FedID | CType |  MessageType  |     RequestId / FilteringId   |  Linha 2 (Bytes 4-7)
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|         Source Handle         |         Target Handle         |  Linha 3 (Bytes 8-11)
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                        Payload Bytes...                       |  N bytes
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

### 3.1. Descrição dos Campos com Precisão de Bits

| Byte(s) | Campo | Tamanho | Descrição e Mapeamento Portico |
| :--- | :--- | :--- | :--- |
| **0** | `Flags` | 8 bits | Máscaras de bit lidas via `BitHelpers.readBooleanBit(0..7)`:<br>• Bit 0 (`0x01`): `isBundle`<br>• Bit 1 (`0x02`): `isEncrypted` (Nonce presente)<br>• Bit 2 (`0x04`): `isAuthenticated` (Token presente)<br>• Bit 3 (`0x08`): `isManualMarshal` (Serialização binária rápida)<br>• Bit 4 (`0x10`): `isFiltering` (RequestId contém FilteringId)<br>• Bit 5 (`0x20`): `isFilteringObjectClass` (1=Objeto, 0=Interação)<br>• Bit 6 (`0x40`): Reservado Portico<br>• Bit 7 (`0x80`): **Flag de Extensão Valhalla (SSZ / KEM Ativo)** |
| **1-3** | `PayloadLength` | 24 bits | Inteiro de 24 bits Big-Endian (0 a 16.777.215 bytes / 16 MB). Exclui os 12 bytes do cabeçalho. |
| **4** | `FedID / CType` | 8 bits | • High Nibble (bits 7-4): `FederationID` (uint4, faixa 0..15)<br>• Low Nibble (bits 3-0): `CallType` (uint4, enum 0..4) |
| **5** | `MessageType` | 8 bits | Identificador do tipo de mensagem (uint8). |
| **6-7** | `RequestId / FilteringId` | 16 bits | Inteiro uint16 Big-Endian com o ID de correlação de requisição/resposta ou Handle de filtragem. |
| **8-9** | `SourceHandle` | 16 bits | ID do federado de origem (uint16 Big-Endian). |
| **10-11**| `TargetHandle` | 16 bits | ID do federado de destino (uint16 Big-Endian). |

---

## 4. Tabela de Tipos de Mensagem (*MessageType.java*)

| Código (Dec) | Código (Hex) | Nome da Mensagem | Descrição |
| :--- | :--- | :--- | :--- |
| `0` | `0x00` | `MsgConnect` | Estabelece sessão de rede do federado. |
| `1` | `0x01` | `MsgDisconnect` | Encerra sessão de rede. |
| `2` | `0x02` | `MsgRtiProbe` | Sonda de presença do executivo RTI. Responde com `"I am Spartacus!"`. |
| `3` | `0x03` | `MsgCreateFederation` | Requisição para instanciar uma execução de federação com módulos FOM. |
| `4` | `0x04` | `MsgJoinFederation` | Ingresso de um federado com nome e tipo. Retorna o handle atribuído. |
| `5` | `0x05` | `MsgResignFederation` | Saída voluntária de um federado da federação. |
| `6` | `0x06` | `MsgDestroyFederation` | Destruição da execução de federação. |
| `7` | `0x07` | `MsgListFederations` | Consulta de federações ativas. |
| `8` | `0x08` | `MsgAuthenticate` | Negociação de credenciais e segurança. |
| `14` | `0x0E` | `MsgSuccessResponse` | Resposta positiva síncrona a requisições de controle. |
| `15` | `0x0F` | `MsgErrorResponse` | Resposta com código e mensagem de erro/exceção. |
| `40` | `0x28` | `MsgRegisterSyncPoint`| Registro de ponto de sincronização. |
| `43` | `0x2B` | `MsgAchieveSyncPoint` | Anúncio de ponto de sincronização alcançado. |
| `50` | `0x32` | `MsgPublishObjectClass` | Publicação de classe de objeto. |
| `51` | `0x33` | `MsgSubscribeObjectClass` | Inscrição em classe de objeto. |
| `52` | `0x34` | `MsgRegisterObject` | Registro de instância de objeto. |
| `53` | `0x35` | `MsgDiscoverObject` | Notificação de objeto descoberto. |
| `54` | `0x36` | `MsgUpdateAttributes` | Envio de atributos de simulação (suporta ManualMarshal). |
| `55` | `0x37` | `MsgReflectAttributes`| Entrega de atualização de atributos aos assinantes. |
| `56` | `0x38` | `MsgDeleteObject` | Remoção de instância de objeto. |
| `80` | `0x50` | `MsgPublishInteractionClass` | Publicação de interação. |
| `81` | `0x51` | `MsgSubscribeInteractionClass` | Inscrição em interação. |
| `82` | `0x52` | `MsgSendInteraction` | Disparo de interação tática (suporta ManualMarshal). |
| `83` | `0x53` | `MsgReceiveInteraction`| Entrega de interação aos inscritos. |
| `100` | `0x64` | `MsgTimeAdvanceRequest` | Solicitação de avanço de tempo lógico (TAR). |
| `101` | `0x65` | `MsgTimeAdvanceGrant` | Concessão de avanço temporal (TAG). |

---

## 5. Sequência do Handshake de Conexão e Ingresso

### 5.1. Handshake no Nível de Transporte TCP (Sincronização 0xBEEF / 0xFEED)
Antes de qualquer mensagem HLA de alto nível ser trocada, a camada de transporte TCP do Portico (`TcpClientProxy.java`) executa uma negociação binária estrita de sincronização:

```
Cliente Go (Valhalla)                         Servidor Portico (rtiexec)
       │                                                   │
       │─────── 1. Conexão TCP (SYN -> ACK) ──────────────>│
       │                                                   │
       │<────── 2. Magic 0xBEEF (int32, Big-Endian) ───────│
       │<────── 3. Tamanho da Mensagem N (int32) ──────────│
       │<────── 4. Mensagem ASCII ("Portico Router...") ───│
       │                                                   │
       │<────── 5. Código READY: 0xFEED (int32) ───────────│
       │─────── 6. Acknowledgment READY: 0xFEED (int32) ──>│
       │                                                   │
       │        [ Canal TCP Estabelecido e Ativo ]         │
```

* **Magic de Boas-Vindas:** `0x0000BEEF` (`int32` = 48879)
* **Texto de Boas-Vindas:** String ASCII identificando versão e HostID atribuído (ex: `"Portico Router (3.0.0 (build 0)): Your ID 4"`).
* **Código de Sincronização:** `0x0000FEED` (`int32` = 65261). O servidor envia `0xFEED` e aguarda obrigatoriamente que o cliente responda com `0xFEED`. Se a conexão fechar antes, o servidor gera `Broken pipe`.

---

## 6. Marshalling de Mensagens e Extensão Valhalla

Conforme implementado em `MessageHelpers.java`:
1. **Manual Marshal (`FlagManualMarshal = 0x08`):**
   - Usado em `UpdateAttributes` e `SendInteraction` para evitar a sobrecarga de reflexão e permitir taxas acima de 100.000 mensagens/segundo.
   - O payload inicia com um byte booleano `0x01` (`true`), seguido pelo `short` do tipo da mensagem, seguido pelos campos binários da mensagem.
2. **Extensão Valhalla (`FlagValhallaExtended = 0x80`):**
   - Quando o nó receptor é um federado Valhalla, o payload é serializado segundo as especificações canônicas **SSZ** com raiz de Merkle criptográfica e suporte a envelopes pós-quânticos (ML-KEM-768 / Kyber).
   - Quando o nó receptor é um nó Portico padrão legado, a arquitetura de **Dual-Stream Dispatch** do Valhalla envia o payload no formato binário canônico esperado pelo Portico, garantindo interoperabilidade total sem necessidade de gateways externos.
