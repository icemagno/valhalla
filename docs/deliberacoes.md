# Deliberações Arquiteturais e Estratégicas: Projeto Valhalla (HLA/RTI Next-Gen)

> **Projeto:** Projeto Valhalla  
> **Autor e Desenvolvedor Principal:** Carlos Magno Oliveira de Abreu (<magno.mabreu@gmail.com>)  
> **Identidade Visual:** Escudo nórdico com 4 setas cardeais (Norte, Sul, Leste, Oeste) em verde e laranja.  
> **Data de Registro:** 24 de Novembro de 2026  
> **Finalidade:** Registro exaustivo e integral das deliberações, análises técnicas, modelos de licenciamento, padrões internacionais e decisões arquiteturais tomadas para o desenvolvimento de uma infraestrutura moderna de simulação baseada em HLA (High Level Architecture) e RTI (Run-Time Infrastructure).

---

## Sumário Executivo
Este documento consolida todos os tópicos discutidos a respeito da especificação oficial da HLA e RTI, da engenharia por trás do **Pitch pRTI**, das carências críticas sentidas pela comunidade internacional de simulação, da incorporação inovadora de estruturas criptográficas e de serialização originárias de blockchain (**SSZ** e árvores de Merkle), da análise de viabilidade do **RPR-FOM**, da transição de formatos (**XML vs. JSON / HLA 4**) e da estratégia mestra de compatibilidade nativa com o ecossistema aberto do **Portico** (criado por Tim Pokorny / Calytrix Technologies).

---

## 1. A Especificação Oficial: HLA e RTI

A **High Level Architecture (HLA)** é um padrão internacional de arquitetura para simulação distribuída desenvolvido para promover a interoperabilidade e o reúso de simuladores e componentes de software. É mantida pela **SISO** (*Simulation Interoperability Standards Organization*) e padronizada pelo **IEEE**.

### 1.1. Evolução Histórica das Normas
1. **HLA 1.3 (DMSO - 1998):**
   * Padrão legado original patrocinado pela *Defense Modeling and Simulation Office* (DoD/EUA).
   * Utilizava arquivos de modelo em formato textual próprio (`.fed`) e APIs em C++ com gerenciamento de memória manual e arcaico. Ainda é encontrado em sistemas militares antigos em operação.
2. **IEEE 1516-2000:**
   * Primeira padronização formal sob a égide do IEEE.
   * Introduziu a estruturação formal dos modelos de dados em XML.
3. **IEEE 1516-2010 (HLA Evolved):**
   * O padrão comercial e militar mais amplamente utilizado no mundo até hoje.
   * **Inovações cruciais:** Introdução de **FOMs Modulares** (divisão de modelos em múltiplos arquivos XML combináveis), suporte a ponteiros inteligentes (*Smart Pointers* em C++), classes utilitárias de codificação (*Encoding Helpers*), suporte formal a WSDL/Web Services e transporte com tolerância a falhas.
4. **IEEE 1516-2025 (HLA 4):**
   * A revisão mais recente da norma.
   * Foco primordial: Computação em nuvem (*Cloud-Native*), arquitetura orientada a serviços (*Modelling and Simulation as a Service - MSaaS*), perfis para federados web leves conectando via **WebSockets/REST** e representação de modelos de dados em **JSON Schema**.

### 1.2. Os Três Pilares da Norma IEEE 1516
* **IEEE 1516 (Framework and Rules):** Define os 10 princípios fundamentais (5 regras mandatórias para Federações e 5 regras para Federados).
* **IEEE 1516.1 (Federate Interface Specification):** A especificação formal da API do *middleware* RTI. Define como o código do federado faz chamadas para a RTI (`RTIambassador`) e como a RTI responde ao federado via chamadas de retorno assíncronas (`FederateAmbassador`).
* **IEEE 1516.2 (Object Model Template - OMT):** A sintaxe formal e a estrutura tabular/esquemática para definição dos modelos de dados trocados na simulação:
  * **FOM (Federation Object Model):** O contrato de dados compartilhado por toda a federação.
  * **SOM (Simulation Object Model):** O subconjunto de dados que um simulador específico é capaz de publicar ou assinar.
  * **MIM (Management Object Model):** Objetos e interações internos de infraestrutura mantidos pela própria RTI para auditoria e controle.

