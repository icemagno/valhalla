# Architectural and Strategic Deliberations: Project Valhalla (HLA/RTI Next-Gen)

> **Project:** Project Valhalla  
> **Lead Developer & Creator:** Carlos Magno Oliveira de Abreu (<magno.mabreu@gmail.com>)  
> **Visual Identity:** Nordic shield with 4 cardinal arrows (North, South, East, West) in green and orange.  
> **Record Date:** November 24, 2026  
> **Purpose:** Exhaustive record of technical deliberations, international standards analysis, licensing models, architectural decisions, and cryptographic integration for a sovereign next-generation simulation infrastructure based on HLA (High Level Architecture) and RTI (Run-Time Infrastructure).

---

## Executive Summary

This document consolidates all strategic decisions regarding official HLA and RTI standards, the engineering behind **Pitch pRTI**, critical gaps in the international modeling & simulation community, the novel integration of blockchain-grade data structures and serialization (**SSZ** and Sparse Merkle Trees), feasibility analysis of **RPR-FOM**, the format evolution (**XML vs. JSON / HLA 4**), and our master strategy of native wire-level multiplexing and backward compatibility with the open-source **Portico** ecosystem (founded by Tim Pokorny / Calytrix Technologies).

---

## 1. Official Specification: HLA and RTI

The **High Level Architecture (HLA)** is an international distributed simulation standard developed to ensure interoperability and reuse of simulators and software components across diverse domains. It is maintained by the **SISO** (*Simulation Interoperability Standards Organization*) and standardized by the **IEEE**.

### 1.1. Historical Evolution of the Standards

1. **HLA 1.3 (DMSO - 1998):**
   * Legacy baseline sponsored by the Defense Modeling and Simulation Office (US DoD).
   * Relied on custom plain-text model files (`.fed`) and archaic C++ APIs with manual memory management. Still present in legacy operational military systems.
2. **IEEE 1516-2000:**
   * First formal standardization under the IEEE umbrella.
   * Introduced structured data modeling in XML.
3. **IEEE 1516-2010 (HLA Evolved):**
   * Most widely adopted standard in defense and industry to date.
   * **Crucial innovations:** Modular FOMs (composing data models across multiple XML files), C++ smart pointer support, encoding helper classes, formal WSDL/Web Services transport specifications, and fault-tolerant communication mechanisms.
4. **IEEE 1516-2025 (HLA 4):**
   * The latest revision of the standard.
   * **Core focus:** Cloud-native architecture, Modelling and Simulation as a Service (MSaaS), lightweight web federates connecting via WebSockets/REST, and data model definitions via JSON Schema.

### 1.2. The Three Pillars of IEEE 1516

* **IEEE 1516 (Framework and Rules):** 10 fundamental architectural rules (5 mandatory rules for Federations and 5 mandatory rules for Federates).
* **IEEE 1516.1 (Federate Interface Specification):** Formal API contract between simulator code and the RTI middleware (`RTIambassador` for outbound requests; `FederateAmbassador` for asynchronous inbound callbacks).
* **IEEE 1516.2 (Object Model Template - OMT):** Syntax and tabular/schematic structure for data models:
  * **FOM (Federation Object Model):** Shared contract governing the entire federation.
  * **SOM (Simulation Object Model):** Subset of data published or subscribed by an individual federate.
  * **MIM (Management Object Model):** Internal RTI infrastructure objects and interactions for introspection and control.

### 1.3. The Seven RTI Service Groups (IEEE 1516.1)

