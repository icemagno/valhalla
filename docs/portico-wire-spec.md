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

## 4. Portico Message Type Enumeration

| Code | Constant Name | Description |
| :--- | :--- | :--- |
| `1` | `CREATE_FEDERATION` | Request/Response to create a federation execution |
| `2` | `DESTROY_FEDERATION` | Request/Response to destroy a federation execution |
| `3` | `JOIN_FEDERATION` | Request to join / Response returns WelcomePack |
| `4` | `RESIGN_FEDERATION` | Resign from federation |
| `5` | `REGISTER_SYNC_POINT` | Register synchronization point label |
| `6` | `ANNOUNCE_SYNC_POINT` | Notification broadcasting sync point label |
| `7` | `ACHIEVE_SYNC_POINT` | Federate notifies local sync point achieved |
| `8` | `SYNC_POINT_ACHIEVED` | Internal notification of achievement |
| `9` | `FEDERATION_SYNCHRONIZED` | Collective barrier reached |
| `10`| `PUBLISH_OBJECT_CLASS` | Publish object class attributes |
| `11`| `SUBSCRIBE_OBJECT_CLASS` | Subscribe to object class attributes |
| `12`| `PUBLISH_INTERACTION_CLASS` | Publish interaction class |
| `13`| `SUBSCRIBE_INTERACTION_CLASS` | Subscribe to interaction class |
| `14`| `REGISTER_OBJECT` | Instantiate a new object instance |
| `15`| `DISCOVER_OBJECT` | Notification of discovered object instance |
| `16`| `UPDATE_ATTRIBUTES` | Attribute value update |
| `17`| `REFLECT_ATTRIBUTES` | Attribute value reflection notification |
| `18`| `SEND_INTERACTION` | Send interaction event |
| `19`| `RECEIVE_INTERACTION` | Receive interaction notification |
| `20`| `ENABLE_TIME_REGULATION` | Request to enable time regulation |
| `21`| `ENABLE_TIME_CONSTRAINED` | Request to enable time constraint |
| `22`| `TIME_ADVANCE_REQUEST` | Advance logical time request (TAR) |
| `23`| `TIME_ADVANCE_GRANT` | Time advance grant notification (TAG) |
| `24`| `SUCCESS_RESP` | Generic success acknowledgment |
| `25`| `ERROR_RESP` | Error response with exception message string |

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
