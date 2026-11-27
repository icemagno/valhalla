# Project Valhalla

<p align="center">
  <img src="assets/logo.svg" alt="Project Valhalla Logo" width="220" height="220" />
</p>

<p align="center">
  <strong>The Next Generation of HLA/RTI: Portico-Compatible HLA-4 (IEEE 1516-2025)</strong><br>
  <em>Ultra-High-Performance Distributed Simulation with Dual-Dialect Multiplexing, Euler Tour Hierarchy Indexing, Vector Bitsets, SSZ Merkle State Verification, and 3D Morton DDM.</em>
</p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-Apache_2.0-orange.svg" alt="License: Apache 2.0"></a>
  <img src="https://img.shields.io/badge/Language-Go_1.25+-00ADD8.svg" alt="Language: Go">
  <img src="https://img.shields.io/badge/HLA_Standard-IEEE_1516--2025_(HLA--4)-green.svg" alt="HLA Standard">
  <img src="https://img.shields.io/badge/Portico_3.0-Wire_Gateway_(Core_Services)-blue.svg" alt="Portico 3.0 Wire Gateway (Core Services)">
  <img src="https://img.shields.io/badge/NATO_IVCT-In_Preparation-lightgrey.svg" alt="NATO IVCT In Preparation">
  <img src="https://img.shields.io/badge/Benchmarks-89.67_µs_Latency-brightgreen.svg" alt="Latency: 89.67 µs">
</p>

---

## 🛡️ About Project Valhalla

**Project Valhalla** is a sovereign, modern, resilient, and ultra-high-performance **Run-Time Infrastructure (RTI)** engineered in pure **Go (Golang)** with optional zero-copy C-ABI/FFI extensions.

The project originated as the natural evolution of the open-source **Portico RTI** ecosystem, elevating it to meet the requirements of **IEEE 1516-2025 (HLA-4)** while resolving critical bottlenecks historically faced by aerospace and defense simulation communities.

### The Crest
The Project Valhalla emblem is a classic **Nordic shield** rendered in **emerald green and safety orange**, bearing four cardinal arrows (North, South, East, West) at its center. The shield represents unyielding resilience, cryptographic security, and federated state integrity; the four cardinal arrows signify omnidirectional high-speed distribution and interoperability across Live, Virtual, and Constructive (LVC) domains.

---

## ⚡ Architectural Pillars & Technical Differentiators

1. **Native Wire Multiplexing & Portico Compatibility (Zero Gateways):**
   * Implements dual-dialect session multiplexing on a single unified port (as well as dedicated 0 ms latency listeners): natively accepts both native high-performance binary frames (**VAL4**) and legacy Portico Java serialization streams (**JOS**).
   * **Dual-Stream Segregated Dispatch:** Modern federates stream via contiguous linear binary headers (VAL4) over high-throughput TCP and IEEE 1516-2025 WebSockets (Roadmap: High-Speed UDP Framing for best-effort coordinate streaming); legacy Portico nodes receive standard IEEE 1516 serialized byte buffers without requiring external proxy gateways.
   * **Per-Interface Dialect Segregation (Optional) [Roadmap]:** Expose CLI flags in `valhalla-crc` (e.g., `-portico-addr`, `-native-addr`, `-web-addr`) to bind dedicated protocol listeners to distinct NICs/IPs on multi-homed hosts—building on the existing `SetPorticoAddr` / `SetNativeAddr` server APIs—alongside optional per-interface/subnet ACLs for segmented or cross-domain network deployments.