1. **Federation Management:** Federation lifecycle (creation, destruction, join, resign, global synchronization points, and save/restore checkpoints).
2. **Declaration Management:** Publication and subscription (Pub/Sub) filtering for object classes, attributes, and interaction classes.
3. **Object Management:** Dynamic instance registration, discovery, attribute updates (`UpdateAttributeValues`), reflections (`ReflectAttributeValues`), and transient event interactions (`SendInteraction` / `ReceiveInteraction`).
4. **Ownership Management:** Negotiated transfer of attribute publication ownership among federates (e.g., aircraft launching a missile whose trajectory computation transfers to a specialized ballistics simulator).
5. **Time Management:** Deterministic logical time progression with Time-Stamp Order (TSO) guarantees, coordinating Time-Regulating and Time-Constrained federates via Lower Bound on Time Stamp (LBTS) and Lookahead calculations.
6. **Data Distribution Management (DDM):** Multi-dimensional spatial filtering (routing spaces and regions of interest). Reduces network load by ensuring federates only receive data generated in designated spatial volumes.
7. **Support Services:** Helper routines for handles, name-to-handle resolution, and transport configurations.

---

## 2. Pitch Technologies Engineering (Pitch pRTI and Tooling)

Swedish firm **Pitch Technologies** (now part of BAE Systems) established itself as the global industry benchmark for strict compliance and execution performance.

### 2.1. Decentralized Architecture

Pitch pRTI avoids centralized bottlenecks through a split architecture:
* **Pitch RTI Executive (Central RTI Component - CRC):** Lightweight central coordinator handling federation lifecycle, FOM schema validation, handle allocation, and synchronization points.
* **Local RTI Component (LRC):** High-performance embedded client library linked inside each federate process.
  * Heavy attribute and interaction traffic **bypasses the central executive**.
  * Real-time data exchanges route directly between LRCs via optimized IP Multicast and peer-to-peer (P2P UDP/TCP) connections, achieving sub-millisecond latencies and high throughput.

### 2.2. Multi-Standard Flexibility

* **Unified Executive:** A single Pitch pRTI instance simultaneously connects federates built for HLA 1.3, IEEE 1516-2000, and IEEE 1516-2010.
* **Language Bindings:** Precompiled native libraries for C++ (32/64-bit Windows/Linux), Java, .NET/C#, and Web bindings.
* **Modular FOM Validation:** Dynamic XML schema parsing and class inheritance trees resolved at `createFederationExecution`.

### 2.3. Commercial Product Ecosystem

Pitch provides an end-to-end commercial suite:
* **Pitch Booster:** WAN acceleration appliance with adaptive compression, UDP packet bundling, and Forward Error Correction (FEC).
* **Pitch Commander:** Visual GUI for federation monitoring, topology visualization, throughput tracking, and node synchronization.
* **Pitch Visual OMT:** IDE for authoring, editing, and validating IEEE 1516.2 FOM/SOM files.
* **Pitch Gateway:** Protocol bridge connecting HLA federations to DIS (Distributed Interactive Simulation / IEEE 1278) or bridging separate HLA federations.
* **Pitch Talk:** Simulated tactical radio and audio communications (military radio, intercoms, tactical data links).

---

## 3. Critical Community Deficiencies and Unmet Demands

Despite widespread adoption, the simulation community faces enduring challenges stemming from legacy architectural decisions:

