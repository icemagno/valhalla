# Plano de Projeto e Arquitetura do Core da CRTI

> **Projeto:** Projeto Valhalla (HLA-4 / IEEE 1516-2025)  
> **Autor / Arquiteto:** Carlos Magno Oliveira de Abreu (`@icemagno` / `magno.mabreu@gmail.com`)  
> **Status:** Documento Normativo e Plano Diretor do Core  
> **Data:** 24/11/2026  

---

## 1. Princípio Arquitetural e Visão Soberana

Uma Run-Time Infrastructure (RTI) segundo o padrão IEEE 1516 não é um conversor de pacotes de rede. A Central RTI Component (CRC / CRTI) é a **autoridade central executiva da simulação distribuída**, responsável por coordenar a coerência de dados, ordenação temporal e sincronização de múltiplos nós.

### 1.1. O Trato com o Portico é Estritamente no Nível de Protocolo
- O Portico é apenas um cliente legado que fala um dialeto de rede específico (Java Object Serialization sobre TCP).
- **O Core da nossa CRTI não deve ser moldado para satisfazer o Portico.**
- O Core é implementado em Go puro a partir dos primeiros princípios da norma **IEEE 1516-2025 (HLA-4)**, incorporando alta concorrência, baixa latência e estado merkleizado.
- Os protocolos de rede (Portico JOS e Valhalla VAL4) são **adaptadores de entrada (*Inbound Adapters*)** periféricos que traduzem pacotes em comandos agnósticos para o Core.

---

## 2. Segregação Estrita de Pacotes e Responsabilidades

Para garantir que o **LRC** (biblioteca cliente do federado) possa ser distribuído e utilizado por desenvolvedores terceiros sem carregar dependências do **CRC** (executável servidor), a estrutura de pastas é governada pelo princípio da dependência unidirecional:

```
                  ┌──────────────────────┐
                  │      pkg/common      │  <-- Tipos Base, Handles, Erros HLA,
                  └──────────▲───────────┘      Wire Protocol VAL4 (Zero dependências)
                             │
            ┌────────────────┴────────────────┐
            │                                 │
  ┌─────────┴─────────┐             ┌─────────┴─────────┐
  │      pkg/lrc      │             │      pkg/crc      │
  │ (Cliente Federado)│             │ (Servidor Central)│
  │  - RTIambassador  │             │  - Core Engine    │
  │  - Net Client     │             │  - Adapters:      │
  └───────────────────┘             │    * Native VAL4  │
                                    │    * Portico JOS  │
                                    └─────────┬─────────┘
                                              │
                                    ┌─────────┴─────────┐
                                    │  pkg/crc/portico  │
                                    │ (Metamodelo JOS)  │
                                    └───────────────────┘
```

### Regras de Ouro de Importação:
1. **`pkg/common`:** Não importa NENHUM outro pacote interno (`crc`, `lrc`). Define:
   - Identificadores e Handles canônicos (`FederateHandle`, `ObjectClassHandle`, `AttributeHandle`, `InteractionClassHandle`).
   - Erros e exceções oficiais da norma IEEE 1516.
   - Especificação e codificador/decodificador do cabeçalho binário nativo **VAL4** (`NativeHeader`, códigos de mensagens `NativeMsg...`).
2. **`pkg/lrc` (A Biblioteca do Federado):**
   - É a biblioteca que o usuário importa em sua aplicação (`import "github.com/icemagno/valhalla/pkg/lrc"`).
   - **NUNCA importa `pkg/crc`.**
   - Importa apenas `pkg/common`.
3. **`pkg/crc` (O Executável Centralizador):**
   - É o motor do servidor (`cmd/valhalla-crc`).
   - Importa `pkg/common` para tipos e cabeçalhos nativos.
   - Contém internamente `pkg/crc/portico` como seu adaptador legado.
   - **NUNCA é importado pelo `pkg/lrc`.**
4. **`pkg/crc/portico` (Compatibilidade Wire-Level):**
   - Fica contido abaixo de `pkg/crc`, contendo apenas o metamodelo JOS e codecs binários legados. Usado exclusivamente pelo adaptador Portico do CRC.

---

## 3. Arquitetura do Core da CRTI (`pkg/crc/core/`)

O Core da CRTI é desacoplado de qualquer transporte (TCP, UDP, sockets Unix) e opera através de interfaces em memória thread-safe.