2. **The Architectural Quadriad:**
   * **Euler Tour Tree Flattening:** Transforms recursive OMT class hierarchies into contiguous integer intervals $[L_c, R_c]$, enabling $O(1)$ subtree inheritance verification in $< 1.2\ \text{ns}$.
   * **64-bit Vectorized Bitsets:** Attribute interest matching evaluated via word-level bitwise operations (`AND`, `OR`, `Subsumes`) in $< 0.8\ \text{ns}$, bypassing hash table and pointer chases.
   * **SSZ Serialization & Sparse Merkle Trees:** Canonical binary layouts with native 32-byte `hash_tree_root` calculation for instant state desync detection and Multi-Level Security (MLS) attribute proofs.
   * **3D Morton Space-Filling Curves (Z-Order Hashing):** High-speed Data Distribution Management (DDM) mapping ECEF $(X, Y, Z)$ coordinates into 64-bit Morton codes for bitwise spatial containment checks in $< 2\ \text{ns}$. Standard subscriptions remain 100% opt-in with zero overhead, coexisting seamlessly with high-speed ballistic and orbital dead reckoning.
3. **Post-Quantum Cryptography (PQC) & Zero-Trust Security [Roadmap]:**
   * Proposed handshake key establishment powered by **ML-KEM-768 (Kyber / FIPS 203)** via HPKE (RFC 9180), followed by hardware-accelerated **AES-256-GCM** (AES-NI) for high-frequency coordinate streaming (Architecture Proposal / In Development for Future Release). Current runtime communication operates over high-throughput TCP VAL4 and WebSockets plaintext transport.
4. **Cloud-Native & IEEE 1516-2025 Web Profile:**
   * Eliminates hard dependencies on raw IP Multicast, running seamlessly across Docker, Kubernetes (DNS-based discovery), and cloud environments.
   * Native **WebSockets and JSON** interfaces for zero-install browser dashboards, telemetry visualizers, and web federates.
5. **Rigorous TDD Validation:**
   * Full end-to-end integration and schema ingestion validated against official **RPR-FOM v2.0** (SISO-STD-001-2015) schemas.

---

## 📊 Empirical Benchmarks

In Project Valhalla, performance is measured and published across two strictly separated layers to uphold scientific rigor and eliminate false equivalencies between in-memory CPU loops and network socket I/O:

### Layer 1: Real End-to-End Network Benchmarks (TCP Loopback Stack)
Measures the full round-trip under the standard methodology of SISO and Gütlein et al. (2020) with a 64-byte tactical payload ($LRC_{pub} \xrightarrow{TCP} CRC \xrightarrow{Router} TCP \xrightarrow{Callback} LRC_{sub}$):

| RTI Engine | Runtime & Language | Real E2E Network Round-Trip Latency | Throughput Profile | Notes |
| :--- | :--- | :--- | :--- | :--- |
| **Pitch pRTI 5.4** | C++ / Java (Commercial) | **300 µs – 800 µs** | High | Gütlein et al. (2020) WSC baseline |
| **MÄK RTI 4.5** | Native C++ (Commercial) | **500 µs – 1,200 µs** | High | Gütlein et al. (2020) WSC baseline |
| **CERTI 3.5.1** | C++ (Open Source) | **1,500 µs – 5,000 µs** | Moderate | Gütlein et al. (2020) WSC baseline |
| **Portico RTI 2.1** | Java / JGroups (Open Source)| **1,000 µs – 60,000 µs** | Variable | GC pauses under heavy load |
| **Project Valhalla** | Pure Go Native (VAL4) | **~81.5 µs** (Mean) / **~71.9 µs** (P50) | ~14,600+ sync roundtrips/s | Windows QPC hardware timer / 0 data races |

### Layer 2: Valhalla In-Memory Algorithmic Microbenchmarks
Measures pure CPU cost of core data structures in isolation (`testing.B` on x86_64):

