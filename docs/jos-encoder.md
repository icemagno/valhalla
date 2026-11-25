# Especificação e Arquitetura do Codificador JOS (Java Object Serialization) em Go

> **Projeto:** Projeto Valhalla (HLA-4 / IEEE 1516-2025)  
> **Autor / Arquiteto:** Carlos Magno Oliveira de Abreu (`@icemagno` / `magno.mabreu@gmail.com`)  
> **Módulo:** `pkg/crc/portico/wire/` e `pkg/crc/portico/wire/metamodel/`  
> **Status:** Homologado e Validado Bit a Bit contra OpenJDK 22 e Portico CRTI / LRC 3.0.0 (Build 0)  

---

## 1. O Desafio de Engenharia

O **Portico RTI** é o middleware de simulação distribuída HLA (IEEE 1516 / HLA 1.3) em Java mais adotado no mundo acadêmico, industrial e de defesa. O executivo central do Portico (`rtiexec` / CRTI) e as bibliotecas clientes (`LRC - Local RTI Component`) comunicam-se pela rede através de um protocolo proprietário encapsulado em sockets TCP e grupos UDP Multicast.

Para a camada de controle administrativo (criação e destruição de federações, ingresso (`Join`), saída (`Resign`), anúncio e sincronização de pontos (`SyncPoints`)), o Portico utiliza internamente o mecanismo nativo **Java Object Serialization (JOS)** através de instâncias de `java.io.ObjectOutputStream` e `java.io.ObjectInputStream`.

O **Projeto Valhalla**, implementado em **Go puro** (com extensões SIMD/Rust), possui como requisito inegociável a **interoperabilidade 100% nativa em nível de fio (*wire-level*)** com o Portico legado:
- Sem a presença de JVM instalada na máquina do federado.
- Sem wrappers CGO/JNI.
- Sem bridges ou gateways externos que degradem a latência.
- **Sem atalhos, sem templates estáticos (`.bin`) e sem "marretas":** toda serialização e desserialização é executada puramente em memória por algoritmos canônicos em Go.

Para viabilizar essa comunicação sem recorrer a contornos frágeis, foi projetado e construído um **motor genérico, agnóstico e extensível de serialização binária Java Object Serialization em Go**, equivalente em comportamento ao `ObjectOutputStream` e `ObjectInputStream` oficiais da JVM.

---

## 2. Anatomia do Java Object Serialization (JOS) no Nível de Bits

A especificação binária formal do Java Object Serialization define uma gramática de fluxo (*stream*) orientada a tokens, prefixada por cabeçalhos estritos e governada por uma tabela interna de referências (*object graph handles*).

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

A especificação Java determina:
1. **Campos primitivos primeiro**, ordenados estritamente por ordem alfabética de seus nomes.
2. **Campos de objetos depois**, também ordenados estritamente por ordem alfabética de seus nomes.

Qualquer discrepância na ordem ou na assinatura de tipos entre o que é anunciado no `ClassDesc` e o que a classe na JVM espera faz o `ObjectStreamClass` lançar `InvalidClassException: incompatible field type`.

---

## 3. Arquitetura Modular de Metamodelos (`pkg/portico/wire/metamodel/`)

