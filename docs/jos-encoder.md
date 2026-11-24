# Especificação e Arquitetura do Codificador JOS (Java Object Serialization) em Go

> **Projeto:** Projeto Valhalla (HLA-4 / IEEE 1516-2025)  
> **Autor / Arquiteto:** Carlos Magno Oliveira de Abreu (`@icemagno` / `magno.mabreu@gmail.com`)  
> **Módulo:** `pkg/portico/wire/jos_*.go`  
> **Status:** Homologado em Produção contra o Portico CRTI 3.0.0 (Build 0)  

---

## 1. O Desafio de Engenharia

O **Portico RTI** é o middleware de simulação distribuída HLA (IEEE 1516 / HLA 1.3) em Java mais adotado no mundo acadêmico, industrial e de defesa. O executivo central do Portico (`rtiexec` / CRTI) e as bibliotecas clientes (`LRC - Local RTI Component`) comunicam-se pela rede através de um protocolo proprietário encapsulado em sockets TCP e grupos UDP Multicast.

Para a camada de controle administrativo (criação e destruição de federações, ingresso (`Join`), saída (`Resign`), anúncio e sincronização de pontos (`SyncPoints`)), o Portico utiliza internamente o mecanismo nativo **Java Object Serialization (JOS)** através de instâncias de `java.io.ObjectOutputStream` e `java.io.ObjectInputStream`.

O **Projeto Valhalla**, implementado em **Go puro** (com extensões SIMD/Rust), possui como requisito inegociável a **interoperabilidade 100% nativa em nível de fio (wire-level)** com o Portico legado, sem a presença de JVM instalada na máquina do federado, sem wrappers CGO/JNI e sem bridges ou gateways externos que degradem a latência.

Para viabilizar essa comunicação sem recorrer a marretas pontuais ou hardcodings frágeis (`serializaProbe()`, `serializaJoin()`), foi projetado e construído um **motor genérico, agnóstico e extensível de serialização binária Java Object Serialization em Go**, equivalente em comportamento ao `ObjectOutputStream` oficial da JVM.

---

## 2. Anatomia do Java Object Serialization (JOS) no Nível de Bits

A especificação binária formal do Java Object Serialization define uma gramática de fluxo (stream) orientada a tokens, prefixada por cabeçalhos estritos e governada por uma tabela interna de referências (object graph handles).

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|       STREAM_MAGIC (0xACED)   |     STREAM_VERSION (0x0005)   |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|  Token (byte) |                   Dados do Elemento...        |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

### 2.1. Tabela Fundamental de Tokens de Controle (JOS Grammar)

| Token | Código Hex | Descrição Canônica Java | Significado no Stream |
| :--- | :--- | :--- | :--- |
| `TC_NULL` | `0x70` | `NullReference` | Referência nula (`null`). Ocupa 1 único byte. |
| `TC_REFERENCE` | `0x71` | `PrevObject` | Referência a um objeto ou descritor já serializado anteriormente. Seguido por `int32` com o handle. |
| `TC_CLASSDESC` | `0x72` | `ClassDescriptor` | Declaração de metadados da classe (nome, `serialVersionUID`, flags, lista de campos). |
| `TC_OBJECT` | `0x73` | `NewObject` | Instanciação de um novo objeto. Seguido pelo `TC_CLASSDESC` e pelos valores dos campos. |
| `TC_STRING` | `0x74` | `NewString` | String UTF-8. Seguido por `uint16` (tamanho em bytes) e os bytes da string. |
| `TC_ARRAY` | `0x75` | `NewArray` | Vetor/Array. |
| `TC_CLASS` | `0x76` | `NewClass` | Referência à classe em si (`Class<?>`). |
| `TC_BLOCKDATA` | `0x77` | `BlockData` | Bloco opaco de dados binários (até 255 bytes). Usado por `writeExternal` ou `writeObject`. |
| `TC_ENDBLOCKDATA`| `0x78` | `EndBlockData` | Marcador de término de bloco de anotações ou dados customizados. |
| `TC_RESET` | `0x79` | `Reset` | Limpa a tabela de handles e referências do stream. |
| `TC_BLOCKDATALONG`| `0x7A`| `BlockDataLong` | Bloco opaco de dados longos (até $2^{31}-1$ bytes). |
| `TC_ENUM` | `0x7E` | `NewEnum` | Constante enumerada Java (`Enum<?>`). |

### 2.2. Sistema de Handles e Pooling de Objetos (`0x007E0000`)

Para preservar a identidade referencial de grafos cíclicos e otimizar a largura de banda, a JVM atribui identificadores inteiros crescentes a cada classe, objeto, enum e string serializados, iniciando no offset base:

$$\text{BaseWireHandle} = \text{0x007E0000}$$

Quando o `JOSEncoder` serializa uma classe (ex: `Ljava/lang/String;`) ou uma string já enviada, ele não reemite o texto ou descritor; em vez disso, emite o token `TC_REFERENCE` (`0x71`) seguido pelo handle de 32 bits Big-Endian correspondente.