### 1.3. Os Sete Grupos de Serviços da RTI (IEEE 1516.1)
1. **Federation Management:** Ciclo de vida da federação (criação, destruição, ingresso/`join`, saída/`resign`, pontos de sincronização coletivos e rotinas de salvar/restaurar estado global).
2. **Declaration Management:** Mecanismo de publicação e assinatura (*Pub/Sub*) onde federados registram interesse em classes de objetos e interações.
3. **Object Management:** Criação, registro e descoberta dinâmica de instâncias de objetos no espaço virtual; envio e recebimento de atualizações de atributos e disparo de interações (eventos transitórios).
4. **Ownership Management:** Transferência dinâmica e negociada do controle de atributos específicos entre federados (ex.: um míssil lançado por um avião que passa a ser calculado pelo simulador de balística).
5. **Time Management:** Coordenação do avanço temporal lógico da simulação. Permite simulação determinística com garantia de ordenação de eventos por carimbo de tempo (*Time-Stamp Order - TSO*), utilizando políticas de regulação temporal (*Time Regulating*) e restrição temporal (*Time Constrained*), baseadas no cálculo matemático do LBTS (*Lower Bound on Time Stamp*) e no horizonte de previsão (*Lookahead*).
6. **Data Distribution Management (DDM):** Mecanismo de filtragem espacial multidimensional (*routing spaces* e regiões de interesse). Reduz o tráfego de rede garantindo que um federado só receba dados gerados em setores do espaço virtual onde ele possui interesse de assinatura ativo.
7. **Support Services:** Métodos auxiliares de manipulação de identificadores (*handles*), conversões de nomes e configurações de transporte.

---

## 2. A Engenharia da Pitch Technologies (Pitch pRTI e Ferramentas)

A empresa sueca **Pitch Technologies** (atualmente parte da BAE Systems) estabeleceu-se como a líder de mercado em conformidade estrita e desempenho para a norma HLA.

### 2.1. Arquitetura Técnica Descentralizada
O Pitch pRTI evita o gargalo de servidores centrais por meio de uma arquitetura híbrida dividida em dois componentes:
* **Pitch RTI Executive (Central RTI Component - CRC):** Processo coordenador central de baixo consumo de recursos. Responsável pela coordenação do ciclo de vida da federação, validação de esquemas XML de FOM, resolução inicial de nomes e pontos de sincronização.
* **Local RTI Component (LRC):** Biblioteca de alto desempenho embutida no próprio processo de cada federado.
  * O tráfego pesado de atributos e interações **não passa pelo executivo central**.
  * A comunicação de dados em tempo real ocorre de forma direta entre os LRCs por meio de **IP Multicast otimizado** e conexões ponto a ponto (**P2P via UDP/TCP**).
  * Reduz a latência para sub-milissegundos e atinge taxas de transferência na ordem de centenas de milhares de atualizações por segundo.

### 2.2. Flexibilidade e Convivência Multiversão
* **Executivo Multi-Padrão:** O mesmo executivo do Pitch pRTI permite que federações conectem simultaneamente federados legados em **HLA 1.3**, federados em **IEEE 1516-2000** e federados modernos em **IEEE 1516-2010 (HLA Evolved)**.
* **Bindings de Linguagem:** Oferece suporte nativo com bibliotecas pré-compiladas para C++ (32 e 64 bits para Windows e diversas distros Linux), Java, .NET / C# e interfaces para Web.
* **Validação de FOMs Modulares:** Motor de análise léxica e validação de esquemas XML que une árvores de herança complexas no momento do `createFederationExecution`.

### 2.3. O Ecossistema Comercial da Pitch
A Pitch não comercializa apenas a RTI, mas uma suíte integrada que domina contratos militares:
* **Pitch Booster:** Módulo de aceleração para redes de longa distância (WAN/Internet). Aplica compressão adaptativa, agrupamento de pequenos pacotes UDP em blocos maiores e técnicas de correção antecipada de erros (FEC) para superar a perda de pacotes e a alta latência entre bases militares geograficamente dispersas.
* **Pitch Commander:** Aplicação gráfica para monitoramento em tempo real do ecossistema da federação: exibe topologia de nós, taxas de transmissão, atraso temporal entre federados e comandos para forçar sincronização ou encerramento de nós.
* **Pitch Visual OMT:** IDE visual para criação, edição estruturada, herança e validação de arquivos FOM e SOM em conformidade com o padrão IEEE 1516.2.
* **Pitch Gateway:** Software comercial de alto valor para conversão de protocolos e interconexão de redes: faz pontes entre federações HLA e redes baseadas em **DIS (Distributed Interactive Simulation / IEEE 1278)**, ou entre duas federações HLA independentes.
* **Pitch Talk:** Solução de rádio e comunicações táticas simuladas (rádio militar, intercomunicadores e canais de dados táticos).

---

## 3. Principais Deficiências e Demandas Não Resolvidas da Comunidade

