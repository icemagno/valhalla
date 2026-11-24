# Especificação Técnica do Protocolo de Rede Portico (Portico Wire Specification)

> **Projeto:** Projeto Valhalla  
> **Status:** Homologado para Desenvolvimento  
> **Referência:** Engenharia da arquitetura de mensagens do The Portico Project (Apache 2.0)

---

## 1. Visão Geral da Arquitetura de Rede do Portico

O Portico utiliza uma arquitetura baseada em pipeline de mensagens (`MessageSink`) onde todo evento de simulação (criação de federação, ingresso, publicação, registro de objeto, atualização de atributos e avanço de tempo) é encapsulado em uma instância de `PorticoMessage`.

Na camada de rede física, as mensagens trafegam através de dois canais principais:
1. **Canal de Controle (TCP Unicast):** Utilizado para conexões ponto a ponto confiáveis, handshakes de ciclo de vida (`Connect`, `Join`, `Resign`, `SyncPoints`) e respostas síncronas (`SUCCESS_RESP`, `ERROR_RESP`).
2. **Canal de Dados (UDP Multicast / TCP Mesh):** Utilizado para a distribuição em alta frequência de atualizações de atributos (`UpdateAttributes`) e interações transitórias (`SendInteraction`).

---

## 2. Estrutura Binária do Cabeçalho de Mensagem (*Portico Wire Header*)

Todas as mensagens binárias do Portico iniciam com um cabeçalho fixo serializado em **Big-Endian (Network Byte Order)**:

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                    Magic Number (0x504F5254)                  |  4 bytes ("PORT")
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|          Version (int16)      |       Message Type (int16)    |  4 bytes
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                    Source Federate Handle (int32)             |  4 bytes
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                    Target Federate Handle (int32)             |  4 bytes
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|      Flags (uint16)           |       Reserved (uint16)       |  4 bytes
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                    Payload Length (int32)                     |  4 bytes
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                        Payload Bytes...                       |  N bytes
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

### 2.1. Descrição dos Campos do Cabeçalho
* **Magic Number (4 bytes):** `0x504F5254` (representação ASCII de `"PORT"`). Garante alinhamento e rejeita pacotes estranhos na porta.
* **Version (2 bytes):** Versão do protocolo de transmissão (ex: `0x0002` para Portico v2.x).
* **Message Type (2 bytes):** Identificador unívoco do tipo de operação (ver tabela abaixo).
* **Source Federate Handle (4 bytes):** ID inteiro do federado de origem (`-1` para mensagens do executivo central).
* **Target Federate Handle (4 bytes):** ID do destinatário (`0` ou `-1` para broadcast na federação).
* **Flags (2 bytes):**
  * `0x0001`: Mensagem com carimbo de tempo (*Timestamp Order - TSO*).
  * `0x0002`: Mensagem comprimida.
  * `0x0004`: Mensagem de resposta síncrona requerida.
  * `0x0080`: **Flag de Extensão Valhalla (SSZ / KEM Ativo)**.
* **Payload Length (4 bytes):** Tamanho exato em bytes do corpo da mensagem que se segue imediatamente.

---

## 3. Tabela de Tipos de Mensagem (*MessageType Enumeration*)

