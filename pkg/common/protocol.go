package common

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	NativeHeaderMagic   uint16 = 0x5641 // "VA" (Valhalla)
	NativeProtocolVer   uint8  = 4      // HLA-4 (IEEE 1516-2025)
	NativeHeaderLength         = 56     // 24 aligned header bytes + 32-byte Merkle Root
	MaxNativePayloadLen uint32 = 64 * 1024 * 1024 // 64 MB maximum safety limit against OOM DoS
)

// Native Protocol Message Types (HLA-4 / IEEE 1516-2025)
const (
	// 01-19: Federation Management & Lifecycle
	NativeMsgConnect                uint8 = 1
	NativeMsgDisconnect             uint8 = 2
	NativeMsgCreateFederation       uint8 = 3
	NativeMsgDestroyFederation      uint8 = 4
	NativeMsgJoin                   uint8 = 5
	NativeMsgJoinSuccess            uint8 = 6
	NativeMsgResign                 uint8 = 7
	NativeMsgRegisterSyncPoint      uint8 = 8
	NativeMsgAnnounceSyncPoint      uint8 = 9
	NativeMsgAchieveSyncPoint       uint8 = 10
	NativeMsgSyncPointAchieved      uint8 = 11
	NativeMsgFederationSynchronized uint8 = 12
	NativeMsgSyncPointRegSucceeded  uint8 = 13
	NativeMsgSyncPointRegFailed     uint8 = 14

	// 20-39: Declaration Management (Pub/Sub)
	NativeMsgPublishObjectClass           uint8 = 20
	NativeMsgUnpublishObjectClass         uint8 = 21
	NativeMsgPublishInteractionClass      uint8 = 22
	NativeMsgUnpublishInteractionClass    uint8 = 23
	NativeMsgSubscribeObjectClassAttrs    uint8 = 24
	NativeMsgUnsubscribeObjectClass       uint8 = 25
	NativeMsgSubscribeInteractionClass    uint8 = 26
	NativeMsgUnsubscribeInteractionClass  uint8 = 27

	// 40-59: Object & Interaction Management
	NativeMsgRegisterObject       uint8 = 40
	NativeMsgDiscoverObject       uint8 = 41
	NativeMsgUpdateAttributes     uint8 = 42
	NativeMsgReflectAttributes    uint8 = 43
	NativeMsgSendInteraction      uint8 = 44
	NativeMsgReceiveInteraction   uint8 = 45
	NativeMsgDeleteObjectInstance uint8 = 46
	NativeMsgRemoveObjectInstance uint8 = 47

	// 60-79: Time Management
	NativeMsgEnableTimeRegulation    uint8 = 60
	NativeMsgTimeRegulationEnabled   uint8 = 61
	NativeMsgDisableTimeRegulation   uint8 = 62
	NativeMsgEnableTimeConstrained   uint8 = 63
	NativeMsgTimeConstrainedEnabled  uint8 = 64
	NativeMsgDisableTimeConstrained  uint8 = 65
	NativeMsgTimeAdvanceRequest      uint8 = 66
	NativeMsgTimeAdvanceGrant        uint8 = 67
	NativeMsgNextMessageRequest      uint8 = 68
	NativeMsgFlushQueueRequest       uint8 = 69

	// 80-89: Ownership Management
	NativeMsgQueryAttributeOwnership    uint8 = 80
	NativeMsgAttributeOwnershipResponse uint8 = 81
	NativeMsgDivestOwnership            uint8 = 82
	NativeMsgAcquireOwnership           uint8 = 83

	// 100-119: Data Distribution Management (DDM)
	NativeMsgCreateRegion            uint8 = 100
	NativeMsgRegionCreated           uint8 = 101
	NativeMsgSetRangeBounds          uint8 = 102
	NativeMsgSetMetricBounds         uint8 = 103
	NativeMsgSubscribeWithRegion     uint8 = 104
	NativeMsgUpdateAttributesSpatial uint8 = 105

	// 120-139: OMT / FOM Introspection & Handle Resolution
	NativeMsgGetObjectClassHandle          uint8 = 120
	NativeMsgGetObjectClassHandleResp      uint8 = 121
	NativeMsgGetAttributeHandle            uint8 = 122
	NativeMsgGetAttributeHandleResp        uint8 = 123
	NativeMsgGetDimensionHandle            uint8 = 124
	NativeMsgGetDimensionHandleResp        uint8 = 125
	NativeMsgGetInteractionClassHandle     uint8 = 126
	NativeMsgGetInteractionClassHandleResp uint8 = 127
	NativeMsgGetParameterHandle            uint8 = 128
	NativeMsgGetParameterHandleResp        uint8 = 129

	// 140-149: Merkle State Proofs & Integrity Verification
	NativeMsgGetMerkleProof     uint8 = 140
	NativeMsgGetMerkleProofResp uint8 = 141

	// 90-99: Generic Control Responses
	NativeMsgSuccessOK uint8 = 90
	NativeMsgError     uint8 = 91
)