Apesar da consolidação da HLA, a indústria de defesa, desenvolvedores de simulação e centros de pesquisa enfrentam gargalos crônicos decorrentes de decisões tomadas nos anos 1990 e início dos anos 2000:

| Área / Necessidade | O Problema Atual na HLA/RTI | Impacto na Prática | O que a Comunidade Precisa |
| :--- | :--- | :--- | :--- |
| **Protocolo de Rede Padronizado (*Wire Protocol*)** | A norma IEEE 1516 só padroniza a API em código, não os pacotes que trafegam no cabo de rede. | **Vendor Lock-in compulsório:** nós compilados com a Pitch não conversam na rede com nós da MAK ou do Portico sem pontes/gateways pesados. | Um protocolo aberto padronizado na camada de rede (como o RTPS faz no padrão concorrente DDS). |
| **Ambientes de Nuvem e Contêineres (Cloud-Native / K8s)** | A arquitetura clássica assume redes locais (LAN) e IP Multicast nativo. Nuvens públicas (AWS, Azure, GCP) bloqueiam multicast por padrão. | Dificuldade imensa para rodar simulações em contêineres Docker, malhas de microsserviços (Kubernetes) ou nós com IPs dinâmicos. | Descoberta automática de nós nativa para nuvem e transporte por protocolos modernos (gRPC, QUIC, WebSockets). |
| **Integração com Game Engines (Unreal 5 / Unity)** | As bibliotecas C++ das RTIs comerciais usam modelos de concorrência e memória que entram em conflito com o ciclo de vida (*Tick/Render*) das engines. | Equipes perdem meses escrevendo *wrappers* caseiros complexos para evitar congelamento de renderização e conflitos de memória/DLL. | Plugins oficiais, pré-compilados e idiomáticos para Unreal e Unity com tratamento assíncrono nativo. |
| **Serialização Ágil e Produtividade** | O OMT/XML é pesado e os *Encoding Helpers* do padrão IEEE 1516 exigem dezenas de linhas de código manual para empacotar estruturas de dados básicas. | Desenvolvimento lento, código propenso a falhas de alocação de memória e baixa produtividade na codificação do modelo de dados. | Geradores automáticos de código baseados em esquemas binários rápidos, determinísticos e modernos (*SSZ*, *FlatBuffers*, *Protobuf*). |
| **Segurança Granular / Zero Trust (MLS)** | O ingresso na federação garante acesso a todo o tráfego publicado. O DDM apenas otimiza o roteamento, **não é barreira de segurança criptográfica**. | Em exercícios militares multinacionais (ex: OTAN), países aliados não conseguem ocultar dados sigilosos de radar/guerra eletrônica sem hardware físico externo caro (*Cross-Domain Solutions*). | Criptografia por atributo e controle de acesso fino (ABAC), permitindo compartilhar a posição global do caça, mas restringir parâmetros secretos da carga útil. |
| **Resiliência e Tolerância a Falhas (*Time Management*)** | No avanço determinístico de tempo, se um único federado oscilar na rede ou travar, o cálculo do LBTS congela **a federação inteira**. | Simulações grandes são vulneráveis a quedas de nós individuais; inserir nós atrasados (*late-joining*) sem paralisar a simulação é quase inviável. | Mecanismos de *heartbeat*, expulsão automática de nós não responsivos e sincronização dinâmica de estado para novos entrantes. |
| **Modelos de Domínio Modernos (RPR-FOM)** | O vocabulário padrão da indústria militar (RPR-FOM) reflete amplamente o protocolo DIS dos anos 1990 (focado em tanques, navios e aeronaves convencionais). | Modelar Guerra Eletrônica profunda, Cibersegurança, Espaço e enxames massivos de milhares de drones autônomos de IA fica lento e artificial. | Um modelo de dados oficial de alta densidade voltado a Operações Multidomínio (MDO) e agentes de IA/enxames. |

---

## 4. Inovação: Aplicação de Conceitos de Blockchain e o Serializador SSZ (Simple Serialize)

A integração de primitivas criptográficas e de estruturas de dados concebidas para a camada de consenso da Web3 (especificamente o **SSZ** do Ethereum 2.0) apresenta um caminho técnico viável e revolucionário para superar as limitações do HLA:

### 4.1. O Serializador SSZ (Simple Serialize)
O SSZ foi projetado para substituir serializadores complexos e pesados, oferecendo características sob medida para simulação em tempo real:
* **Determinismo e Canonicidade Estrita:** Cada estrutura de dados tem uma representação binária única e invariante.
* **Merkleização Nativa (`hash_tree_root`):** Qualquer estrutura serializada em SSZ pode ser instantaneamente convertida em uma árvore binária de Merkle, produzindo uma raiz de hash única que resume todo o estado.
* **Layout de Memória Linear (*Zero-Copy*):** Tipos fixos são empacotados sequencialmente no início do buffer, seguidos por deslocamentos (*offsets*) para tipos variáveis. Permite leitura direta de atributos da memória sem alocações intermediárias de objetos.

