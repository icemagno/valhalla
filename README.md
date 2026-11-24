# Projeto Valhalla

<p align="center">
  <img src="assets/logo.svg" alt="Logotipo Projeto Valhalla" width="220" height="220" />
</p>

<p align="center">
  <strong>A Próxima Geração de HLA/RTI: Evolução do PorticoRTI para a Norma HLA-4 (IEEE 1516-2025)</strong><br>
  <em>Simulação Distribuída de Alto Desempenho com Serialização SSZ, Merkleização, Criptografia Pós-Quântica (ML-KEM) e Compatibilidade Nativa com a Rede Portico.</em>
</p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-Apache_2.0-orange.svg" alt="License: Apache 2.0"></a>
  <img src="https://img.shields.io/badge/Language-Go_1.25+-00ADD8.svg" alt="Language: Go">
  <img src="https://img.shields.io/badge/HLA_Standard-IEEE_1516--2025_(HLA--4)-green.svg" alt="HLA Standard">
  <img src="https://img.shields.io/badge/Compatibility-PorticoRTI_Native-blue.svg" alt="Compatibility: Portico">
</p>

---

## 🛡️ Sobre o Projeto Valhalla

O **Projeto Valhalla** é uma infraestrutura de tempo de execução (**Run-Time Infrastructure - RTI**) moderna, resiliente e de altíssimo desempenho, desenvolvida em **Go (Golang)** com módulos de aceleração em **Rust**. 

O projeto nasce como a evolução natural do ecossistema de código aberto do **PorticoRTI**, elevando-o aos requisitos modernos da especificação **IEEE 1516-2025 (HLA-4)** e atendendo às dores mais críticas da comunidade militar e aeroespacial de simulação.

### O Símbolo
O logotipo do Projeto Valhalla é um **escudo nórdico** clássico nas cores **verde e laranja**, ostentando no centro **quatro setas apontando para as direções cardeais (Norte, Sul, Leste e Oeste)**. O escudo representa a resiliência inquebrável, segurança criptográfica e robustez da federação; as quatro setas simbolizam o barramento de distribuição e interoperabilidade multidirecional entre sistemas virtuais, construtivos e reais (*LVC - Live, Virtual, and Constructive*).

---

## ⚡ Pilares Arquiteturais

1. **Convivência Nativa com PorticoRTI (Zero Gateways Externos):**
   * O Valhalla implementa o protocolo binário de rede original do Portico, conectando-se diretamente a instâncias legadas (incluindo ambientes containerizados como o **Projeto Arcanjo**).
   * **Despacho Duplo Segregado (*Dual-Stream Dispatch*):** Nós modernos comunicam-se entre si via SSZ e QUIC/UDP de alta velocidade; nós legados recebem cirurgicamente seus pacotes traduzidos no padrão clássico IEEE 1516.
2. **Serialização SSZ (Simple Serialize) e Árvores de Merkle:**
   * Troca de dados ultra-rápida (*zero-copy*) com cálculo canônico de `hash_tree_root`.
   * **Detecção Imediata de Dessincronização (*Desync*):** Verificação de integridade entre federados através de hashes de 32 bytes.
   * **Segurança Multinível (MLS / Zero-Trust):** Provas de inclusão de Merkle (*Merkle Proofs*) permitindo revelação seletiva de atributos confidenciais.
3. **Criptografia Pós-Quântica (PQC) com KEM:**
   * Adoção de **ML-KEM-768 (Kyber / FIPS 203)** via HPKE (RFC 9180) no aperto de mão, seguido de cifragem simétrica de alta frequência com **AES-256-GCM** acelerado por hardware (**AES-NI**). Latência adicional desprezível (< 2 µs por pacote).
4. **Prontidão para Nuvem e HLA-4 Web Profile:**
   * Sem dependência de IP Multicast em nuvem (descoberta nativa via Kubernetes DNS).
   * Interfaces **WebSockets e JSON** nativas para painéis de controle, dashboards em navegadores e clientes leves.
5. **Base de Testes Rigorosa:**
   * Testado e validado de ponta a ponta utilizando os esquemas oficiais do **RPR-FOM v2.0** (SISO-STD-001-2015).

---

## 📚 Documentação do Projeto

* 📋 [Deliberações Arquiteturais e Conceituais](file:///c:/Magno/Projetos/hla-rti/deliberacoes.md): Registro histórico completo de todas as decisões, análises de mercado (Pitch, MAK, Portico), viabilidade jurídica e princípios de engenharia.
* 🗺️ [Plano de Projeto e Sprints TDD](file:///c:/Magno/Projetos/hla-rti/plano_de_projeto.md): Roadmap detalhado das Sprints 0 a 9 com metodologia Test-Driven Development (TDD) e critérios de aceitação.

---

## 🛠️ Stack Tecnológica

* **Core & Network:** Go 1.25+
* **Aceleração Crítica / FFI:** Rust (quando necessário para SIMD e alocação zero)
* **Ambiente de Testes Oraculares:** Docker / Projeto Arcanjo (Portico)
* **Modelo de Dados Base:** RPR-FOM v2.0 (SISO)

---

## 👤 Autoria e Liderança do Projeto

* **Criador e Desenvolvedor Principal:** Carlos Magno Oliveira de Abreu ([@icemagno](https://github.com/icemagno))
* **E-mail:** [magno.mabreu@gmail.com](mailto:magno.mabreu@gmail.com)

---

## 📄 Licença

O Projeto Valhalla é distribuído sob a licença de código aberto **Apache License, Version 2.0**. Consulte o arquivo [LICENSE](file:///c:/Magno/Projetos/hla-rti/LICENSE) para obter o texto integral dos termos e condições.