// Native Header Flags
const (
	FlagNativeSSZ    uint8 = 0x01 // Bit 0: SSZ-encoded payload
	FlagNativeMerkle uint8 = 0x02 // Bit 1: Valid Merkle Root present in header
	FlagNativePQC    uint8 = 0x04 // Bit 2: Payload protected with PQC key (ML-KEM / Kyber)
)

// NativeHeader defines the canonical, high-speed linear binary header for Project Valhalla
type NativeHeader struct {
	Magic      uint16   // 0x5641 ("VA")
	Version    uint8    // 4 (HLA-4 / IEEE 1516-2025)
	Flags      uint8    // Bit 0: SSZ, Bit 1: Merkle, Bit 2: PQC
	PayloadLen uint32   // SSZ payload length
	MsgType    uint8    // Native message type identifier
	Reserved   uint8    // Alignment byte
	ReqID      uint16   // Request identifier for sync correlation
	FedID      uint32   // Federation execution ID
	FederateID uint32   // Federate member ID
	Reserved2  uint32   // Alignment for 32-byte Merkle Root
	MerkleRoot [32]byte // 32-byte deterministic state Merkle Root
}

// ReadNativeHeader deserializes the 56-byte native binary header from an io.Reader
func ReadNativeHeader(r io.Reader) (NativeHeader, error) {
	var buf [NativeHeaderLength]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return NativeHeader{}, err
	}

	magic := binary.LittleEndian.Uint16(buf[0:2])
	if magic != NativeHeaderMagic {
		return NativeHeader{}, fmt.Errorf("invalid native magic: 0x%04X", magic)
	}

	version := buf[2]
	if version != NativeProtocolVer {
		return NativeHeader{}, fmt.Errorf("unsupported native protocol version: %d", version)
	}

	payloadLen := binary.LittleEndian.Uint32(buf[4:8])
	if payloadLen > MaxNativePayloadLen {
		return NativeHeader{}, fmt.Errorf("%w: %d bytes (limit: %d)", ErrNativePayloadTooLarge, payloadLen, MaxNativePayloadLen)
	}

	var merkle [32]byte
	copy(merkle[:], buf[24:56])

	return NativeHeader{
		Magic:      magic,
		Version:    version,
		Flags:      buf[3],
		PayloadLen: payloadLen,
		MsgType:    buf[8],
		Reserved:   buf[9],
		ReqID:      binary.LittleEndian.Uint16(buf[10:12]),
		FedID:      binary.LittleEndian.Uint32(buf[12:16]),
		FederateID: binary.LittleEndian.Uint32(buf[16:20]),
		Reserved2:  binary.LittleEndian.Uint32(buf[20:24]),
		MerkleRoot: merkle,
	}, nil
}

// WriteNativeHeader serializes the 56-byte native binary header into a fixed array
func WriteNativeHeader(hdr NativeHeader) [NativeHeaderLength]byte {
	var buf [NativeHeaderLength]byte
	binary.LittleEndian.PutUint16(buf[0:2], NativeHeaderMagic)
	buf[2] = NativeProtocolVer
	buf[3] = hdr.Flags
	binary.LittleEndian.PutUint32(buf[4:8], hdr.PayloadLen)
	buf[8] = hdr.MsgType
	buf[9] = hdr.Reserved
	binary.LittleEndian.PutUint16(buf[10:12], hdr.ReqID)
	binary.LittleEndian.PutUint32(buf[12:16], hdr.FedID)
	binary.LittleEndian.PutUint32(buf[16:20], hdr.FederateID)
	binary.LittleEndian.PutUint32(buf[20:24], hdr.Reserved2)
	copy(buf[24:56], hdr.MerkleRoot[:])
	return buf
}

// Common native protocol errors
var (
	ErrNativeInvalidMagic    = errors.New("native protocol: invalid magic")
	ErrNativeInvalidVersion  = errors.New("native protocol: unsupported version")
	ErrNativePayloadTooLarge = errors.New("native protocol: payload exceeds maximum safety limit")
)