### 4.2. Aplicações Práticas no RTI
1. **Detecção Imediata de Dessincronização (*Desync Detection*):**
   * Em simulações físicas determinísticas (*Lockstep* ou *Time-Managed*), os federados calculam o estado de milhares de entidades de forma autônoma.
   * Em vez de transmitir todos os dados pela rede para comparar estados, os federados trocam apenas a raiz de Merkle de 32 bytes (`hash_tree_root`). Se um único bit de uma coordenada divergir entre o simulador do piloto e o simulador da torre, a raiz será diferente, acusando o *desync* no ato e permitindo rastrear o nó exato da árvore que divergiu.
2. **Provas de Merkle e Revelação Seletiva para Multi-Level Security (MLS):**
   * Usando **Merkle Proofs** (ou *Multiproofs*), um federado pode publicar a raiz de Merkle comprovando que o seu veículo possui determinado estado global e revelar apenas o atributo de `Posição Geográfica` acompanhado de uma prova matemática.
   * O federado receptor valida a veracidade da posição contra a raiz global sem jamais ter acesso a atributos confidenciais contidos nos outros galhos da árvore (como frequências secretas de radar ou inventário de armas).
3. **Auditoria Imutável e Não-Repúdio para AAR (*After Action Review*):**
   * Eventos de combate e interações de disparo/detonação são encadeados criptograficamente com assinaturas digitais leves (ex: Ed25519).
   * Garante a integridade absoluta da reconstituição pós-missão: nenhum participante do exercício militar pode adulterar os logs após o término para esconder erros táticos ou fogo amigo.
4. **Eliminação de SPOF com Consenso Leve (Raft / BFT):**
   * Substituição do processo centralizador único (*RTI Exec*) por um comitê de federados rodando consenso distribuído de alta velocidade para arbitrar o ciclo de vida e a posse de objetos (*Ownership*).
5. **Zero-Knowledge Proofs (ZKP) para Engajamentos de Guerra Eletrônica:**
   * Federados em exercícios multinacionais podem rodar algoritmos verificáveis onde o míssil prova formalmente que obteve travamento de mira (*lock-on*) baseado na assinatura de radar do alvo, **sem expor a fórmula secreta da seção reta radar (RCS) nem a potência confidencial do sensor**.

### 4.3. Restrições Fundamentais (O que NÃO importar do Blockchain)
* **Sem Prova de Trabalho (Proof of Work - PoW):** Inviável pelo consumo energético e lentidão computacional.
* **Sem Latência de Blocos:** Simulação militar distribuída exige taxas de atualização de **30 Hz a 100 Hz** com latência inferior a **10 milissegundos**. O hashing e a Merkleização devem ocorrer de forma assíncrona ou em intervalos temporais estratégicos (*checkpoints*), enquanto o fluxo contínuo de coordenadas permanece trafegando por canais de alta velocidade.

---

## 5. A Análise do "Protocolo Unificado": O Risco da Fragmentação

Ao analisar a proposição de criar um "protocolo de rede unificado para a HLA", concluiu-se que:

1. **A Armadilha do 15º Padrão (Síndrome XKCD 927):** Criar um novo protocolo de rede do zero sem que ele seja aprovado formalmente pela SISO ou pelo IEEE resultará apenas em mais um protocolo isolado e incompatível com o resto do mundo.
2. **Inviabilidade de Implementar os Protocolos Proprietários da Pitch ou da MAK:**
   * Os protocolos de rede da Pitch e da MAK são **segredos industriais fechados e patenteados**.
   * Não existe documentação aberta sobre a estrutura de seus pacotes.
   * Engenharia reversa desses protocolos viola termos de licença (EULA), acarreta riscos jurídicos graves e cria uma arquitetura frágil, sujeita a quebrar a cada nova atualização de versão dos fabricantes.
3. **A Solução Prática Adotada pelo Mercado:**
   * **Padronização na API (IEEE 1516.1):** O código-fonte do simulador é universal. A troca de fornecedor de rede é realizada via substituição da biblioteca dinâmica vinculada (`.dll` ou `.so`), sem alteração no código do federado.
   * **Gateways de Federação:** Utilização de programas híbridos atuando como nós pontes entre federações heterogêneas.
   * **Adoção de Padrões Abertos Consolidados:** Utilização de padrões de transporte já consagrados internacionalmente (como o protocolo **RTPS** da especificação DDS/OMG) como camada de rede aberta.