```
═════════════════════════════════════════════════════════════════════
                   VALHALLA CRTI CORE ENGINE
═════════════════════════════════════════════════════════════════════
  [1] FederationRegistry & Barriers
      - Gerenciamento de ciclo de vida de federações e federados.
      - Barreiras de sincronização coletivas (SyncPoints) com cálculo de quórum.

  [2] DeclarationRegistry & Interest Matcher
      - Árvore hierárquica de classes de objetos e interações (OMT).
      - Roteador Pub/Sub em memória: mapeia (ClassHandle, AttributeHandle) -> Assinantes.
      - Suporte a herança polimórfica (assinatura na superclasse recebe subclasses).

  [3] ObjectRegistry & Merkle State Store
      - Tabela de instâncias vivas de objetos e posse de atributos.
      - Despacho de descobertas seletivas (DiscoverObject).
      - Roteamento de reflexões (ReflectAttributes) e interações (ReceiveInteraction).
      - Cálculo de raiz de Merkle (hash_tree_root de 32 bytes) por entidade.

  [4] Time Management Engine
      - Estado temporal por federado: CurrentTime, Lookahead, Regulating, Constrained.
      - Cálculo concorrente de LITS (Lowest Impending Time Stamp) e GALT.
      - Concessão determinística de avanço (TimeAdvanceGrant).
      - Filas de mensagens TSO (Time-Stamp Order) ordenadas por timestamp.
═════════════════════════════════════════════════════════════════════
```

### 3.1. Interface de Notificação Agnóstica (`FederateSession`)
O Core se comunica de volta com os federados (sejam eles nativos VAL4 ou legados Portico) através de uma interface abstrata de entrega de eventos:

```go
type FederateSession interface {
    ID() int32
    Name() string
    Protocol() ProtocolType
    OnAnnounceSyncPoint(label string, tag []byte) error
    OnFederationSynchronized(label string) error
    OnDiscoverObject(objectHandle, classHandle int32, name string) error
    OnReflectAttributes(objectHandle int32, attributes map[int32][]byte, tag []byte, timestamp *float64) error
    OnReceiveInteraction(classHandle int32, parameters map[int32][]byte, tag []byte, timestamp *float64) error
    OnTimeAdvanceGrant(time float64) error
}
```

Dessa forma, o Core **não sabe se o federado fala JOS ou VAL4**. Ele apenas invoca `session.OnTimeAdvanceGrant(time)`, e cada adaptador serializa no formato correspondente do seu protocolo.

---

## 4. Cronograma Executivo de Desenvolvimento (4 Fases)

```
┌────────────────────────────────────────────────────────────────────────┐
│ FASE 1: Limpeza e Quebra do Acoplamento lrc -> crc                     │
│  - Criar pkg/common/ com NativeHeader, NativeMsg... e erros HLA.       │
│  - Refatorar pkg/lrc para importar apenas pkg/common.                  │
│  - Garantir 0 dependências de crc dentro de lrc.                       │
└──────────────────────────────────┬─────────────────────────────────────┘
                                   │
┌──────────────────────────────────▼─────────────────────────────────────┐
│ FASE 2: Declaration Management & Roteador Pub/Sub em Memória           │
│  - Tabela hierárquica OMT (Classes de Objetos, Atributos, Interações). │
│  - Registry de Publicações e Assinaturas com resolução de herança.     │
│  - Testes unitários com benchmarks de casamento em nanosegundos.       │
└──────────────────────────────────┬─────────────────────────────────────┘
                                   │
┌──────────────────────────────────▼─────────────────────────────────────┐
│ FASE 3: Object Management & Estado Merkleizado                         │
│  - Registro de instâncias, posse de atributos e exclusão.              │
│  - Despacho automático de DiscoverObject e ReflectAttributes.          │
│  - Rastreamento de integridade via Merkle Tree Root (SSZ / SHA-256).   │
└──────────────────────────────────┬─────────────────────────────────────┘
                                   │
┌──────────────────────────────────▼─────────────────────────────────────┐
│ FASE 4: Time Management Engine (GALT / LITS & Filas TSO)               │
│  - Rastreador de avanço temporal (Regulating, Constrained, Lookahead). │
│  - Algoritmo de cálculo de LITS e GALT para avanço seguro.             │
│  - Filas de prioridade mínima para mensagens TSO.                      │
└────────────────────────────────────────────────────────────────────────┘
```

---

## 5. Próximos Passos Imediatos

1. Executar a **Fase 1**: Criar `pkg/common/` e migrar as definições do protocolo nativo e tipos comuns de `pkg/crc/native_handler.go` para lá.
2. Atualizar o `pkg/lrc` para importar `pkg/common`, eliminando completamente o `import "github.com/icemagno/valhalla/pkg/crc"`.
3. Validar a compilação e testes de todo o repositório (`go test ./...`).