| Algorithmic Subsystem | Go Benchmark | Latency / op | Heap Allocations | Computational Complexity |
| :--- | :--- | :--- | :--- | :--- |
| **Binary Header Marshaling** | `BenchmarkHeader_MarshalTo` | **3.22 ns** | **0 B/op (0 allocs)** | $O(1)$ linear contiguous |
| **OMT Class Hierarchy** | `BenchmarkOMTRegistry_IsSubclassOfObject` | **15.40 ns** | **0 B/op (0 allocs)** | $O(1)$ Euler Tour intervals |
| **OMT Attribute Resolution** | `BenchmarkOMTRegistry_ResolveAttribute` | **23.20 ns** | **0 B/op (0 allocs)** | $O(1)$ dynamic index |
| **Vector Bitset Subsumes** | `BenchmarkBitset_Union` | **33.70 ns** | **0 B/op (0 allocs)** | $O(W)$ 64-bit word ops |
| **Interest Matching** | `BenchmarkInterestMatcher_GetAttributeSubscribers` | **275.70 ns** | **0 B/op (0 allocs)** | Bitset intersection |
| **Merkle State Root (Cached)** | `BenchmarkMerkleTree_GetGlobalRoot_Cached` | **30.10 ns** | **0 B/op (0 allocs)** | $O(1)$ atomic cache |
| **Spatial DDM (3D Morton)** | `BenchmarkMorton_ContainsPoint` | **< 4 ns** | **0 B/op (0 allocs)** | Bitwise dilated bounds |

---

## 📁 Project Directory Structure & Distribution Layout

Project Valhalla separates source packages, language SDKs, test suites, and distribution artifacts into a modular layout:

```
hla-rti/
├── cmd/                                # Application entry points
│   ├── valhalla-crc/                   # Central RTI Component (CRC server binary)
│   └── valhalla-federate/              # Native Go HLA-4 federate (LRC client binary)
├── dist/                               # Ready-to-go compiled binaries and distribution artifacts
│   ├── bin/                            # Native binaries (valhalla-crc, valhalla_lrc.dll, libvalhalla_lrc.so)
│   ├── include/                        # C/C++ distribution headers (rti1516e standard interfaces)
│   ├── java/                           # Java Client SDK distribution (valhalla-lrc.jar fat JAR with embedded DLL + SO)
│   ├── lib/                            # Import/static libraries (librti1516e.a)
│   └── web/                            # HLA-4 Web Profile JavaScript/TypeScript bundle
├── docs/                               # Architectural deliberations, whitepapers & specifications
├── examples/                           # Standalone reference federates across supported languages
│   ├── cpp-federate/                   # Standalone C++17 federate consuming Valhalla via CMake
│   ├── java-federate/                  # Standalone Maven Java federate consuming Valhalla via IEEE 1516e
│   └── web-federate/                   # Standalone HTML5/WebSocket tactical radar web federate
├── foms/                               # Standard HLA Object Model Template (FOM/MIM) XML schemas
├── pkg/                                # Core Go implementation packages
│   ├── cabi/                           # C-ABI / CGo export layer for native LRC dynamic library
│   ├── common/                         # Canonical handles, SSZ codecs, wire protocol catalog, Merkle state
│   ├── crc/                            # Central RTI Component executive server & session multiplexer
│   │   ├── core/                       # Simulation engines: OMT Euler Tour, Bitsets, 3D Morton DDM, Merkle trees
│   │   └── portico/                    # Portico wire protocol (JOS codec, message catalog, transport)
│   ├── logger/                         # High-throughput non-blocking asynchronous logging engine
│   ├── lrc/                            # Sovereign Go Local RTI Component (RTIambassador client)
│   └── metamodel/                      # IEEE 1516-2010 Modular XML FOM parser and schema compiler
├── portico-3.0-homolog/                # Official homologation test suite for Portico IEEE 1516e compatibility
├── scripts/                            # Build automation, packaging, and release scripts
├── sdk/                                # Language-specific client SDKs
│   ├── c/                              # Native C Client SDK (headers, CGo wrapper, CMake integration)
│   ├── cpp/                            # Modern C++17 Client SDK (RAII wrappers, CMake target)
│   ├── java/                           # Native Java Client SDK (canonical IEEE 1516e interfaces + JNA)
│   └── web/                            # HLA-4 Web Profile client SDK (WebSocket RFC 6455)
└── test/                               # Verification suites & test federates
    ├── benchmark/                      # End-to-end network latency & Layer 1 algorithmic benchmarks
    ├── c_test/                         # Native C verification test federate
    ├── integration/                    # Cross-dialect integration test scenarios (Native, Portico, Web)
    ├── java_test/                      # Java IEEE 1516e verification federate
    └── resources/                      # Official SISO RPR-FOM v2.0 (SISO-STD-001-2015) XML schemas
```

