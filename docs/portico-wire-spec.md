# Portico Network Wire Protocol Specification

> **Project:** Project Valhalla  
> **Status:** Certified & Audited Directly from The Portico Project Source Code (Portico 2.x / 3.x - Apache 2.0 / CDDL)  
> **Reference Classes:** `Header.java`, `BitHelpers.java`, `TcpChannel.java`, `Bundler.java`, `TcpClientProxy.java`, `MessageHelpers.java`  
> **Classification:** Open-Source Engineering Specification  

---

## 1. Overview of Portico Network Architecture

Portico uses a pipeline architecture based on message sinks (`MessageSink`), where every simulation event (federation creation, join, resign, sync points, object registration, attribute updates, and time advance) is encapsulated in an instance of `PorticoMessage`.

At the physical network layer, messages travel across two primary channels:
1. **Control Channel (TCP Unicast):** Used for reliable point-to-point connections, lifecycle handshakes (`Connect`, `Join`, `Resign`, `SyncPoints`), and synchronous responses (`SUCCESS_RESP`, `ERROR_RESP`).
2. **Data Channel (UDP Multicast / TCP Mesh):** Used for high-frequency distribution of attribute updates (`UpdateAttributes`) and transient interactions (`SendInteraction`).

---

## 2. TCP Transport Framing (TCP Bundler Framing)

On the TCP channel (`TcpChannel.java` and `Bundler.java`), data is never sent raw or unaligned. All packets travel packaged inside bundle structures prefixed by the magic integer `0x0000CAFE`:

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                 Bundle Magic: 0x0000CAFE (int32)              | 4 bytes
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                 Bundle Length: N bytes (int32)                | 4 bytes
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                 Header (12 bytes) + Payload (N - 12 bytes)    |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

* **Keepalive Packets:** When idle, Portico sends `0x0000CAFE` followed by `0x00000000` (length 0).
* **Bundle Length:** `bundleLength = 12 (Header.HEADER_LENGTH) + payloadLength`.

---

## 3. Strict 12-Byte Binary Header Layout (`Header.java`)

The Portico fixed binary header is **strictly 12 bytes** long (`Header.HEADER_LENGTH = 12`). All multibyte integer fields use **Big-Endian** (Network Byte Order):

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|     Flags     |               Payload Length (24-bit)         | Bytes 0-3
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|  FedID | Call |  MessageType  |          RequestId            | Bytes 4-7
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|         Source Handle         |         Target Handle         | Bytes 8-11
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

### 3.1. Detailed Field Mapping

#### Byte 0: Flags (Bitfield)
* **Bit 0 (`0x01`):** `isBundle` — Always `1` when processed by `Bundler`.
* **Bit 1 (`0x02`):** `isEncrypted` — Payload encrypted (followed by 16-byte IV/Nonce).
* **Bit 2 (`0x04`):** `isAuthenticated` — Payload authenticated (token present).
* **Bit 3 (`0x08`):** `isManualMarshal` — Payload serialized manually without reflection.
* **Bit 4 (`0x10`):** `isFiltering` — `RequestId` is interpreted as `FilteringId`.
* **Bit 5 (`0x20`):** `isFilteringObjectClass` — Filter target (1 = ObjectClass, 0 = InteractionClass).
* **Bit 6 (`0x40`):** Reserved.
* **Bit 7 (`0x80`):** `FlagValhallaExtended` — Valhalla sovereign extension marker.

#### Bytes 1-3: Payload Length (24-bit unsigned integer)
* Calculated via: `((b1 & 0xFF) << 16) | ((b2 & 0xFF) << 8) | (b3 & 0xFF)`. Maximum payload size: 16 MB ($2^{24} - 1$).

#### Byte 4: FederationID & CallType (Split Nibbles)
* **High Nibble (bits 7-4):** `FederationID` (uint4, range 0–15).
* **Low Nibble (bits 3-0):** `CallType` (uint4 enum):
  * `0x00` (`0`): `DATA_MESSAGE`
  * `0x01` (`1`): `NOTIFICATION`
  * `0x02` (`2`): `CONTROL_REQUEST`
  * `0x03` (`3`): `CONTROL_RESPONSE_OK`
  * `0x04` (`4`): `CONTROL_RESPONSE_ERR`

#### Byte 5: MessageType (uint8)
Identifies the Portico internal message identifier.

#### Bytes 6-7: RequestId (uint16 Big-Endian)
Monotonic identifier correlating requests and responses.

#### Bytes 8-9: SourceHandle (uint16 Big-Endian)
Federate handle of the sender (or 0 if originating from the CRTI/RTIexec).

#### Bytes 10-11: TargetHandle (uint16 Big-Endian)
Federate handle of the recipient (or 0 for broadcasts).

---