| Area / Need | Current HLA/RTI Limitation | Practical Impact | Community Requirement |
| :--- | :--- | :--- | :--- |
| **Standard Wire Protocol** | IEEE 1516 standardizes API bindings, not network wire packets. | **Vendor Lock-in:** Nodes linked with Pitch cannot exchange network packets with MAK or Portico nodes without heavy gateways. | An open, standardized wire protocol (analogous to RTPS in DDS). |
| **Cloud & Containers (Cloud-Native / K8s)** | Classical RTI assumes LANs and raw IP Multicast. Cloud providers (AWS, GCP, Azure) block multicast by default. | Running simulations in Docker containers or Kubernetes meshes with dynamic IPs is fraught with workarounds. | Cloud-native node discovery and modern transport (gRPC, QUIC, WebSockets). |
| **Game Engine Integration (Unreal 5 / Unity)** | Commercial C++ RTI libraries use concurrency models that conflict with game engine frame loops (Tick/Render). | Teams spend months writing brittle wrappers to avoid thread contention, crashes, and DLL conflicts. | Idiomatic, pre-compiled plugins for Unreal Engine and Unity with non-blocking ring buffers. |
| **Serialization Performance & Usability** | XML OMT is heavy; IEEE 1516 Encoding Helpers require dozens of lines of manual code for basic structures. | Slow serialization, memory allocations on the hot path, and poor developer ergonomics. | Zero-copy, deterministic schema compilers (SSZ, FlatBuffers). |
| **Fine-Grained Security / Zero Trust (MLS)** | Joining a federation gives visibility to all published traffic; DDM is a routing optimization, **not a cryptographic boundary**. | In coalition exercises (e.g., NATO), classified sensor/EW parameters cannot be hidden from allies on the same bus without expensive physical Cross-Domain Solutions. | Attribute-level encryption and Attribute-Based Access Control (ABAC), sharing global position while protecting payload parameters. |
| **Time Management Fault Tolerance** | In deterministic TSO time advancement, if a single federate stutters or hangs, LBTS stalls the **entire federation**. | Large simulations are vulnerable to single-node failures; dynamically admitting late-joining federates is problematic. | Heartbeat monitoring, dynamic eviction of hung nodes from LBTS, and smooth state catch-up. |
| **Modern Domain Models** | RPR-FOM v2 reflects 1990s DIS conventions (tanks, ships, aircraft). | Modeling Electronic Warfare, Cyber, Space assets, and massive autonomous AI drone swarms is clunky and constrained. | High-density multi-domain operations (MDO) models optimized for autonomous AI swarms. |

---

## 4. Innovation: Blockchain-Grade Serialization (SSZ) and Sparse Merkle Trees

Integrating deterministic serialization and cryptographic primitives engineered for Web3 consensus layers (**SSZ - Simple Serialize**, per Ethereum 2.0 specifications) offers a breakthrough approach to HLA performance and security:

### 4.1. The Simple Serialize (SSZ) Engine

SSZ was designed as a lean, zero-copy alternative to bloated serialization formats:
* **Strict Determinism and Canonical Form:** Every structured data object maps to an invariant, unambiguous binary representation.
* **Native Merkleization (`hash_tree_root`):** Any SSZ structure can be recursively hashed into a binary Merkle tree, yielding an immutable 32-byte root representing total state.
* **Zero-Copy In-Memory Layout:** Fixed-size fields are laid out contiguously at buffer start, followed by dynamic field offsets. Attributes can be read directly in place without intermediate allocations.

### 4.2. Practical RTI Applications

1. **Instant Desynchronization Detection:**
   * In lockstep or time-managed simulations, federates compute autonomous physical entities locally.
   * Instead of broadcasting massive coordinate dumps, federates compare the 32-byte Merkle root (`hash_tree_root`). If a single bit diverges between pilot and control tower, the roots differ instantly, pinpointing the exact out-of-sync leaf.
2. **Merkle Proofs for Multi-Level Security (MLS):**
   * Using Merkle inclusion proofs (or multiproofs), a federate publishes its state root and selectively discloses only public attributes (e.g., `KinematicPosition`) with an attached proof.
   * A recipient mathematically validates that the position belongs to the authentic state without ever seeing classified branches (e.g., radar frequencies or weapon inventory).
3. **Immutable Audit Trails for After Action Review (AAR):**
   * Tactical engagements, weapon fires, and detonations are cryptographically hashed and digitally signed (e.g., Ed25519).
   * Post-mission reconstruction cannot be altered or disputed by participating units.
4. **Decentralized Consensus for High Availability:**
   * Eliminating single points of failure by replacing the central CRC with high-speed consensus (Raft/BFT) to arbitrate federation lifecycle and attribute ownership.
5. **Zero-Knowledge Proofs (ZKP) for Electronic Warfare:**
   * Missile guidance federates prove target lock-on using verifiable computation over target radar signatures **without revealing proprietary Radar Cross Section (RCS) data or classified sensor sensitivity curves**.

### 4.3. Fundamental Constraints (What NOT to Import from Blockchain)