### 2.3. Ordenação Canônica de Campos (JOS Spec §4.3)

Um erro comum em implementações parciais de JOS é a ordem de serialização dos campos no descritor de classe (`TC_CLASSDESC`). A especificação Java determina:
1. **Campos primitivos primeiro**, ordenados estritamente por ordem alfabética de seus nomes.
2. **Campos de objetos depois**, também ordenados estritamente por ordem alfabética de seus nomes.

Qualquer discrepância na ordem ou na assinatura de tipos entre o que é anunciado no `ClassDesc` e o que a classe na JVM espera faz o `ObjectStreamClass` lançar um `InvalidClassException: incompatible field type`.

### 2.4. Tratamento Especial de Java Enums

Diferente de objetos comuns, instâncias de `Enum` **não serializam seus campos de instância**. A gramática do JOS para enums é estritamente:

```
TC_ENUM (0x7E)
  ClassDescriptor (Nome do Enum, serialVersionUID=0L, Flags=SC_SERIALIZABLE|SC_ENUM [0x12], Fields=0)
  SuperClassDescriptor (java.lang.Enum, serialVersionUID=0L, Flags=0x12, Fields=0)
  Handle Assignment (0x007E0000 + n)
  TC_STRING (Nome da constante enum, ex: "NO_ACTION")
```

---

## 3. Arquitetura de Software do `JOSEncoder` no Valhalla

O subsistema foi desenhado seguindo os princípios **SOLID**, separando completamente os conceitos de **metamodelo**, **motor de codificação** e **catálogo de esquemas**.

```
                   +----------------------------------+
                   |          jos_types.go            |
                   |   - FieldType (byte)             |
                   |   - FieldDesc (Nome, Tipo)       |
                   |   - ClassDesc (Metamodelo)       |
                   |   - JavaObject (Instância Go)    |
                   |   - JavaEnum (Constante Enum)    |
                   +-----------------+----------------+
                                     ^
                                     | usa metamodelo
         +---------------------------+---------------------------+
         |                                                       |
+--------------------+                                  +--------------------+
|   jos_encoder.go   |                                  |   jos_portico.go   |
| Motor Agnóstico    |                                  | Catálogo Portico   |
| - Header (ACED0005)|                                  | - ClassDescPortico |
| - Pool de Handles  | <--- executa serialização de --- | - ClassDescJoin    |
| - Recursive Chain  |                                  | - ClassDescResign  |
| - Big-Endian IO    |                                  | - ClassDescSync    |
+--------------------+                                  +--------------------+
```

### 3.1. Metamodelo JOS (`jos_types.go`)

Representa as estruturas em memória de forma agnóstica:
* `FieldType`: Enumeração dos códigos de tipo da JVM (`TypeInt = 'I'`, `TypeBoolean = 'Z'`, `TypeObject = 'L'`, etc.).
* `ClassDesc`: Descreve a classe Java, seu `serialVersionUID`, flags (`SC_SERIALIZABLE`, `SC_WRITE_METHOD`, `SC_ENUM`), campos declarados e superclasse.
* `JavaObject`: Estrutura de dados genérica com mapa fluente (`Set(campo, valor)` / `Get(campo)`). Permite representar qualquer objeto Java sem gerar structs Go específicas para cada classe da JVM.
* `JavaEnum`: Representa uma constante enumerada associada ao seu `ClassDesc`.

### 3.2. Motor Codificador Genérico (`jos_encoder.go`)

O codificador é o equivalente em Go ao `ObjectOutputStream`:
1. **Controle de Cabeçalho:** Garante emissão única de `0xACED 0x0005`.
2. **Gerenciamento de Tabela de Referências:** Mapeia referências ativas (`refMap map[string]uint32`) gerando tokens `TC_REFERENCE` sempre que possível.
3. **Resolução de Herança Invertida:** Em Java, a hierarquia de descritores de classe é declarada da subclasse para a superclasse (`Sub -> Super -> Object`), mas os **valores dos dados** são gravados da classe base para a subclasse (`Object -> Super -> Sub`). O `JOSEncoder` realiza a inversão da cadeia automaticamente via `getClassHierarchy()`.
4. **Custom Writers:** Classes que implementam `writeObject` ou `writeExternal` (como `PorticoMessage` e `ArrayList`) possuem hooks `CustomWriter func(enc *JOSEncoder, obj *JavaObject)` que injetam seus blocos `TC_BLOCKDATA` com transparência.

### 3.3. Catálogo Declarativo do Portico (`jos_portico.go`)

Todas as mensagens de controle do Portico são registradas como meros descritores de metadados:
* `ClassDescPorticoMessage`: Classe base (`org.portico.utils.messaging.PorticoMessage`, `serialVersionUID = 98121116105109L`) com bloco de 22 bytes de cabeçalho interno.
* `ClassDescJoinFederation`: Requisição de ingresso na federação (`org.portico2.common.services.federation.msg.JoinFederation`).
* `ClassDescCreateFederation`: Criação de federação.
* `ClassDescResignFederation`: Desligamento de federação com enum canônico `JResignAction`.
* `ClassDescSyncPointAchieved`: Sinalização de ponto de sincronização alcançado.
* `ClassDescArrayList`: Suporte a listas encadeadas da JVM (`java.util.ArrayList`).