---

## 6. Modelos de Licenciamento da Pitch e Estratégias Comerciais

### 6.1. As Restrições Comerciais da Pitch
* O **Pitch pRTI** e o **Pitch Gateway** são produtos proprietários, comerciais e fechados.
* Exigem licenças de desenvolvimento e licenças de tempo de execução atreladas a travas físicas (*dongles* USB) ou servidores de licença flutuante (*FlexLM*).
* É estritamente **proibido redistribuir** as bibliotecas dinâmicas (`.dll` / `.so`) da Pitch embutidas no instalador de softwares de terceiros sem a contratação de acordos formais e custosos de OEM (*Original Equipment Manufacturer*).

### 6.2. Estratégias Legais de Engenharia e Mercado
* **Modelo BYOL (*Bring Your Own License*):** O software do simulador é desenvolvido e compilado estritamente contra os cabeçalhos padrão da norma **IEEE 1516.1**. O cliente final (ex.: Marinha, Força Aérea, Embraer) fornece o ambiente de execução e a licença da Pitch que ele já possui em sua infraestrutura. O software carrega a biblioteca dinamicamente via `LoadLibrary` / `dlopen`.
* **Desenvolvimento de Gateway Próprio:** Em vez de adquirir a licença do software *Pitch Gateway*, desenvolve-se um federado em C++ ou Java que se conecta a duas federações simultâneas utilizando as APIs padrão. Esse programa consome apenas uma licença padrão de federado do cliente.
* **Adoção de RTIs de Código Aberto:** Utilização de infraestruturas livres para distribuição irrestrita (como **Portico** e **CERTI**).

---

## 7. O RPR-FOM (Real-time Platform Reference FOM)

* **Identificação Correta:** A sigla RPR significa **Real-time Platform Reference FOM** (SISO-STD-001). A pronúncia fonética comum em inglês assemelha-se a *"reaper"*, gerando confusão ocasional com sistemas não tripulados, mas trata-se do padrão de referência de plataformas em tempo real.
* **Natureza Aberta:** O RPR-FOM é um **padrão internacional 100% aberto e gratuito**, desenvolvido e distribuído pela **SISO**. Não há royalties, taxas de licença ou restrições de uso comercial para seus esquemas e especificações.
* **Adoção Prática:** É a **língua franca obrigatória** para simulação militar distribuída no mundo ocidental. Adotá-lo garante interoperabilidade imediata com simuladores consolidados (*MAK VR-Forces*, *Bohemia Interactive VBS*, simuladores da *Saab/Gripen*, *Thales* e padrões OTAN/STANAG).

### 7.1. Estrutura Modular do RPR-FOM v2.0 (SISO-STD-001-2015)
O modelo é distribuído em módulos XML independentes:
* `RPR-Base.xml`: Definições fundamentais de tipos de dados, vetores, identificadores e convenções de coordenadas globais (Geocêntricas ECEF / WGS-84).
* `RPR-Physical.xml`: Hierarquia de entidades físicas móveis (Aeronaves, Navios, Veículos Terrestres, Sensores).
* `RPR-Warfare.xml`: Interações de combate (disparos de armamento e detonações).
* `RPR-Radio.xml`: Comunicações de rádio tático, frequências e enlaces de dados (Link 16).
* `RPR-SimManagement.xml`: Controle de execução da simulação (iniciar, congelar, resetar).
* `RPR-Logistics.xml`: Ações de suprimento, manutenção e reparo de danos em combate.

### 7.2. Regra de Engenharia para Extensibilidade
**Os arquivos XML oficiais do RPR-FOM nunca devem ser modificados diretamente.** 
A abordagem correta sob a norma IEEE 1516-2010 é a criação de um **módulo de extensão próprio** (ex.: `Extensao-Custom.xml`) que importe os módulos padrão da SISO e crie subclasses com atributos adicionais. Assim, sistemas legados continuam enxergando a entidade básica pelo RPR-FOM, enquanto sistemas novos processam os dados estendidos.

---

## 8. Formatos de Modelo: XML vs. JSON e a Realidade do HLA 4

### 8.1. O Domínio Atual do XML
* Na prática de mercado atual, **mais de 99% de todos os FOMs em uso ativo são arquivos XML** validados contra o esquema `HLA.xsd` (norma IEEE 1516-2010).
* Todo o ecossistema existente da SISO (incluindo o RPR-FOM oficial) e todas as ferramentas consagradas de edição (*Pitch Visual OMT*, editores da MAK) operam nativamente sobre XML.