* **No Proof of Work (PoW):** Excessive latency and compute overhead make PoW unviable.
* **No Block Latency:** Distributed simulation requires **30 Hz to 100 Hz** tick rates with $< 10\ \text{ms}$ latency. Hashing and Merkleization run asynchronously or on configured checkpoints, while real-time coordinate streaming flows uninterrupted.

---

## 5. The Unified Protocol Analysis: Mitigating Ecosystem Fragmentation

Evaluating the proposal of creating a "novel unified HLA network protocol" yields critical insights:

1. **The 15th Standard Trap (XKCD 927):** Inventing an unstandardized protocol from scratch risks creating yet another silo disconnected from established simulators.
2. **Proprietary Protocol Hazards:**
   * Pitch and MAK wire protocols are closed trade secrets.
   * Reverse-engineering them risks legal liabilities and creates fragile implementations vulnerable to upstream vendor updates.
3. **The Pragmatic Market Path:**
   * **API Standardization (IEEE 1516.1):** Maintain simulator portability via standard C++ and Java API headers.
   * **Native Open Multiplexing:** Provide wire-level compatibility with the premier open-source implementation (**Portico**), combined with modern high-speed extensions.

---

## 6. Pitch Licensing Realities and Commercial Architecture

### 6.1. Pitch Licensing Constraints
* Pitch pRTI and Pitch Gateway are proprietary, closed commercial products requiring USB dongles or floating FlexLM license servers.
* Embedding Pitch runtime DLLs (`.dll` / `.so`) into third-party commercial software without costly OEM agreements is strictly prohibited.

### 6.2. Strategic Engineering Alternatives
* **Bring Your Own License (BYOL):** Build federate software against standard IEEE 1516.1 C++ headers and load RTI libraries dynamically (`LoadLibrary` / `dlopen`). The end user provides their own Pitch license.
* **Self-Contained Gateway Federates:** Bridge disparate federations using standard API federates, consuming ordinary client seats rather than proprietary gateway licenses.
* **Open-Source Sovereign RTI:** Build and deploy a high-performance open-source RTI licensed under Apache 2.0 to eliminate per-seat licensing barriers.

---

## 7. The RPR-FOM (Real-time Platform Reference FOM)

* **Designation:** **Real-time Platform Reference FOM** (SISO-STD-001).
* **Open Standard:** RPR-FOM is 100% open and royalty-free, maintained by SISO.
* **Industry Standard:** RPR-FOM serves as the universal lingua franca for defense simulation across NATO, US DoD, and international partners, ensuring interoperability with MAK VR-Forces, Bohemia Interactive VBS, Saab Gripen trainers, and Thales platforms.

### 7.1. Modular Structure of RPR-FOM v2.0 (SISO-STD-001-2015)
* `RPR-Base.xml`: Fundamental coordinate conventions (Geocentric ECEF / WGS-84), basic vectors, and identifiers.
* `RPR-Physical.xml`: Physical entity class hierarchies (Aircraft, Surface Vessels, Subsurface, Land Vehicles, Munitions, Sensors).
* `RPR-Warfare.xml`: Combat engagements (`WeaponFire`, `MunitionDetonation`).
* `RPR-Radio.xml`: Tactical communications, RF frequencies, transmitter states, and Link 16 datalinks.
* `RPR-SimManagement.xml`: Execution control (start, pause, reset).
* `RPR-Logistics.xml`: Resupply, refueling, maintenance, and battle damage repair.

### 7.2. Extensibility Engineering Rules
**Standard RPR-FOM XML files must never be edited directly.**  
Under IEEE 1516-2010, extensions must be defined in modular extension schemas (e.g., `CustomExtension.xml`) that import SISO standard modules and subclass existing entities, maintaining backward compatibility with legacy observers.

---

## 8. Data Model Formats: XML vs. JSON and HLA 4