Seguindo o princípio de responsabilidade única (SRP) e modularidade rigorosa, **cada classe Java possui seu próprio arquivo Go dedicado** no pacote [`pkg/portico/wire/metamodel/`](file:///c:/Magno/Projetos/hla-rti/pkg/portico/wire/metamodel/), eliminando arquivos monolíticos.

```
pkg/portico/wire/metamodel/
├── types.go                             # Interfaces JOSWriter/JOSReader, ClassDesc, FieldDesc, Tokens
├── registry.go                          # Catálogo concorrente RegisterClassDesc / LookupClassDesc
├── java_lang_object.go                  # java.lang.Object (Superclasse raiz)
├── java_lang_number.go                  # java.lang.Number (SUID: 0x86AC951D0B94E08B)
├── java_lang_integer.go                 # java.lang.Integer (SUID: 0x12E2A0A4F7818738)
├── java_lang_long.go                    # java.lang.Long (SUID: 0x3B8BAA2444859841)
├── java_lang_boolean.go                 # java.lang.Boolean (SUID: 0xCD207280D59CFAEE)
├── java_lang_enum.go                    # java.lang.Enum (Superclasse canônica de enums)
├── java_util_hashmap.go                 # java.util.HashMap (SUID: 0x0507DAC1C31660D1)
├── java_util_arraylist.go               # java.util.ArrayList (SUID: 0x7881D21D99C7619D)
├── portico_message.go                   # org.portico.utils.messaging.PorticoMessage (Base 22b)
├── portico_response_message.go          # org.portico2.common.messaging.ResponseMessage
├── portico_extended_success_response.go # org.portico2.common.messaging.ExtendedSuccessResponse
├── portico_create_federation.go         # org.portico2.common.services.federation.msg.CreateFederation
├── portico_join_federation.go           # org.portico2.common.services.federation.msg.JoinFederation
├── portico_jresign_action.go            # org.portico.lrc.compat.JResignAction (Enum)
├── portico_resign_federation.go         # org.portico2.common.services.federation.msg.ResignFederation
├── portico_sync_point_achieved.go       # org.portico2.common.services.sync.msg.SyncPointAchieved
├── portico_announce_sync_point.go       # org.portico2.common.services.sync.msg.AnnounceSyncPoint
├── portico_federation_synchronized.go   # org.portico2.common.services.sync.msg.FederationSynchronized
├── portico_welcome_pack.go              # org.portico2.common.services.federation.msg.WelcomePack
└── portico_rti_probe.go                 # org.portico2.common.services.federation.msg.RtiProbe
```

---

## 4. O Mecanismo `CustomOnly` vs `defaultWriteObject`

Uma das descobertas técnicas mais importantes na engenharia do decodificador e codificador diz respeito ao comportamento de classes que implementam `writeObject` / `writeExternal`:

### 4.1. O Contrato de `SC_WRITE_METHOD` (0x01)
Pela especificação do Java Object Serialization, quando uma classe declara `private void writeObject(ObjectOutputStream out)`:
* Se ela invocar `out.defaultWriteObject()`, a JVM serializa os campos declarados antes de qualquer dado customizado. Este é o comportamento de classes como `java.util.HashMap` (que grava `loadFactor` e `threshold` antes das entradas da tabela).
* Se ela **NÃO invocar `out.defaultWriteObject()`**, a JVM **não grava os campos soltos**. Ela escreve exclusivamente os dados customizados empacotados em um bloco `TC_BLOCKDATA`.

### 4.2. Otimização de 22 Bytes do `PorticoMessage`
No código oficial do Portico (`PorticoMessage.java`), a classe declara 6 campos de instância (`sourceFederate`, `targetFederate`, `targetFederation`, `isFromRti`, `immediate`, `timestamp`). No entanto, no método de serialização, o Portico faz:

```java
private void writeObject( ObjectOutputStream oos ) throws IOException {
    writeExternal( oos ); // NÃO chama defaultWriteObject!
}
public void writeExternal( ObjectOutput output ) throws IOException {
    output.writeBoolean( isFromRti );     // 1 byte
    output.writeInt( sourceFederate );    // 4 bytes
    output.writeInt( targetFederate );    // 4 bytes
    output.writeInt( targetFederation );  // 4 bytes
    output.writeDouble( timestamp );      // 8 bytes
    output.writeBoolean( immediate );     // 1 byte
}
```

O total exato é **22 bytes** gravados em um único bloco `TC_BLOCKDATA` (`0x77 0x16 ... 0x78`).

Para modelar isso declarativamente em Go, introduzimos a flag `CustomOnly bool` na struct `ClassDesc`:
* Quando `CustomOnly: true` (como em `PorticoMessage` e `ResponseMessage`), o codificador Go suprime a gravação individual de campos e executa diretamente o `CustomWriter`, garantindo alinhamento estrito de bytes com a JVM.

### 4.3. Regra de Classes `Externalizable` (`SC_EXTERNALIZABLE = 0x04`)
Quando uma classe implementa `java.io.Externalizable` (como `ExtendedSuccessResponse` e `WelcomePack`), a JVM ignora a hierarquia de superclasses durante a gravação/leitura de dados. O codificador e o decodificador Go executam exclusivamente o `CustomWriter` / `CustomReader` da própria classe concreta, sem emitir blocos espúrios de superclasses que provocariam `java.io.OptionalDataException` na JVM.

### 4.4. Suporte Canônico a Coleções: `java.util.HashSet` e `java.util.HashMap`
* **`java.util.HashSet` (`pkg/portico/wire/metamodel/java_util_hashset.go`):** SUID `-5024744406713321676L` (`0xBA44859596B8B734`). O `HashSet` grava cabeçalho de 12 bytes (`capacity`, `loadFactor`, `size`), seguido por seus elementos individuais e terminador `TC_ENDBLOCKDATA` (0x78).
* **`java.util.HashMap` Polimórfico:** Chaves e valores são desserializados como `r.ReadObject()`, suportando chaves do tipo `Integer` (handles) e `String` sem corromper o fluxo.

### 4.5. Descarte Universal de Dados Customizados (`skipCustomData`)
Em árvores de classes com `SC_WRITE_METHOD` onde a classe concreta não possui leitor customizado no metamodelo Go, o decodificador executa `skipCustomData()`, avançando os blocos binários e sub-objetos com segurança até o delimitador `TC_ENDBLOCKDATA` (0x78), assegurando que leituras de campos subsequentes no objeto não colidam com tokens residuais.

---

## 5. Chaves Oficiais do Portico no `ExtendedSuccessResponse`

Na criação de federação (`CreateFederation`), o handler oficial do LRC Portico (`CreateFederationHandler.java`, linha 99) executa:

```java
int federationHandle = context.getSuccessResultAsInt( CreateFederation.KEY_FEDERATION_HANDLE );
```

A constante `CreateFederation.KEY_FEDERATION_HANDLE` avalia exatamente para a string `"federationHandle"`. Se a resposta contiver apenas `"result"`, o desempacotamento de `(int) null` estoura `NullPointerException`. Por essa razão, o gerador `BuildCreateFederationResponsePayload` popula canonicamente:
- `"federationHandle"` -> `Integer(fedHandle)` *(lido pelo CreateFederationHandler)*
- `"federationName"` -> `String(fedName)`
- `"result"` -> `Integer(fedHandle)` *(para compatibilidade geral com getResult())*

Já no ingresso de federação (`JoinFederation`), o `JoinFederationHandler.java` consome `(WelcomePack) context.getSuccessResult()`, que busca pela chave `"result"`, onde o `WelcomePack` é entregue.

---

## 6. Homologação Bit a Bit contra OpenJDK 22

Para comprovar formalmente a exatidão matemática do motor Go, o fluxo binário sintetizado por `BuildCreateFederationResponsePayload` e `BuildSuccessResponsePayload` foi comparado diretamente contra o fluxo gerado pelo `ObjectOutputStream` do OpenJDK 22 compilado:

* **Tamanho gerado em Go:** 312 bytes
* **Tamanho gerado pelo OpenJDK 22:** 312 bytes
* **Diferença Binária (Diff Bit a Bit):** **0 bytes (100% idêntico)**
* **Desserialização no OpenJDK:**
  ```text
  Successfully deserialized: org.portico2.common.messaging.ExtendedSuccessResponse
  Result value: 1
  ```

---

## 7. Conclusão

Com o metamodelo modular em `pkg/portico/wire/metamodel/` e o motor agnóstico em `pkg/portico/wire/`, o Projeto Valhalla atinge soberania completa: comunica-se no mais baixo nível com o ecossistema Portico legado sem depender de nenhuma máquina virtual Java, mantendo zero overhead no caminho crítico do protocolo nativo HLA-4 (SSZ / Merkle).
