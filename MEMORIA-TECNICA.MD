# MEMÓRIA TÉCNICA E DIÁRIO DE BORDO: PROJETO VALHALLA

> **Projeto:** Projeto Valhalla  
> **Finalidade:** Diário de bordo técnico contínuo para registro histórico, rastreabilidade de decisões arquiteturais, justificativas de engenharia, marcos de implementação, resultados de testes (TDD) e controle de regressões.  
> **Criador e Desenvolvedor Principal:** Carlos Magno Oliveira de Abreu ([@icemagno](https://github.com/icemagno) | `magno.mabreu@gmail.com`)  
> **Licença:** Apache License, Version 2.0  
> **Repositório:** `github.com/icemagno/valhalla`  
> **Escopo Central:** Evolução do PorticoRTI para a especificação **IEEE 1516-2025 (HLA-4)**, incorporando **Serialização SSZ**, **Árvores de Merkle**, **Criptografia Pós-Quântica (ML-KEM/Kyber)** e **Compatibilidade Nativa de Rede (*Wire-Compatible*) com o PorticoRTI**, utilizando o **RPR-FOM v2.0** como base normativa estrita de testes.

---

## Estrutura do Documento
Este diário de bordo é organizado cronologicamente em entradas diárias no formato:
`## Decisões Técnicas - dd/mm/aaaa`

Cada entrada documenta minuciosamente:
1. As decisões conceituais, estratégicas e arquiteturais deliberadas no dia.
2. O racional de engenharia (o *"porquê"* de termos escolhido essa abordagem e descartado alternativas).
3. O que foi efetivamente implementado, testado e validado em código.
4. Resultados de benchmarks e métricas de desempenho.
5. Possíveis impactos em compatibilidade, efeitos colaterais e pontos de atenção para futuras manutenções ou regressões.

---

## Decisões Técnicas - 24/11/2026

### 1. Contextualização e Alinhamento da Especificação Oficial (HLA/RTI)
* **Histórico das Normas:** Analisamos a evolução desde a HLA 1.3 (DMSO), passando por IEEE 1516-2000, IEEE 1516-2010 (HLA Evolved) até a recente IEEE 1516-2025 (HLA-4).
* **Os Três Pilares:** Revisamos a divisão da norma entre IEEE 1516 (Regras), IEEE 1516.1 (API da RTI) e IEEE 1516.2 (OMT / FOMs).
* **A Engenharia da Pitch Technologies:** Estudamos a arquitetura do Pitch pRTI (descentralização via CRC executivo + LRC embutido em cada federado com P2P e Multicast), seu suporte multiversão e seu ecossistema fechado (Pitch Booster, Commander, Visual OMT, Gateway, Talk).

### 2. Identificação das Deficiências Críticas da Comunidade
Mapeamos as 7 dores centrais do mercado internacional de simulação:
1. Ausência de um *Wire Protocol* padronizado na norma (gerando *vendor lock-in* obrigatório).
2. Dificuldade severa em ambientes Cloud-Native / Kubernetes (bloqueio de IP Multicast em nuvens públicas).
3. Fricção de concorrência e memória ao integrar com Unreal Engine 5 e Unity.
4. Serialização arcaica baseada nos *Encoding Helpers* verbosos do padrão.
5. Inexistência de segurança granular / Zero Trust (o DDM filtra tráfego de rede, mas não provê proteção criptográfica).
6. Fragilidade no avanço temporal determinístico (*Time Management* paralisa a federação se um nó oscilar).
7. Vocabulário do RPR-FOM ainda preso a conceitos dos anos 1990 (DIS).

### 3. A Decisão do "Protocolo Unificado": Não Criar o 15º Padrão Isolado
* **O Dilema Analisado:** Discutiu-se se o projeto deveria criar um "protocolo de rede unificado".
* **Decisão Tomada:** Concluiu-se que criar um protocolo novo do zero apenas repetiria a armadilha do XKCD 927 ("criar o 15º padrão concorrente"). Além disso, implementar os protocolos da Pitch ou MAK é juridicamente inviável e tecnicamente frágil (segredos industriais fechados sob EULA).
* **A Estratégia Adotada:** Em vez de inventar um protocolo isolado, o Projeto Valhalla adota **compatibilidade nativa de rede com o PorticoRTI** (The Portico Project, de Tim Pokorny / Calytrix Technologies). Isso ancora o projeto em uma base de código aberto consolidada mundialmente, permitindo interoperabilidade imediata sem fricção.

### 4. Modelo de Licenciamento: Apache License 2.0
* **Análise Legal:** O Portico no GitHub adota a licença permissiva **Apache 2.0**.
* **Decisão Tomada:** O Projeto Valhalla foi formalmente licenciado sob a **Apache License, Version 2.0**.
* **Justificativa:** Diferente da GPL, a Apache 2.0 não é viral (*no copyleft*). Ela oferece segurança jurídica para adoção em Ministérios da Defesa, agências espaciais e integradores militares (como Embraer, Saab, Boeing, Lockheed), permitindo que incluam o Valhalla em plataformas de armas e simuladores classificados sem o receio de serem compelidos a abrir o código confidencial de suas aplicações.

### 5. Inovação: Serialização SSZ, Merkleização e Provas de Inclusão
* **Adoção do SSZ (Simple Serialize do Ethereum 2.0):**
  * Substituição dos *Encoding Helpers* manuais do padrão IEEE por layouts binários lineares com suporte a decodificação *zero-copy*.
  * **Merkleização Nativa (`hash_tree_root`):** Geração canônica de raiz de Merkle de 32 bytes para o estado de entidades da simulação.
  * **Detecção Instantânea de Dessincronização (*Desync Detection*):** Em simulações físicas determinísticas, federados comparam apenas o hash da raiz de Merkle a cada ciclo; qualquer discrepância de 1 bit é identificada no ato.
  * **Segurança Multinível (MLS / Zero Trust):** Uso de *Merkle Proofs* (provas de inclusão) para permitir que um federado prove matematicamente que sua posição geográfica pertence ao estado global do veículo, sem precisar revelar atributos confidenciais (ex: potência de radar secreta ou inventário de armas).

### 6. Criptografia Pós-Quântica: Primitiva KEM (Key Encapsulation Mechanism)
* **Requisito Estratégico:** Embora a IEEE 1516 não exija criptografia, doutrinas militares modernas (NIST FIPS 203, CNSA 2.0 da NSA e comitês da OTAN) exigem transição para PQC contra ataques *Harvest Now, Decrypt Later*.
* **Decisão Tomada:** Integração de **ML-KEM-768 (Kyber)** via padrão híbrido HPKE (RFC 9180).
* **Arquitetura em Duas Fases:**
  1. *Fase de Handshake:* KEM roda **uma única vez** ao conectar (custo de 15 a 40 microssegundos de CPU), encapsulando um segredo simétrico efêmero.
  2. *Fase de Streaming em Tempo Real (60 a 100 Hz):* Cifragem simétrica autenticada com **AES-256-GCM** via instruções de hardware do processador (**AES-NI**).
* **Impacto no Desempenho:** A latência adicional é inferior a **0,002 milissegundos (< 2 µs)** por pacote, permitindo gigabytes por segundo de vazão sem engasgos na simulação.

### 7. Resolução do Dilema de Compatibilidade: Despacho Duplo Segregado (*Dual-Stream Dispatch*)
* **O Problema Identificado:** Se tivermos 100 componentes conectados, sendo 99 nós modernos em SSZ e 1 nó legado do Portico clássico que espera o formato binário tradicional da SISO, como evitar que a rede inteira se rebaixe ao formato antigo?
* **Decisão Tomada:** Implementação do **Despacho Duplo Segregado (*Dual-Stream Dispatch*)**:
  1. Os 99 nós modernos continuam comunicando-se exclusivamente pelo canal rápido via **SSZ + Merkle Root + KEM**.
  2. Exclusivamente para o nó legado, o publicador gera sob demanda uma cópia codificada no padrão clássico **IEEE 1516 / RPR-FOM** e a entrega via unicast no canal tradicional do Portico (custo de CPU insignificante em Go de ~200 a 300 ns).
  3. O despacho duplo só afeta os atributos que o nó legado efetivamente assinou.

### 8. Integração com o Projeto Arcanjo (Docker)
* **Registro de Autoria:** Registrado que o desenvolvedor principal Carlos Magno é autor do **Projeto Arcanjo**, que implementa a conteinerização do Portico em Docker.
* **Decisão Técnica:** O contêiner do Projeto Arcanjo é formalmente adotado como o ambiente oracular para os testes de integração contínua (CI/CD) de rede da Sprint 1 em diante, eliminando a necessidade de mockups manuais da RTI.

### 9. Adoção e Extensibilidade do RPR-FOM v2.0
* **Nome Formal:** Esclarecido que RPR significa *Real-time Platform Reference FOM* (SISO-STD-001).
* **Natureza Aberta:** O RPR-FOM é 100% aberto e livre de royalties.
* **Regra de Ouro:** Os arquivos XML oficiais da SISO nunca serão alterados diretamente; qualquer extensão própria do Valhalla será implementada em módulos XML de extensão herdando das classes base oficiais.

---

### 10. Implementação Prática Concluída (Sprint 0)

1. **Estrutura do Projeto:**
   * Inicializado módulo Go: `github.com/icemagno/valhalla`.
   * Estruturada a árvore de diretórios conforme o padrão Go (`/cmd`, `/pkg/portico/wire`, `/pkg/portico/transport`, `/pkg/ssz`, `/pkg/crypto`, `/pkg/omt`, `/pkg/hla4`, `/test/resources/rpr-fom-v2.0`, `/test/integration`).
2. **Identidade Visual e Documentação:**
   * Criado o logotipo oficial em [assets/logo.svg](assets/logo.svg) (Escudo nórdico com 4 setas cardeais em laranja e verde).
   * Criado o [README.md](README.md) com badges da Apache 2.0, Go 1.25+, HLA-4, Portico e seção de autoria de Carlos Magno Oliveira de Abreu.
   * Documentos arquivados na pasta `/docs`: [deliberacoes.md](docs/deliberacoes.md) e [plano_de_projeto.md](docs/plano_de_projeto.md).
3. **Ingestão da Suíte RPR-FOM v2.0:**
   * Baixados os 14 módulos XML normativos oficiais da SISO (SISO-STD-001-2015) para `test/resources/rpr-fom-v2.0/`.
4. **Testes TDD e Validação:**
   * `test/resources_test.go`: Validação sintática e de tags raiz `<objectModel>` dos 14 esquemas XML do RPR-FOM. **Status: PASS (0.37s)**.

---

### 11. Homologação do Oráculo Portico (Projeto Arcanjo em Docker)
* **Resolução de Incompatibilidades no Build do Portico 3.0 / Master:**
  1. *Target Duplicado no Ant 1.10:* Renomeação de `<extension-point name="compile">` para `compile.ep` em `profiles/linux/hla13.xml` e `ieee1516e.xml`.
  2. *Typo em Dependência de Alvo:* Correção de `depends="compile.gcc11"` para `depends="compile"` em `profiles/linux/ieee1516e.xml`.
  3. *Incompatibilidade de Versão do JDK:* Fixação de `java.compiler.source = 11` e `target = 11` no `build.properties`.
  4. *Flag Inválida no Java 11:* Remoção da flag exclusiva do JDK 21 (`-proc:full`) em `profiles/system.macros.xml`.
  5. *Bug de Loop Infinito do RtiCli (NullPointerException):* Diagnóstico de que a execução em `-d` (sem tty) fechava o STDIN e fazia `console.readLine()` retornar `null`. Corrigido iniciando o container com as flags interativas `-dit`.
* **Validação de Conectividade em Rede:**
  * Container do Portico 3.0 operacional no endpoint `192.168.1.226`:
    * Porta TCP de Controle: `52295`
    * Canal Multicast/UDP: `20913`
  * Teste de conectividade TCP disparado da máquina de desenvolvimento (`192.168.1.160`) para `192.168.1.226:52295`: **`TcpTestSucceeded : True`**.

---

### 12. Auditoria Integral do Código-Fonte do Portico e Eliminação de Suposições
Em conformidade com a diretriz do desenvolvedor principal de **rejeitar código morto, stubs ou simulações falsas**, realizamos a auditoria direta das classes oficiais do Portico (`openlvc/portico`):
* `org.portico2.common.network.Header.java`
* `org.portico.utils.bithelpers.BitHelpers.java`
* `org.portico2.common.network.transport.tcp.channel.TcpChannel.java`
* `org.portico2.common.network.transport.tcp.channel.Bundler.java`
* `org.portico2.common.network.transport.tcp.TcpClientProxy.java`
* `org.portico.lrc.utils.MessageHelpers.java`

#### Descobertas Estruturais Críticas:
1. **Tamanho Real do Cabeçalho:** O cabeçalho fixo do Portico possui **exatamente 12 bytes** (`Header.HEADER_LENGTH = 12`), e não 24 bytes como em modelos hipotéticos prévios.
2. **Mapeamento Exato de Bits das Flags (Byte 0):**
   * Bit 0 (`0x01`): `isBundle`
   * Bit 1 (`0x02`): `isEncrypted` (indica presença de Nonce/IV)
   * Bit 2 (`0x04`): `isAuthenticated` (indica presença de Token)
   * Bit 3 (`0x08`): `isManualMarshal` (indica serialização binária direta sem reflexão Java)
   * Bit 4 (`0x10`): `isFiltering` (indica que `RequestId` opera como `FilteringId`)
   * Bit 5 (`0x20`): `isFilteringObjectClass` (1 = Objeto, 0 = Interação)
   * Bit 6 (`0x40`): Reservado Portico
   * Bit 7 (`0x80`): **Extensão Valhalla (`FlagValhallaExtended`)**
3. **Distribuição dos Nibbles do Byte 4:**
   * High Nibble (bits 7-4): `FederationID` (uint4, faixa 0..15).
   * Low Nibble (bits 3-0): `CallType` (uint4, enum `DataMessage=0`, `Notification=1`, `ControlRequest=2`, `ControlResponseOK=3`, `ControlResponseErr=4`).
4. **Enquadramento de Transporte TCP (*TCP Bundling*):**
   * O Portico nunca envia pacotes soltos no socket TCP. O `Bundler.java` e o `TcpChannel.java` encapsulam cada transmissão com o marcador mágico `0x0000CAFE` (`int32`), seguido pelo `bundleLength` (`int32`), seguido pelas mensagens contendo cabeçalho de 12 bytes e seus respectivos payloads.

---

### 13. Implementação e Validação Prática em Go (Sprint 1)

1. **Refatoração do Pacote `pkg/portico/wire`:**
   * `pkg/portico/wire/header.go`: Implementação rigorosa do cabeçalho de 12 bytes, alinhamento de bits Big-Endian, enums `CallType` e `MessageType`, métodos `MarshalTo` e `UnmarshalBinary`.
   * `pkg/portico/wire/header_test.go`: Bateria de testes unitários cobrindo roundtrip, limites de 24 bits de payload (16 MB), variações completas de 0 a 15 em FederationID e 0 a 4 em CallType.
   * **Resultados dos Benchmarks:**
     * `BenchmarkHeader_MarshalTo`: **3.22 ns/op** | **0 B/op** | **0 allocs/op**
     * `BenchmarkHeader_UnmarshalBinary`: **1.93 ns/op** | **0 B/op** | **0 allocs/op**
2. **Refatoração do Pacote `pkg/portico/transport`:**
   * `pkg/portico/transport/tcp_client.go`: Arquitetura full-duplex de rede em Go com mutexes segregados (`sendMu`, `recvMu`, `stateMu`), handshake oficial (`0xBEEF` + welcome + `0xFEED` <-> `0xFEED`) e enquadramento TCP bundle oficial (`0xCAFE`).
   * `pkg/portico/transport/tcp_client_test.go`: Servidor mock completo simulando tanto o handshake quanto a troca bidirecional de pacotes `0xCAFE`. **Status: PASS (0.34s)**.
3. **Teste de Integração Real contra o Oráculo Portico:**
   * `test/integration/live_portico_test.go`: Conexão ao vivo em `192.168.1.226:52295`.
   * Resposta oficial recebida: `[Portico Router (3.0.0 (build 0)): Your ID5]`.
   * Pacote `RtiProbe` (12 bytes, bundle `0xCAFE`) enviado e processado pelo servidor com sucesso, sem nenhuma ocorrência de `Broken pipe`. **Status: PASS (1.01s)**.

---

*Fim da entrada diária de 24/11/2026. Próximo marco: Implementação das mensagens de ciclo de vida (CreateFederation, JoinFederation, ResignFederation) com correlation ID e serialização manual.*


---

### 14. Engenharia do Protocolo JOS (Java Object Serialization) e Ingresso Real na Federação (Sprint 1)

1. **Diagnóstico do WARN `Couldn't convert byte[] (RtiProbe) into PorticoMessage`:**
   * O código oficial de `MessageHelpers.inflate2` do Portico exige que mensagens administrativas venham no envelope binário do `ObjectInputStream` do Java. Um payload vazio de 0 bytes disparava `EOFException` imediato na JVM.
2. **Implementação do Serializador JOS Canônico em Go (`pkg/portico/wire/jos.go`):**
   * Desenvolvido sem qualquer dependência de JVM ou runtime Java, implementando a especificação binária formal do Java Object Serialization (RFC/JOS):
     * Cabeçalho de stream: `STREAM_MAGIC` (`0xACED`) e `STREAM_VERSION` (`0x0005`).
     * `TC_BLOCKDATA` com `0x00` indicando ausência de `ManualMarshal`.
     * `TC_OBJECT` e `TC_CLASSDESC` para `PorticoMessage` (`serialVersionUID = 98121116105109L`, flags `0x03` = `SC_SERIALIZABLE | SC_WRITE_METHOD`).
     * Escrita dos 6 campos primitivos ordenados de `PorticoMessage` (`immediate`, `isFromRti`, `sourceFederate`, `targetFederate`, `targetFederation`, `timestamp`).
     * Descritor e valores para `JoinFederation` com as strings parametrizadas (`federateName`, `federateType`, `federationName`) e descritores de lista `ArrayList`.
3. **Validação Unitária (`pkg/portico/wire/jos_test.go`):**
   * Testes cobrindo validação de Magic, Versão, Tokens de Tipo, classes PorticoMessage/JoinFederation e integridade de nomes parametrizados. **Status: PASS (0.00s)**.
4. **Teste de Integração Real: Ingresso do Valhalla na `ExampleFederation` Viva:**
   * Executado `test/integration/live_portico_test.go` contra o contêiner Arcanjo (`192.168.1.226:52295`), onde a federação `ExampleFederation` estava ativa com o federado oficial Portico `Thor`:
     ```
     === RUN   TestIntegration_LivePortico_HandshakeAndProbe
         live_portico_test.go:19: Iniciando conexao ao vivo contra o executivo Portico no Arcanjo: 192.168.1.226:52295
         live_portico_test.go:33: >>> HANDSHAKE BEM-SUCEDIDO! Resposta oficial do Portico: [Portico Router (3.0.0 (build 0)): Your ID7]
         live_portico_test.go:43: Enviando RtiProbe (cabecalho 12 bytes + payload JOS 247 bytes, bundle 0xCAFE)...
         live_portico_test.go:61: Enviando JoinFederation na ExampleFederation (payload JOS 530 bytes)...
         live_portico_test.go:71: >>> RESPOSTA RECEBIDA DO PORTICO PARA JoinFederation: Tipo: JoinFederation(4), Call: ControlResponseOK(3), ReqId: 2, PayloadLen: 33495
         live_portico_test.go:76: Sessao finalizada com sucesso.
     --- PASS: TestIntegration_LivePortico_HandshakeAndProbe (2.54s)
     ```
   * **Resultado Histórico:** O CRTI Portico oficial processou a solicitação `JoinFederation` em Go e respondeu com **`ControlResponseOK (CallType = 3)`**, entregando o pacote oficial de boas-vindas (*WelcomePack*) com **33.495 bytes** contendo os metadados da federação e o handle do novo federado!

---

*Fim da entrada diária de 24/11/2026. Próximo marco: Decodificação do WelcomePack, registro de sincronização e avanço de tempo com o Thor.*