### 8.1. Current Prevalence of XML
* Over 99% of deployed industrial FOMs rely on XML validated against `HLA.xsd` (IEEE 1516-2010).
* All existing SISO standards and visual tools (Pitch Visual OMT, MAK OMT) operate natively on XML.

### 8.2. JSON in HLA 4 (IEEE 1516-2025)
HLA 4 incorporates JSON in two distinct domains:
1. **Alternative FOM Specification (JSON Schema):** Standardizes OMT definitions using JSON Schema, accelerating web and cloud developers.
2. **Web Federate Wire Data:** The critical disruption of HLA 4. Instead of opaque binary buffers, web federates exchange JSON payloads over WebSockets and REST, enabling zero-install browser dashboards (React, Vue, Node.js) without native C++ compilation.

### 8.3. Implementation Strategy
* **Mandatory Baseline:** Full parsing and merging of modular XML FOMs (including RPR-FOM v2.0).
* **Next-Gen Advantage:** Native JSON Schema OMT ingestion and WebSockets/JSON real-time streaming interfaces.

### 8.4. Data Contract Lifecycle: FOM vs. SOM and the DSEEP Process (IEEE 1730)

A fundamental architectural question in distributed simulation is: **How do two independent simulators (e.g., a radar from Vendor A and a vessel from Vendor B) interoperate if neither vendor possessed the other's source code at compile time?**

1. **Pre-Simulation Contract Harmonization (DSEEP):**
   * Under the **Distributed Simulation Engineering and Execution Process (IEEE 1730 / DSEEP)**, developers never code against unknown or ad-hoc runtime schemas.
   * **Simulation Object Model (SOM):** Each vendor documents the precise subset of data their simulator can publish or subscribe (e.g., `Radar-SOM.xml` publishes radar tracks and subscribes to mobile physical entities; `Ship-SOM.xml` publishes vessel kinematics).
   * **Federation Object Model (FOM):** Prior to simulation execution, systems architects harmonize individual SOMs into a unified federation data contract. Both vendors compile their software against this shared schema (or standard industry schemas such as **RPR-FOM v2.0**).
2. **Polymorphic Interoperability through Inheritance:**
   * The radar does not need to know the proprietary attributes of an experimental vessel.
   * Because RPR-FOM establishes canonical class hierarchies (`HLAobjectRoot` $\to$ `BaseEntity` $\to$ `PhysicalEntity` $\to$ `Platform` $\to$ `SurfacePlatform`), the radar subscribes to `BaseEntity` (receiving standard coordinates and velocities).
   * When the vessel registers as an instance of `SurfacePlatform`, the RTI's **InterestMatcher** automatically routes updates to the radar via polymorphic subsumption.

### 8.5. Runtime Handle Resolution: Portico "Eager Push" vs. Pitch "Lazy Pull"

The IEEE 1516 specification standardizes the API call `getObjectClassHandle("Class.Name")`, but **deliberately omits the underlying network wire protocol for handle resolution**. This led to divergent architectural implementations:

| Dimension | Portico Strategy ("Eager Push") | Pitch Strategy ("Lazy Pull + Cache") | Valhalla Sovereign Strategy |
| :--- | :--- | :--- | :--- |
| **Join Handshake** | **Heavyweight (33 KB to MBs):** Serializes the entire in-memory Java `ObjectModel` into the `WelcomePack` byte payload. | **Ultra-lightweight (< 100 bytes):** Handshake exchanges only identity, handle tokens, and authentication tickets. | **Dual-Dialect Adaptive:** Synthesizes `WelcomePack` for Portico nodes; uses 56-byte linear headers for modern VAL4 nodes. |
| **Name-to-Handle Resolution** | Evaluated purely in client-side memory via Java HashMap lookups with 0 additional network calls. | Resolved against local XML cache in 0 ns; if missing, issues a lightweight binary query to CRC and caches the result. | Local cache in 0 ns; CRC server evaluates class inheritance in $< 1.2\ \text{ns}$ via **Euler Tour Interval Encoding** ($O(1)$). |
| **Coupling & Fragility** | Fragile: tightly coupled to Java `ObjectInputStream` and JVM class `serialVersionUID`s. | Language-agnostic: clean binary wire protocol for C++, Java, C#, and web clients. | Zero JVM dependency: pure Go/C-ABI wire protocol with backward-compatible JOS translation. |
| **Scalability** | Struggles on constrained WAN links during simultaneous federate joins due to massive XML/Object serialization. | Scales seamlessly to thousands of federates across distributed wide-area networks. | Maximum scalability: zero-copy SSZ layouts, zero-allocation buffers, and instant bitset interest matching. |