### What is Produced When Compiling LRC?
When you compile the Local RTI Component (LRC), the build produces:
1. **Native C-ABI Shared Libraries (`dist/bin/valhalla_lrc.dll` and `dist/bin/libvalhalla_lrc.so`):** Zero-dependency shared dynamic libraries exporting standard C functions for connection, federation execution, publication/subscription, object registration, attribute reflection, 3D Morton DDM routing, and attribute ownership handover.
2. **C Header Files (`sdk/c/include/valhalla_lrc.h`):** Complete C/C++ interface definition allowing direct integration into C, C++, Rust, Python, or legacy simulation engines.
3. **Java SDK Fat JAR (`dist/java/valhalla-lrc.jar`):** A standalone, fully self-contained Java archive implementing the canonical **IEEE 1516e (`hla.rti1516e.*`)** interfaces. It internally embeds **both** the Windows DLL and Linux SO native shared libraries alongside shaded JNA. End users simply drop `valhalla-lrc.jar` onto their Java classpath or declare it in Maven without installing external prerequisites or configuring system library paths.
4. **C++17 Static Library & Headers (`dist/lib/librti1516e.a` and `dist/include/rti1516e/`):** Standard IEEE 1516e C++ interface library and header files for native C++ simulations.
5. **HLA-4 Web Profile Client Bundle (`dist/web/valhalla-hla.js`):** Lightweight RFC 6455 WebSocket client library with TypeScript definitions (`dist/web/valhalla-hla.d.ts`) for browser and Node.js federates.

---

## 🔨 Building and Compiling (CRC & LRC)

### Prerequisites
* **Go:** 1.25+
* **C Compiler:** GCC (MinGW-W64 on Windows or standard GCC on Linux)
* **Java Development Kit:** JDK 17+ (JDK 22 supported)
* **Build Tool:** Apache Maven 3.9+

### 1. Compiling Central RTI Component (CRC)
```powershell
go build -o dist/bin/valhalla-crc.exe ./cmd/valhalla-crc
```

### 2. Compiling Native Go HLA-4 Federate
```powershell
go build -o dist/bin/valhalla-federate.exe ./cmd/valhalla-federate
```

### 3. Compiling Native C-ABI LRC Shared Library
```powershell
# Windows (DLL)
go build -buildmode=c-shared -o dist/bin/valhalla_lrc.dll ./pkg/cabi

# Linux (SO)
go build -buildmode=c-shared -o dist/bin/libvalhalla_lrc.so ./pkg/cabi
```

### 4. Compiling and Packaging the Java SDK (Automated Scripts)
To compile the C-ABI shared library, distribute it to `dist/bin/`, copy it to Java resources, and package `valhalla-lrc.jar` in a single command (without publishing to GitHub):
```cmd
# Windows (CMD / Batch)
scripts\build-sdk.bat

# Linux (Bash)
./scripts/build-sdk.sh
```

### 5. Publishing Java SDK to GitHub Maven Repository (`mvn-repo`)
When you are ready to publish a new version of the Java SDK to the official GitHub Maven repository (`mvn-repo` branch) for external `pom.xml` consumption:
```cmd
# Windows (CMD / Batch)
scripts\publish-maven.bat 1.0.0

# Linux / macOS (Bash)
./scripts/publish-maven.sh 1.0.0
```

---

## 🚀 Running & Quick Start Guide