### 8.2. Onde o JSON Atua no HLA 4 (IEEE 1516-2025)?
O novo padrão HLA 4 introduziu o JSON em dois domínios distintos:
1. **Representação Alternativa do FOM (JSON Schema):** A norma padronizou formalmente como descrever o OMT usando JSON Schema em vez de XML, atraindo desenvolvedores que utilizam Python, TypeScript e Go.
2. **Tráfego de Dados na Rede para Web Federates:** A inovação mais disruptiva do HLA 4. No modelo clássico, os atributos viajam como bytes binários brutos; no perfil Web do HLA 4, a RTI provê interfaces **WebSockets/REST** onde os dados transitam formatados diretamente como **objetos JSON**, permitindo a criação de interfaces web (React, Vue, Node.js) sem compiladores C++.

### 8.3. Diretriz para a Nova Implementação
* **Compatibilidade Obrigatória:** O motor deve processar **XML** para ser capaz de carregar o RPR-FOM e os modelos de mercado existentes.
* **Diferencial de Mercado:** Oferecer suporte nativo à ingestão de modelos em **JSON** e expor interfaces de dados via JSON/WebSockets coloca a arquitetura na vanguarda da norma HLA 4.

---

## 9. Status de Implementação do HLA 4 no Mercado Global

O padrão HLA 4 (IEEE 1516-2025) encontra-se em fase de **Adoção Inicial (*Early Adoption*)**:
* **Pitch Technologies:** Pioneira na demonstração de protótipos funcionais, impulsionada pelo fato de seus líderes técnicos terem coordenado a redação da norma no comitê SISO PDG.
* **MAK Technologies (VT MAK):** Desenvolveu suporte no *MAK RTI* e integrou os requisitos web à sua solução corporativa *WebLVC*.
* **Fraunhofer IOSB e OTAN:** O instituto alemão Fraunhofer, juntamente com os grupos da OTAN (NATO MSG-164 / MSG-197), construiu bancos de teste de validação para comprovar a viabilidade de simulações militares em nuvem (*MSaaS*) antes da homologação final da norma.
* **Projetos Open Source:** Iniciativas como o Portico monitoram a evolução através de branches de teste, mas a consolidação completa no mundo aberto é tradicionalmente mais lenta.

---

## 10. O Ecossistema Portico e a Calytrix Technologies (Tim Pokorny)

* **Origem:** O **Portico** foi idealizado e arquitetado principalmente por **Tim Pokorny** (cofundador e CTO da **Calytrix Technologies**, Austrália) juntamente com sua equipe de engenharia.
* **Modelo da Calytrix:** A empresa disponibilizou a RTI do Portico em código aberto para democratizar a tecnologia HLA mundialmente e monetizou vendendo serviços de suporte, engenharia de integração e produtos especializados que operam sobre a RTI (como o *LVC Game*, que conecta simuladores comerciais e motores como Unreal Engine ao barramento militar, além da linha de rádios *CNR-Sim*).

### 10.1. Adoção Oficial do Portico por Entidades Globais
O Portico é amplamente utilizado por instituições de grande porte:
1. **Forças Armadas da Austrália (ADF & DSTG):** Utilizado diretamente em programas militares conjuntos de treinamento distribuído (LVC).
2. **NASA (EUA):** Adotado em centros espaciais para simulação robótica e estudos de exploração espacial distribuída entre múltiplos centros e universidades, eliminando custos de licenciamento por assento.
3. **Grupos de Trabalho da OTAN (NATO STO / MSG):** Escolhido frequentemente como a RTI neutra de referência em projetos multinacionais de pesquisa para evitar que países-membros sejam forçados a adquirir licenças proprietárias para participar dos experimentos.
4. **Grandes Integradores de Defesa:** Utilizado globalmente nos ambientes de desenvolvimento local de engenheiros e em esteiras de integração contínua (DevOps / CI/CD no GitLab e GitHub) para rodar testes automatizados sem consumir as licenças comerciais da Pitch ou MAK.

---

## 11. Estratégia Mestra do Projeto: Inovação + Compatibilidade Nativa com o Portico

A estratégia técnica e de mercado definida consiste em **construir uma RTI inovadora e de alta performance, mas que mantenha compatibilidade nativa de rede (*wire compatibility*) com o ecossistema Portico**.

### 11.1. Viabilidade Jurídica
* O Portico moderno é distribuído sob a **Apache License 2.0**.
* Trata-se de uma licença altamente permissiva que autoriza a leitura da base de código, a reprodução compatível de suas estruturas de dados de rede, a extensão de funcionalidades e a comercialização de produtos derivados sem exigência de pagamento de royalties ou abertura compulsória de código proprietário.