---

## 9. Global HLA 4 Adoption Status

HLA 4 (IEEE 1516-2025) is currently in the **Early Adoption** phase:
* **Pitch Technologies:** Pioneering reference implementations and demos (led by Pitch engineers who chaired SISO drafting committees).
* **MAK Technologies:** WebLVC and MAK RTI updates supporting cloud/web profiles.
* **Fraunhofer IOSB & NATO:** Research groups (NATO MSG-164 / MSG-197) validating MSaaS concepts in cloud testbeds.
* **Open Source:** Portico and CERTI maintain experimental branches, but community production releases remain pending.

---

## 10. The Portico Ecosystem and Calytrix Technologies

* **Origin:** Portico was architected by **Tim Pokorny** (Co-founder & CTO of **Calytrix Technologies**, Australia) and his engineering team.
* **Business Model:** Calytrix released Portico under an open-source license to establish an accessible baseline, monetizing integration services, specialized gateways (LVC Game for Unreal/Unity), and tactical audio suites (CNR-Sim).

### 10.1. Global Adoption of Portico
1. **Australian Defence Force (ADF & DSTG):** Extensively used in joint synthetic training environments.
2. **NASA:** Deployed across space flight simulation centers for collaborative distributed robotics and orbital studies, eliminating seat licensing costs.
3. **NATO Science and Technology (NATO STO / MSG):** Standard neutral baseline for multinational synthetic testbeds.
4. **Major Defense Integrators:** Deployed globally in local developer sandboxes and CI/CD automated test pipelines (GitLab/GitHub) to run integration tests without commercial license contention.

---

## 11. Core Strategy: Sovereign Innovation + Native Portico Wire Compatibility

Our technical strategy delivers a **sovereign, high-performance Go-based RTI while maintaining native wire-level compatibility with the Portico ecosystem**.

### 11.1. Legal Viabilidade
* Portico is distributed under the **Apache License 2.0**.
* Apache 2.0 permits studying source code, implementing interoperable network protocols, extending features, and distributing derivative works without royalty obligations or copyleft restrictions.

### 11.2. Three-Layer Architecture

```
+---------------------------------------------------------------------------------+
| LAYER 3: Modern Data Models & Deterministic Serialization                      |
| - SSZ (Simple Serialize) zero-copy layouts & FlatBuffers                        |
| - State integrity verification via Merkle Roots (hash_tree_root)                |
| - Full compliance with RPR-FOM v2.0 (official XML modules and extensions)       |
+---------------------------------------------------------------------------------+
| LAYER 2: Advanced HLA Services & Cloud Connectivity                             |
| - IEEE 1516-2025 (HLA 4) WebSockets, JSON, and Web Profiles                     |
| - Native non-blocking ring-buffer dispatch for Unreal Engine 5 & Unity          |
| - Handshake Feature Negotiation (SSZ, Merkle, Wire Protocol Dialect)             |
+---------------------------------------------------------------------------------+
| LAYER 1: Core High-Performance Network Transport                                |
| - Native Portico Wire Compatibility (PorticoMessage / JGroups / TCP / UDP)      |
| - Sovereign native Go engine with P2P UDP, TCP, and zero-allocation framing     |
+---------------------------------------------------------------------------------+
```

