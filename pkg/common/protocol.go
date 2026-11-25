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
	NativeHeaderLength         = 56     // 24 bytes cabeçalho alinhado + 32 bytes Merkle Root
	MaxNativePayloadLen uint32 = 64 * 1024 * 1024 // Limite de 64 MB contra esgotamento de memória (OOM)
)

// Tipos de Mensagem do Protocolo Nativo Valhalla (HLA-4 / IEEE 1516-2025)
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
	NativeMsgQueryAttributeOwnership uint8 = 80
	NativeMsgAttributeIsOwned        uint8 = 81

	// 90-99: Respostas Genéricas de Controle
	NativeMsgSuccessOK uint8 = 90
	NativeMsgError     uint8 = 91
)

// Flags de cabeçalho nativo
const (
	FlagNativeSSZ    uint8 = 0x01 // Bit 0: Payload codificado em SSZ
	FlagNativeMerkle uint8 = 0x02 // Bit 1: Merkle Root válida presente no cabeçalho
	FlagNativePQC    uint8 = 0x04 // Bit 2: Payload protegido com chave PQC (ML-KEM / Kyber)
)

// NativeHeader define o cabeçalho canônico linear e de alta velocidade do protocolo nativo do Valhalla
type NativeHeader struct {
	Magic      uint16   // 0x5641
	Version    uint8    // 4
	Flags      uint8    // Bit 0: SSZ, Bit 1: Merkle, Bit 2: PQC
	PayloadLen uint32   // Tamanho do payload SSZ
	MsgType    uint8    // Tipo de mensagem nativa
	Reserved   uint8    // Alinhamento
	ReqID      uint16   // Identificador da requisição
	FedID      uint32   // ID da Federação
	FederateID uint32   // ID do Federado
	Reserved2  uint32   // Alinhamento para Merkle Root
	MerkleRoot [32]byte // Raiz de Merkle do estado/dados (32 bytes)
}

// ReadNativeHeader desserializa o cabeçalho binário nativo de 56 bytes a partir de um reader
func ReadNativeHeader(r io.Reader) (NativeHeader, error) {
	var buf [NativeHeaderLength]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return NativeHeader{}, err
	}

	magic := binary.BigEndian.Uint16(buf[0:2])
	if magic != NativeHeaderMagic {
		return NativeHeader{}, fmt.Errorf("invalid native magic: 0x%04X", magic)
	}

	version := buf[2]
	if version != NativeProtocolVer {
		return NativeHeader{}, fmt.Errorf("unsupported native protocol version: %d", version)
	}

	payloadLen := binary.BigEndian.Uint32(buf[4:8])
	if payloadLen > MaxNativePayloadLen {
		return NativeHeader{}, fmt.Errorf("%w: %d bytes (limite: %d)", ErrNativePayloadTooLarge, payloadLen, MaxNativePayloadLen)
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
		ReqID:      binary.BigEndian.Uint16(buf[10:12]),
		FedID:      binary.BigEndian.Uint32(buf[12:16]),
		FederateID: binary.BigEndian.Uint32(buf[16:20]),
		Reserved2:  binary.BigEndian.Uint32(buf[20:24]),
		MerkleRoot: merkle,
	}, nil
}

// WriteNativeHeader serializa o cabeçalho binário nativo de 56 bytes
func WriteNativeHeader(hdr NativeHeader) [NativeHeaderLength]byte {
	var buf [NativeHeaderLength]byte
	binary.BigEndian.PutUint16(buf[0:2], NativeHeaderMagic)
	buf[2] = NativeProtocolVer
	buf[3] = hdr.Flags
	binary.BigEndian.PutUint32(buf[4:8], hdr.PayloadLen)
	buf[8] = hdr.MsgType
	buf[9] = hdr.Reserved
	binary.BigEndian.PutUint16(buf[10:12], hdr.ReqID)
	binary.BigEndian.PutUint32(buf[12:16], hdr.FedID)
	binary.BigEndian.PutUint32(buf[16:20], hdr.FederateID)
	binary.BigEndian.PutUint32(buf[20:24], hdr.Reserved2)
	copy(buf[24:56], hdr.MerkleRoot[:])
	return buf
}

// Erros comuns de protocolo
var (
	ErrNativeInvalidMagic    = errors.New("native protocol: magic invalido")
	ErrNativeInvalidVersion  = errors.New("native protocol: versao nao suportada")
	ErrNativePayloadTooLarge = errors.New("native protocol: payload excede o limite maximo de seguranca")
)