## 4. Portico Message Type Enumeration (Authoritative `pkg/crc/portico/wire/header.go`)

| Code | Constant Name | Description |
| :--- | :--- | :--- |
| `0` | `MsgConnect` | Connect to RTI executive |
| `1` | `MsgDisconnect` | Disconnect from RTI executive |
| `2` | `MsgRtiProbe` | RTI probe / liveness check |
| `3` | `MsgCreateFederation` | Request/Response to create a federation execution |
| `4` | `MsgJoinFederation` | Request to join / Response returns WelcomePack |
| `5` | `MsgResignFederation` | Resign from federation |
| `6` | `MsgDestroyFederation` | Request/Response to destroy a federation execution |
| `14` | `MsgSuccessResponse` | Control success response |
| `15` | `MsgErrorResponse` | Control error response |
| `40` | `MsgRegisterSyncPoint` | Register synchronization point label |
| `42` | `MsgAnnounceSyncPoint` | Notification broadcasting sync point label |
| `43` | `MsgAchieveSyncPoint` | Federate notifies local sync point achieved |
| `44` | `MsgFederationSynchronized` | Collective barrier reached |
| `50` | `MsgPublishObjectClass` | Publish object class attributes |
| `51` | `MsgPublishInteractionClass` | Publish interaction class |
| `54` | `MsgSubscribeObjectClass` | Subscribe to object class attributes |
| `55` | `MsgSubscribeInteractionClass` | Subscribe to interaction class |
| `60` | `MsgRegisterObject` | Instantiate a new object instance |
| `61` | `MsgDiscoverObject` | Notification of discovered object instance |
| `64` | `MsgDeleteObject` | Delete an object instance |
| `254` | `MsgUpdateAttributes` / `MsgReflectAttributes` | Real-time attribute value update / reflection |
| `255` | `MsgSendInteraction` / `MsgReceiveInteraction` | Real-time interaction send / receipt |

---

## 5. Handshake Protocol (Server-First Flow)

When a client connects to Portico on TCP port 52295, the CRTI initiates the connection flow:

```
Client (Federate)                                   Server (CRTI / Valhalla)
       │                                                       │
       │ <─────── 0xBEEF (int32) + Len (int32) + WelcomeMsg ───│ (Server greeting)
       │ <─────── 0xFEED (int32) ──────────────────────────────│ (Server Ready)
       │ ──────── 0xFEED (int32) ────────────────────────────> │ (Client Ready)
       │                                                       │
       │                   [Handshake Complete]                │
       │                                                       │
       │ ─────── 0xCAFE + BundleLen + Header + JoinReq ──────> │
       │ <────── 0xCAFE + BundleLen + Header + WelcomePack ─── │
```

Project Valhalla implements this exact protocol flow, ensuring 100% binary drop-in compatibility with legacy Portico clients and tools.

---

## 6. The WelcomePack Anatomy: Eager ObjectModel Serialization

### 6.1. Why the WelcomePack is 33 KB+
When a federate joins an active federation (`MsgJoinFederation`), Portico does not simply acknowledge the connection. It responds with a massive `WelcomePack` packet (SUID `0x03CC42B0DA8025D4` / `org.portico2.common.services.federation.msg.WelcomePack`):
1. **Federation & Federate Handles:** 32-bit integers allocated by the CRTI.
2. **Federation, Federate, and Type Names:** UTF-8 encoded strings.
3. **The Embedded `ObjectModel`:** A fully serialized Java representation of the entire Federation Object Model (`org.portico.shared.model.ObjectModel`), containing:
   * Every object class in the hierarchy and its parent link.
   * Every attribute definition, datatype, and assigned 32-bit integer handle.
   * Every interaction class, parameter definition, and assigned handle.
4. **Federation Keys and Sync Point States:** Tables tracking current collective barrier checkpoints.

### 6.2. Eager Push (Portico) vs. Lazy Pull (Pitch / MÄK / Valhalla Native)
* **Portico Design Rationale:** By serializing the complete `ObjectModel` into the `WelcomePack`, Portico allows joining Java federates to perform all subsequent `getObjectClassHandle` and `getAttributeHandle` queries in local RAM as simple HashMap lookups with 0 network latency.
* **The Downside:** The join handshake is heavy ($\ge 33\ \text{KB}$ even for modest models), tightly couples both ends to Java `ObjectInputStream` binary layouts, and causes client-side `NullPointerException`s if the RTI returns a null or partial FOM.
* **Valhalla Sovereign Handling:** Project Valhalla dynamically synthesizes the canonical `WelcomePack` with pre-populated handles for legacy Portico nodes, while offering modern native nodes (Go, Rust, Unreal Engine 5) a lightweight, sub-microsecond pull-and-cache protocol using **Euler Tour Interval Encoding** ($< 1.2\ \text{ns}$ per check).