### 11.2. A Arquitetura em Três Camadas

```
+---------------------------------------------------------------------------------+
| CAMADA 3: Modelos & Serialização Moderna                                       |
| - Tipos de dados em SSZ (Simple Serialize) e FlatBuffers                        |
| - Verificação de estado por Raiz de Merkle (hash_tree_root)                    |
| - Conformidade total com RPR-FOM v2.0 (módulos XML oficiais e extensões)        |
+---------------------------------------------------------------------------------+
| CAMADA 2: Serviços Avançados HLA e Conectividade Híbrida                        |
| - Suporte à especificação HLA 4 (WebSockets, JSON, Web Profiles)                |
| - Conectores assíncronos dedicados para Unreal Engine 5 e Unity                |
| - Negociação de Capacidades no Handshake (Feature Negotiation)                  |
+---------------------------------------------------------------------------------+
| CAMADA 1: Núcleo de Transporte de Rede                                          |
| - Compatibilidade binária nativa com o protocolo do Portico (PorticoMessage)    |
| - Suporte a transporte direto P2P / UDP / TCP e descoberta transparente        |
+---------------------------------------------------------------------------------+
```

### 11.3. Mecanismos de Integração Técnica
1. **Transparência de Payload:**
   * Para a camada de transporte do Portico, o conteúdo de um atributo ou interação é um array de bytes opaco (`byte[]` / `VariableLengthData`).
   * Nós modernos desenvolvidos com esta nova arquitetura podem trafegar dados serializados em **SSZ** através da rede do Portico sem que o Portico original sofra qualquer erro de decodificação.
2. **Negociação de Recursos no Handshake (*Feature Negotiation*):**
   * Ao ingressar na federação, o nó anuncia suas capacidades através de metadados de conexão.
   * Se o par for um nó Portico padrão do Tim Pokorny, o sistema desativa as rotinas proprietárias e opera no dialeto clássico do Portico.
   * Se o par for outro nó de nossa arquitetura moderna, o canal de dados ativa a sincronização avançada de raízes de Merkle, verificação criptográfica e transporte acelerado.
3. **Nó Ponte Híbrido (Portico na LAN <-> Nuvem/Web):**
   * O sistema atua como um nó legítimo Portico na rede local física, mas abre simultaneamente portas para o mundo exterior oferecendo **HLA 4 sobre WebSockets/JSON**, permitindo que navegadores e nós na nuvem acessem a simulação do Portico sem atrito.

### 11.4. Proposta de Valor e Posicionamento Comercial
Esta estratégia resolve simultaneamente os problemas de isolamento e custo:
> *"Conecte seu simulador nativamente à base instalada e gratuita do Portico sem alterar nenhuma linha de código existente, mas agregue suporte oficial a Unreal Engine 5, facilidade de conexão em nuvem, compatibilidade com HLA 4 e integridade criptográfica verificável com SSZ que o Portico original e as soluções comerciais fechadas não oferecem."*

---

## 12. Criptografia KEM (Key Encapsulation Mechanism) e Segurança Pós-Quântica (PQC)

### 12.1. O que é KEM e Qual Seu Papel na Arquitetura?
O **Key Encapsulation Mechanism (KEM)** é uma primitiva de criptografia assimétrica projetada especificamente para estabelecer com segurança chaves simétricas compartilhadas através de um canal desprotegido.
* Diferente da criptografia assimétrica tradicional de streaming (que é lenta e inadequada para grandes volumes de dados), o KEM opera em **duas fases estritas**:
  1. **Fase de Estabelecimento (Handshake):** O KEM roda **uma única vez** no momento da conexão para encapsular e derivar uma chave simétrica efêmera (ex: AES-256 de 32 bytes).
  2. **Fase de Fluxo em Tempo Real (Streaming):** Todos os pacotes de alta frequência subsequentes (60 Hz a 100 Hz) são cifrados usando algoritmos simétricos autenticados (**AEAD** como **AES-256-GCM** ou **ChaCha20-Poly1305**).

### 12.2. É um Requisito da Norma HLA?
* **Pela IEEE 1516:** **Não.** O padrão HLA delega a segurança à rede subjacente.
* **Pela Doutrina de Defesa Moderna (OTAN / EUA / Brasil):** **Sim, em vias de obrigatoriedade.**
  * As diretivas de segurança nacional (como a **CNSA 2.0** da NSA e comitês de interoperabilidade da OTAN) exigem a transição mandatória para **Criptografia Pós-Quântica (PQC)** para mitigar ataques *Harvest Now, Decrypt Later* (HNDL).
  * O padrão oficial de KEM pós-quântico selecionado pelo NIST é o **ML-KEM (Kyber / FIPS 203)**.
  * A adoção de ML-KEM confere à RTI um diferencial competitivo inédito no mercado de defesa global.