### 11.3. Integration Mechanisms
1. **Payload Transparency:**
   * Portico treats attribute and interaction payloads as opaque byte buffers (`byte[]` / `VariableLengthData`).
   * Modern nodes can embed deterministic SSZ buffers inside standard Portico messages without causing decoding errors on legacy nodes.
2. **Feature Negotiation during Handshake:**
   * On joining a federation, nodes exchange capability flags.
   * If the peer is an original Java Portico node, the connection adheres to the classic Portico dialect.
   * If the peer is a modern Valhalla node, the channel activates high-speed UDP framing, Merkle root verification, and SSZ streaming.
3. **Hybrid Edge Bridge (LAN Portico <-> Cloud/Web):**
   * The Valhalla node participates as a legitimate Portico federate on the physical LAN while concurrently serving WebSockets/JSON to browsers and cloud nodes.

---

## 12. Post-Quantum Cryptography (PQC) and KEM

### 12.1. Key Encapsulation Mechanisms (KEM)
KEM is an asymmetric cryptographic primitive designed to securely establish shared symmetric keys over unencrypted channels:
1. **Handshake Phase:** KEM runs once during initial connection to derive a high-entropy 256-bit symmetric key.
2. **Streaming Phase:** High-frequency updates (60 Hz to 100 Hz) are encrypted using hardware-accelerated Authenticated Encryption with Associated Data (AEAD) algorithms (AES-256-GCM or ChaCha20-Poly1305).

### 12.2. Defense Standards Alignment
* While IEEE 1516 leaves transport security to underlying network layers, modernization mandates (such as NSA **CNSA 2.0** and NATO interoperability roadmaps) require transitions to Post-Quantum Cryptography to protect against *Harvest Now, Decrypt Later* threats.
* The NIST-standardized PQC algorithm is **ML-KEM (Kyber / FIPS 203)**. Incorporating ML-KEM provides an unprecedented technical differentiator in defense simulation.

### 12.3. Performance Analysis
* **Handshake Cost:** ML-KEM-768 key encapsulation takes $\approx 15\text{--}40\ \mu\text{s}$ of CPU time in Go, exchanging $\approx 1\ \text{KB}$ once per session.
* **Streaming Throughput:** Hardware-accelerated AES-256-GCM (AES-NI / ARM Crypto) processes $> 3\text{--}5\ \text{GB/s}$ per core, adding $< 2\ \mu\text{s}$ of latency per packet.

---

## 13. Dual-Stream Segregated Dispatch

### 13.1. The Lowest Common Denominator Trap
In a mixed deployment of 100 federates (99 modern Valhalla nodes and 1 legacy Portico node), forcing the federation to downgrade to legacy formats would discard the performance, security, and integrity gains of SSZ and Merkle trees.

### 13.2. Segregated Dispatch Solution
Project Valhalla implements selective dual-path delivery at the publishing boundary:
1. **Modern Nodes Run at Peak Performance:** The publisher encodes the update in SSZ with Merkle root metadata and transmits it over high-speed channels to the 99 modern peers.
2. **Targeted Legacy Delivery:** For the single legacy Portico subscriber, the publisher generates a standard IEEE 1516 / RPR-FOM byte buffer (costing $\approx 200\text{--}300\ \text{ns}$ in Go) and delivers it over Portico's unicast channel.
3. **Subscription Scoping:** The dual path only triggers for attributes explicitly subscribed by the legacy node.

---

## 14. Project Licensing Decision: Apache License 2.0

Project Valhalla is released under the **Apache License, Version 2.0**:
1. **Government and Enterprise Adoption:** Apache 2.0 is the gold standard for open infrastructure (Kubernetes, Linux Foundation, Apache Foundation), providing legal clarity for military and defense contractors.
2. **Patent Grants and Non-Viral Integration:** Unlike copyleft licenses (GPL), Apache 2.0 contains an express patent grant and allows aerospace and defense integrators to embed Valhalla without risking proprietary simulator or weapons system IP.
3. **Ecosystem Symmetry:** Alignment with Portico's Apache 2.0 license establishes Project Valhalla as its natural, compliant successor.