### 1. Starting the Central RTI Component (CRC)
Launch the sovereign CRC execution server:
```powershell
# Windows
.\dist\bin\valhalla-crc.exe -port :52295

# Linux
./dist/bin/valhalla-crc -port :52295
```
*The CRC starts listening for dual-dialect multiplexed connections (native VAL4 and legacy Portico JOS) on a unified TCP port.*

### 2. Running the Native Go HLA-4 Federate
Launch the native Go simulation federate:
```powershell
# Windows
.\dist\bin\valhalla-federate.exe -address 127.0.0.1:52295 -federation DefenseSimulation -name Radar1

# Linux
./dist/bin/valhalla-federate -address 127.0.0.1:52295 -federation DefenseSimulation -name Radar1
```

---

### 3. Developing and Running a Java Federate (IEEE 1516e)

The Java Client SDK (`dist/java/valhalla-lrc.jar`) is 100% self-contained. It contains the standard IEEE 1516e interfaces, shaded JNA, and embedded native libraries.

#### Using as a Maven Dependency (External Projects)
Add the Valhalla GitHub Maven repository and dependency to your project's `pom.xml` (no authentication or GitHub token needed):

```xml
<repositories>
    <repository>
        <id>valhalla-mvn-repo</id>
        <url>https://raw.githubusercontent.com/icemagno/valhalla/mvn-repo/</url>
        <snapshots>
            <enabled>true</enabled>
            <updatePolicy>always</updatePolicy>
        </snapshots>
    </repository>
</repositories>

<dependencies>
    <dependency>
        <groupId>io.valhalla</groupId>
        <artifactId>valhalla-lrc</artifactId>
        <version>1.0.0</version>
    </dependency>
</dependencies>
```

#### Example Java Code (`SimpleFederate.java`)
```java
import hla.rti1516e.*;
import hla.rti1516e.exceptions.*;

public class SimpleFederate {
    public static void main(String[] args) throws RTIexception {
        // 1. Obtain official standard RTI factory & ambassador
        RtiFactory factory = RtiFactoryFactory.getRtiFactory();
        RTIambassador rtiAmb = factory.getRtiAmbassador();

        // 2. Connect to Valhalla CRC
        FederateAmbassador fedAmb = new NullFederateAmbassador();
        rtiAmb.connect(fedAmb, CallbackModel.HLA_EVOKED, "127.0.0.1:52295");
        System.out.println("Connected to Valhalla CRC!");

        // 3. Create and Join Federation Execution
        String federationName = "DefenseSimulation";
        try {
            rtiAmb.createFederationExecution(federationName);
        } catch (FederationExecutionAlreadyExists ignored) {}

        FederateHandle fedHandle = rtiAmb.joinFederationExecution("TacticalRadar", federationName);
        System.out.println("Joined federation with handle: " + fedHandle);

        // 4. Publish and Register Object Instance
        ObjectClassHandle radarClass = new ObjectClassHandle(1);
        AttributeHandle statusAttr = new AttributeHandle(1);
        AttributeHandleSet attrs = new AttributeHandleSet();
        attrs.add(statusAttr);

        rtiAmb.publishObjectClassAttributes(radarClass, attrs);
        ObjectInstanceHandle radarObj = rtiAmb.registerObjectInstance(radarClass, "AN/MPQ-64");
        System.out.println("Registered Radar Instance: " + radarObj);

        // 5. Clean teardown
        rtiAmb.resignFederationExecution(ResignAction.NO_ACTION);
        rtiAmb.disconnect();
        System.out.println("Disconnected cleanly.");
    }
}
```

#### Compiling and Running the Java Federate
```powershell
# Windows
javac -cp dist/java/valhalla-lrc.jar -d out SimpleFederate.java
java -cp "out;dist/java/valhalla-lrc.jar" SimpleFederate

# Linux
javac -cp dist/java/valhalla-lrc.jar -d out SimpleFederate.java
java -cp "out:dist/java/valhalla-lrc.jar" SimpleFederate
```

