# Java Object Serialization (JOS) Binary Engine in Go: Specification & Architecture

> **Project:** Project Valhalla (HLA-4 / IEEE 1516-2025)  
> **Lead Architect & Developer:** Carlos Magno Oliveira de Abreu (`@icemagno` / `magno.mabreu@gmail.com`)  
> **Package:** `pkg/crc/portico/wire/` and `pkg/crc/portico/wire/metamodel/`  
> **Status:** Certified & Validated Bit-for-Bit against OpenJDK 11/22 and Portico CRTI / LRC 3.0.0  
> **Classification:** Open-Source Engineering Documentation  

---

## 1. Engineering Motivation

The **Portico RTI** is the most widely adopted open-source HLA (IEEE 1516 / HLA 1.3) distributed simulation middleware in the defense, academic, and industrial sectors. The central executive (`rtiexec` / CRTI) and client libraries (`LRC - Local RTI Component`) communicate across the network via proprietary protocols encapsulated inside TCP sockets and UDP Multicast groups.

For administrative control messages (federation creation, destruction, `Join`, `Resign`, and synchronization points), Portico utilizes native **Java Object Serialization (JOS)** via instances of `java.io.ObjectOutputStream` and `java.io.ObjectInputStream`.

**Project Valhalla**, written in **pure Go**, requires **100% native wire-level interoperability** with legacy Portico federates:
* Zero JVM dependency on client machines.
* Zero CGO / JNI wrappers.
* Zero external proxy bridges or sidecars.
* **Zero static template hacks (`.bin` blobs):** All serialization and deserialization is executed dynamically in-memory via canonical Go algorithms.

To achieve this, Valhalla features an extensible, pure Go implementation of the Java Object Serialization stream protocol matching the normative behavior of the JVM's `ObjectOutputStream` and `ObjectInputStream`.

---

## 2. Java Object Serialization (JOS) Binary Grammar

The formal binary grammar of Java Object Serialization defines a token-driven stream prefixed by a 4-byte magic stream header and governed by an internal object reference table:

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|       STREAM_MAGIC (0xACED)   |     STREAM_VERSION (0x0005)   |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|  Token (byte) |                   Element Payload...          |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

### 2.1. Fundamental Control Tokens (JOS Grammar)

| Token | Hex Code | Canonical Java Name | Stream Meaning |
| :--- | :--- | :--- | :--- |
| `TC_NULL` | `0x70` | `NullReference` | Null pointer reference (`null`). Encoded in 1 byte. |
| `TC_REFERENCE` | `0x71` | `PrevObject` | Reference to a previously serialized object or descriptor. Followed by `int32` handle. |
| `TC_CLASSDESC` | `0x72` | `ClassDescriptor` | Class metadata declaration (name, `serialVersionUID`, flags, field descriptors). |
| `TC_OBJECT` | `0x73` | `NewObject` | Instantiation of a new object. Followed by `TC_CLASSDESC` and field values. |
| `TC_STRING` | `0x74` | `NewString` | UTF-8 string. Followed by `uint16` length and string bytes. |
| `TC_ARRAY` | `0x75` | `NewArray` | Array instantiation. |
| `TC_CLASS` | `0x76` | `NewClass` | Reference to a `Class<?>` descriptor. |
| `TC_BLOCKDATA` | `0x77` | `BlockData` | Unstructured primitive block (length $\le 255$ bytes). Followed by `uint8` length. |
| `TC_ENDBLOCKDATA`| `0x78` | `EndBlockData` | Terminator marker for custom `writeObject` or `writeExternal` blocks. |
| `TC_RESET` | `0x79` | `Reset` | Resets the stream's object reference table. |
| `TC_BLOCKDATALONG`| `0x7A` | `BlockDataLong` | Long block data (length $> 255$ bytes). Followed by `uint32` length. |
| `TC_ENUM` | `0x7E` | `NewEnum` | Enumeration constant instance. Followed by class description and string constant. |

---

## 3. Architecture of the Go JOS Engine

The Go implementation is divided into three coordinated layers:

```
┌─────────────────────────────────────────────────────────────┐
│                   PORTICO MESSAGE METAMODEL                 │
│  (PorticoMessage, WelcomePack, ExtendedSuccessResponse,     │
│   CreateFederation, JoinFederation, JavaHashMap, ArrayList) │
└──────────────────────────────┬──────────────────────────────┘
                               │
┌──────────────────────────────▼──────────────────────────────┐
│                    JOSEncoder / JOSDecoder                  │
│  - Object reference handle management (0x7E0000 base)       │
│  - Primitive types (Big-Endian uint16, int32, float64, etc.)│
│  - Custom writeObject / writeExternal dispatch              │
└──────────────────────────────┬──────────────────────────────┘
                               │
┌──────────────────────────────▼──────────────────────────────┐
│                 NETWORK WIRE TRANSPORT (TCP)                │
│  - Portico Bundler (0xCAFE + length + 12-byte header)       │
└─────────────────────────────────────────────────────────────┘
```

### 3.1. Reference Table (`0x7E0000` Base)
In JOS, every class descriptor, new string, array, and object instance is assigned an incremental 32-bit handle starting at `0x007E0000`. When an identical object or class descriptor is serialized a second time, the engine emits `TC_REFERENCE` followed by its handle instead of re-serializing the structure. The Go `JOSEncoder` maintains an internal map `map[any]uint32` matching the JVM's reference table behavior.

### 3.2. Custom Serializer Support (`SC_WRITE_METHOD` & `SC_EXTERNALIZABLE`)
Many core Java collections (`java.util.HashMap`, `java.util.HashSet`, `java.util.ArrayList`) and Portico messages implement custom `writeObject(ObjectOutputStream)` or `writeExternal(ObjectOutput)` methods.

Valhalla provides the `CustomWriter` and `CustomReader` interfaces:

```go
type CustomWriter interface {
    WriteObjectData(w JOSWriter, enc *JOSEncoder) error
}

type CustomReader interface {
    ReadObjectData(r JOSReader, dec *JOSDecoder) error
}
```

This architecture allows standard Java collection internals (such as HashMap capacity, load factor, and bucket entries) to be written with 100% bit-level fidelity without external dependencies.

---

## 4. Verification and Compliance

The JOS engine has been tested against real byte streams dumped from live Portico executions (`test/benchmark` and `pkg/crc/portico/wire/jos_test.go`), confirming round-trip serialization and deserialization:

* `TestJOS_GenericEncoder_RtiProbe`: **PASS (0.00s)**
* `TestJOS_GenericEncoder_JoinFederation`: **PASS (0.00s)**
* `TestJOS_GenericEncoder_CreateAndResign`: **PASS (0.00s)**
* `TestHashSet_MetamodelRoundtrip`: **PASS (0.00s)**
* `TestTimeManagement_MetamodelPayloads`: **PASS (0.00s)**

Valhalla provides complete, native, JVM-free binary interoperability with the worldwide installed base of Portico HLA federations.