| Código (Hex) | Nome da Mensagem | Descrição |
| :--- | :--- | :--- |
| `0x0001` | `SUCCESS_RESP` | Resposta positiva genérica do executivo. |
| `0x0002` | `ERROR_RESP` | Resposta com código e mensagem de exceção HLA. |
| `0x0010` | `CREATE_FEDERATION` | Requisição para instanciar uma execução de federação com módulos FOM. |
| `0x0011` | `DESTROY_FEDERATION` | Requisição para encerrar a federação. |
| `0x0012` | `JOIN_FEDERATION` | Ingresso de um federado com nome e tipo. Retorna o handle atribuído. |
| `0x0013` | `RESIGN_FEDERATION` | Saída voluntária de um federado da federação. |
| `0x0020` | `REGISTER_OBJECT` | Anúncio de criação de uma nova instância de classe de objeto. |
| `0x0021` | `DISCOVER_OBJECT` | Notificação aos demais federados sobre objeto descoberto. |
| `0x0022` | `UPDATE_ATTRIBUTES` | Envio de mapa de valores de atributos (`AttributeHandleValueMap`). |
| `0x0023` | `REFLECT_ATTRIBUTES`| Callback de entrega de atualização de atributos a federados inscritos. |
| `0x0030` | `SEND_INTERACTION` | Disparo de uma interação tática transitória com parâmetros. |
| `0x0031` | `RECEIVE_INTERACTION`| Callback de entrega de interação tática aos assinantes. |
| `0x0040` | `TIME_ADVANCE_REQUEST`| Solicitação de avanço de tempo lógico (TAR). |
| `0x0041` | `TIME_ADVANCE_GRANT` | Concessão de avanço temporal pelo motor LBTS (TAG). |
| `0x0050` | `SYNC_POINT_REGISTER`| Registro de ponto de sincronização global. |
| `0x0051` | `SYNC_POINT_ACHIEVED`| Notificação de alcance de ponto de sincronização. |

---

## 4. Sequência do Handshake de Conexão e Ingresso

### 4.1. Handshake no Nível de Transporte TCP (Sincronização 0xBEEF / 0xFEED)
Antes de qualquer mensagem HLA de alto nível ser trocada, a camada de transporte TCP do Portico (`TcpClientProxy`) executa uma negociação binária estrita de sincronização:

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
* **Texto de Boas-Vindas:** String ASCII identificando versão e HostID atribuído (ex: `"Portico Router (3.0.0): Your ID 1"`).
* **Código de Sincronização:** `0x0000FEED` (`int32` = 65261). O servidor envia `0xFEED` e aguarda obrigatoriamente que o cliente responda exatamente com `0xFEED`. Se a conexão fechar antes, o servidor gera `Broken pipe`.

---

### 4.2. Sequência de Mensagens de Ciclo de Vida da Federação (HLA Level)

```
Federado Go (Valhalla)                      Executivo Portico (Java)
       │                                                │
       │─── 1. Handshake TCP (0xBEEF / 0xFEED) ────────>│
       │                                                │
       │─── 2. CREATE_FEDERATION ("ValhallaFed") ──────>│
       │<── 3. SUCCESS_RESP (ou FED_ALREADY_EXISTS) ────│
       │                                                │
       │─── 4. JOIN_FEDERATION ("GoPilot", "Aircraft") >│
       │<── 5. SUCCESS_RESP { FederateHandle: 101 } ────│
       │                                                │
       │    [ Federado autenticado e ativo na rede ]    │
```

### 4.3. Payload de `JOIN_FEDERATION`
1. `FederationName` (String com prefixo de tamanho de 2 bytes).
2. `FederateName` (String com prefixo de tamanho de 2 bytes).
3. `FederateType` (String com prefixo de tamanho de 2 bytes).
4. `MOM Data / Metadados de Capacidade` (Bytes adicionais onde o Valhalla injeta sua assinatura `VALHALLA-EXT`).

---

## 5. Estratégia de Extensão Valhalla sem Quebra

1. **Assinatura de Capacidade:** O Valhalla envia no handshake sua chave de capacidades. Se o servidor for o Portico Java original, o Portico ignora bytes não mapeados após o nome do federado ou os armazena como metadados opacos.
2. **Encapsulamento de Atributos:** O payload de `UPDATE_ATTRIBUTES` contém um contador de atributos seguido por pares:
   * `AttributeHandle (int32)`
   * `ValueLength (int32)`
   * `ValueBytes (byte[])`
3. O buffer **SSZ** ou o buffer cifrado com **KEM** é gravado diretamente no interior de `ValueBytes`.