#### Running the Built-in Java End-to-End Test Suite (DDM + Ownership)
```powershell
powershell -File test/java_test/run_test.ps1
```

### 4. Standalone Example Federate (Official 11-Stage Homologation)

A complete, standalone reference federate that runs against the sovereign CRC and verifies all 11 homologation stages (connection, creation, join, declaration, 3D Morton DDM, registration, spatial updates, attribute reflection, attribute ownership divestiture and acquisition, object deletion, resignation, destruction, disconnection) is located in `examples/java-federate`:

```cmd
# Windows (CMD / Batch)
cd examples\java-federate
run.bat

# Linux / macOS (Bash)
cd examples/java-federate
./run.sh
```

---

### 4. Developing and Running a Native C / C++ Federate

Using the native C SDK ([`sdk/c/include/valhalla_lrc.h`](sdk/c/include/valhalla_lrc.h)) and dynamic linking against `dist/bin/`:

#### Compiling and Running
```powershell
# Windows (MinGW-W64 GCC)
gcc -I sdk/c/include test/c_test/main.c -L dist/bin -lvalhalla_lrc -o test/c_test/c_test.exe
$env:PATH += ";dist/bin"
.\test\c_test\c_test.exe

# Linux
gcc -I sdk/c/include test/c_test/main.c -L dist/bin -lvalhalla_lrc -o test/c_test/c_test
LD_LIBRARY_PATH=dist/bin ./test/c_test/c_test
```

---

## 📚 Technical Documentation

* 📜 [Technical Memory & Engineering Log](TECHNICAL-MEMORY.md): Continuous chronological engineering log, architectural decisions, empirical benchmarks, and implementation milestones (Milestones 1 to 49).
* 🏛️ [Architectural Deliberations](docs/architectural-deliberations.md): Exhaustive analysis of official HLA standards, market landscapes (Pitch, MAK, Portico), licensing frameworks, and cryptographic design principles.
* 🗺️ [Project Master Plan](docs/project-master-plan.md): Comprehensive 10-sprint TDD roadmap detailing tasks, test cases, and definitions of done.
* 📐 [Core Technologies Whitepaper](docs/valhalla-core-technologies.md): Deep-dive into Euler Tour numbering, vectorized bitset matching, SSZ Merkle trees, and 3D Morton space-filling curves for DDM.
* 📈 [Master Development Plan](docs/core-development-plan.md): Architectural roadmap and phased milestones for core simulation engine components.
* 🔬 [Benchmark Report](docs/benchmark.md): Complete latency and throughput empirical benchmarks versus baseline RTIs.
* 🔌 [Portico Wire Specification](docs/portico-wire-spec.md): Reverse-engineered packet format specification of Portico's wire protocol.
* ⚙️ [JOS Stream Encoder Guide](docs/jos-encoder.md): Technical specification of the pure Go Java Object Serialization encoder.

---

## 🛠️ Technology Stack

* **Core Engine & Transport:** Go 1.25+
* **Data Model Standard:** IEEE 1516-2025 (HLA-4) & IEEE 1516-2010 (HLA Evolved)
* **Canonical Object Model:** RPR-FOM v2.0 (SISO-STD-001-2015)
* **Serialization & State Verification:** SSZ (Simple Serialize) & Sparse Merkle Trees
* **Cryptographic Suite [Roadmap]:** ML-KEM-768 (FIPS 203), HPKE (RFC 9180), AES-256-GCM

---

## 👤 Authorship & Project Leadership

* **Lead Developer & Creator:** Carlos Magno Oliveira de Abreu ([@icemagno](https://github.com/icemagno))
* **Email:** [magno.mabreu@gmail.com](mailto:magno.mabreu@gmail.com)

---

## 📄 License

Project Valhalla is distributed under the open-source **Apache License, Version 2.0**. Refer to the [LICENSE](file:///c:/Magno/Projetos/hla-rti/LICENSE) file for the complete terms and conditions.