Para adicionar qualquer nova mensagem do Portico ao Valhalla, **nenhuma nova função de codificação precisa ser escrita**: basta declarar a nova struct `ClassDesc`.

---

## 4. O Caminho Inverso: A Contraparte `JOSDecoder`

Assim como o `JOSEncoder` atua como `ObjectOutputStream`, o caminho reverso exige o **`JOSDecoder`**, atuando como **`ObjectInputStream`**.

### 4.1. O que o Valhalla Recebe do Portico?

Quando o Valhalla envia um `JoinFederation`, o Portico responde com um pacote de controle contendo um objeto Java serializado:
1. `ExtendedSuccessResponse`: Contém o mapa `HashMap<String, Object>` com os resultados da operação.
2. No caso do `JoinFederation`, o resultado principal é o **`WelcomePack`** (~33 KB):
   * `federateHandle` (ex: `uint16 = 7` ou `int32`).
   * `federationHandle`.
   * `fom` / `ObjectModel`: A árvore FOM completa dos objetos e interações configurados na federação.
3. No caso de notificações assíncronas do Portico, o Valhalla recebe:
   * `AnnounceSyncPoint` (`label`, `tag`).
   * `FederationSynchronized` (`label`).
   * `DiscoverObjectInstance` e `ReflectAttributeValues` (quando trafegando via canal de controle TCP).

### 4.2. Arquitetura do `JOSDecoder`

O `JOSDecoder` funciona como uma máquina de estados finitos que consome a gramática JOS:

```
[Leitura STREAM_MAGIC 0xACED + VERSION 0x0005]
                  │
                  ▼
         [Leitura de Token]
       ┌──────────┼──────────┬──────────┬──────────┐
       ▼          ▼          ▼          ▼          ▼
    TC_OBJECT  TC_STRING  TC_ARRAY   TC_REF    TC_BLOCK
       │          │          │          │          │
    Lê Desc    Lê UTF     Lê Len     Busca      Lê Len
    Aloca Obj  Registra   e Itens    Handle     e Bytes
    Lê Campos  Handle     Registra   Tabela
       │                     Handle
       ▼
 [Retorna *JavaObject]
```

1. **Reconstituição de `ClassDesc`:** Lê o nome da classe, `serialVersionUID` e a lista de campos declarados.
2. **Resolução de Referências:** Armazena cada objeto, string e classe lida em um vetor indexado a partir de `0x007E0000`. Ao encontrar `TC_REFERENCE`, retorna imediatamente o ponteiro do objeto já instanciado.
3. **Mapeamento Polimórfico:**
   * Tipos primitivos são lidos diretamente para tipos Go (`int32`, `float64`, `bool`).
   * Strings tornam-se `string`.
   * Instâncias de classes tornam-se `*JavaObject` com os campos preenchidos em `Fields map[string]any`.
   * Enums tornam-se `*JavaEnum` com o nome da constante.

---

## 5. Homologação e Testes Automatizados

O motor JOS foi submetido a baterias de testes unitários e de integração real:

1. **Testes Unitários de Isolamento (`pkg/portico/wire/jos_test.go`):**
   * Validação de integridade do Magic e Version (`0xACED0005`).
   * Validação da gramática de `RtiProbe`, `JoinFederation`, `CreateFederation`, `ResignFederation`.
   * Validação da emissão do token `TC_ENUM` (`0x7E`) para `JResignAction.NO_ACTION`.
   * Validação da ausência de deadlocks e alocações desnecessárias. **Status: 100% PASS (0.00s)**.

2. **Teste de Integração Real contra o Portico CRTI (`test/integration/live_portico_test.go`):**
   * Conexão viva via TCP com o executivo Portico no contêiner Arcanjo (`192.168.1.226:52295`).
   * Ciclo de vida completo executado:
     - Handshake TCP `0xBEEF` / `0xFEED`.
     - Ingresso em federação viva (`ExampleFederation`) com o federado `Thor` ativo.
     - Recepção do pacote de 33.498 bytes (`ControlResponseOK`, CallType = 3).
     - Saída graciosa (`ResignFederation` com `NO_ACTION`).
     - Recepção da confirmação de saída (`ResignFederation`, `ControlResponseOK`, CallType = 3).
   * **Status: 100% PASS (0.26s)**.

---

## 6. Conclusão

Com a criação do `JOSEncoder` e do metamodelo em `pkg/portico/wire`, o Projeto Valhalla provou ser viável interagir com runtimes Java legados sem introduzir complexidade de compilação ou dependências externas. A solução preserva a arquitetura limpa do Go, mantém a compatibilidade normativa estrita com o Portico e estabelece as bases sólidas para a implementação simétrica do `JOSDecoder`.