### 12.3. Impacto na Velocidade e Latência de Transmissão
A análise matemática e computacional comprova que **não há degradação no throughput nem na latência** da simulação em tempo real:
* **No Handshake:** O custo de execução do ML-KEM-768 é de aproximadamente **15 a 40 microssegundos** de CPU em Go, com overhead de rede de ~1 KB trocado apenas no aperto de mão inicial.
* **No Tráfego Contínuo:** Utilizando **AES-256-GCM**, a cifragem e decifragem são executadas diretamente pelas instruções de hardware do processador (**AES-NI** em Intel/AMD e ARMv8 Crypto Extensions).
* **Throughput:** Atinge taxas superiores a **3 a 5 GB/s por núcleo de CPU**, adicionando menos de **0,002 milissegundos (< 2 microssegundos)** de latência por pacote transmitido.

### 12.4. Integração do KEM na Convivência com o Portico
1. **Entre Nós Modernos (Next-Gen <-> Next-Gen):**
   * Realizam o aperto de mão KEM via protocolo híbrido HPKE (RFC 9180) e cifram o canal de transporte ponto a ponto com AES-256-GCM.
2. **Em Malhas Mistas (com Nós Portico Java Legados):**
   * O Portico tradicional não compreende KEM.
   * **Cifragem de Granularidade Fina (MLS por Atributo):** Os nós modernos utilizam o KEM para derivar chaves de grupo de segurança (*Group Domain Keys*). Atributos confidenciais do RPR-FOM têm seus bytes cifrados individualmente antes de serem inseridos no `byte[]` padrão do Portico.
   * O Portico Java atua como repetidor cego de rede, incapaz de ler o conteúdo sigiloso, garantindo confidencialidade absoluta em redes heterogêneas sem violar o protocolo legado.

---

## 13. Despacho Duplo Segregado (Dual-Stream Dispatch) e o Princípio da Não-Degradação

### 13.1. O Problema do Menor Denominador Comum
Em um cenário heterogêneo onde existam 100 federados conectados (99 nós modernos do Projeto Valhalla e apenas 1 nó legado do Portico clássico), uma abordagem ingênua forçaria todos os nós a se rebaixarem ao formato clássico do IEEE 1516, anulando as vantagens do SSZ, da Merkleização e do KEM.

### 13.2. A Solução: Despacho Duplo Segregado (*Dual-Stream Dispatch*)
O Projeto Valhalla adota a segregação por canal e destinatário no momento da publicação:
1. **Os Nós Modernos Mantêm Desempenho Máximo:** O nó publicador gera o buffer em **SSZ com raiz de Merkle e KEM** e transmite pelo canal de alta velocidade (UDP/QUIC) para os 99 nós modernos. Nenhuma funcionalidade de ponta é sacrificada.
2. **Entrega Cirúrgica ao Nó Legado:** Exclusivamente para o nó legado registrado, o publicador gera uma cópia codificada no formato padrão **IEEE 1516 / RPR-FOM clássico** (custo desprezível em Go de ~200 a 300 nanossegundos de CPU) e a entrega via canal unicast do Portico.
3. **Escopo Restrito à Assinatura:** Essa via dupla só é disparada para os atributos específicos que o nó legado efetivamente assinou. O restante do tráfego da federação permanece 100% no modo moderno.

---

## 14. Decisão de Licenciamento do Projeto Valhalla: Apache License 2.0

### 14.1. Definição Formal da Licença
Fica deliberado e homologado que o **Projeto Valhalla** será distribuído integralmente sob a **Apache License, Version 2.0**.

### 14.2. Justificativas Estratégicas
1. **Adoção Irrestrita pela Indústria e Governo:** A licença Apache 2.0 é o padrão de ouro internacional para infraestrutura aberta corporativa (adotada por Kubernetes, Apache Foundation, Linux Foundation e o próprio Portico). Ela oferece segurança jurídica para adoção em órgãos governamentais e forças armadas.
2. **Proteção de Patentes e Não-Contaminação Viral:** Ao contrário da GPL, a Apache 2.0 não possui cláusula viral de *copyleft*. Empresas e integradores de defesa podem incorporar o Valhalla em seus sistemas de armas e simuladores sem o receio de serem compelidos a abrir o código confidencial de suas plataformas militares.
3. **Convergência com o Ecossistema Portico:** A simetria com a licença Apache 2.0 do Portico consolida o Valhalla como a sua evolução natural e legítima, facilitando a transição de toda a base instalada global existente.


