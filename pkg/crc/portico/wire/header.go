package wire

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// HeaderLength define o tamanho fixo exato de 12 bytes do cabecalho Portico (Header.java)
const HeaderLength = 12

// MagicTCPBundle define o marcador de enquadramento do canal TCP do Portico (0xCAFE = 52094)
const MagicTCPBundle int32 = 0xcafe

// CallType identifica a natureza da chamada segundo CallType.java do Portico
type CallType uint8

const (
	CallTypeDataMessage        CallType = 0
	CallTypeNotification       CallType = 1
	CallTypeControlRequest     CallType = 2
	CallTypeControlResponseOK  CallType = 3
	CallTypeControlResponseErr CallType = 4
)

func (c CallType) String() string {
	switch c {
	case CallTypeDataMessage:
		return "DataMessage(0)"
	case CallTypeNotification:
		return "Notification(1)"
	case CallTypeControlRequest:
		return "ControlRequest(2)"
	case CallTypeControlResponseOK:
		return "ControlResponseOK(3)"
	case CallTypeControlResponseErr:
		return "ControlResponseErr(4)"
	default:
		return fmt.Sprintf("CallTypeUnknown(%d)", c)
	}
}

// MessageType enumera os identificadores oficiais do Portico segundo MessageType.java
type MessageType uint8

const (
	// Mensagens de Ciclo de Vida e Sessao (000-009)
	MsgConnect           MessageType = 0
	MsgDisconnect        MessageType = 1
	MsgRtiProbe          MessageType = 2
	MsgCreateFederation  MessageType = 3
	MsgJoinFederation    MessageType = 4
	MsgResignFederation  MessageType = 5
	MsgDestroyFederation MessageType = 6
	MsgListFederations   MessageType = 7
	MsgAuthenticate      MessageType = 8

	// Respostas de Controle (010-039)
	MsgSuccessResponse MessageType = 14
	MsgErrorResponse   MessageType = 15

	// Pontos de Sincronizacao (040-049)
	MsgRegisterSyncPoint       MessageType = 40
	MsgRegisterSyncPointResult MessageType = 41
	MsgAnnounceSyncPoint       MessageType = 42
	MsgAchieveSyncPoint        MessageType = 43
	MsgFederationSynchronized  MessageType = 44
	MsgSyncPointFederation     MessageType = 44

	// Publicacao e Subscricao (050-059)
	MsgPublishObjectClass          MessageType = 50
	MsgPublishInteractionClass     MessageType = 51
	MsgUnpublishObjectClass        MessageType = 52
	MsgUnpublishInteractionClass   MessageType = 53
	MsgSubscribeObjectClass        MessageType = 54
	MsgSubscribeInteractionClass   MessageType = 55
	MsgUnsubscribeObjectClass      MessageType = 56
	MsgUnsubscribeInteractionClass MessageType = 57

	// Gerenciamento de Objetos (060-079)
	MsgRegisterObject          MessageType = 60
	MsgDiscoverObject          MessageType = 61
	MsgDeleteObject            MessageType = 64
	MsgLocalDeleteObject       MessageType = 65
	MsgReserveObjectName       MessageType = 66
	MsgReserveObjectNameResult MessageType = 67
	MsgRequestObjectUpdate     MessageType = 68
	MsgRequestClassUpdate      MessageType = 69

	// Prioritárias / Simulação em tempo real (254, 255)
	MsgUpdateAttributes  MessageType = 254
	MsgReflectAttributes MessageType = 254
	MsgSendInteraction   MessageType = 255
	MsgReceiveInteraction MessageType = 255

	// Time Management (120-139)
	MsgEnableTimeConstrained  MessageType = 120
	MsgDisableTimeConstrained MessageType = 121
	MsgEnableTimeRegulation   MessageType = 122
	MsgDisableTimeRegulation  MessageType = 123
	MsgModifyLookahead        MessageType = 124
	MsgTimeAdvanceRequest     MessageType = 125
	MsgTimeAdvanceGrant       MessageType = 126
	MsgNextEventRequest       MessageType = 127
	MsgFlushQueueRequest      MessageType = 128
)

func (m MessageType) String() string {
	switch m {
	case MsgConnect:
		return "Connect(0)"
	case MsgDisconnect:
		return "Disconnect(1)"
	case MsgRtiProbe:
		return "RtiProbe(2)"
	case MsgCreateFederation:
		return "CreateFederation(3)"
	case MsgJoinFederation:
		return "JoinFederation(4)"
	case MsgResignFederation:
		return "ResignFederation(5)"
	case MsgDestroyFederation:
		return "DestroyFederation(6)"
	case MsgListFederations:
		return "ListFederations(7)"
	case MsgAuthenticate:
		return "Authenticate(8)"
	case MsgSuccessResponse:
		return "SuccessResponse(14)"
	case MsgErrorResponse:
		return "ErrorResponse(15)"
	case MsgRegisterObject:
		return "RegisterObject(60)"
	case MsgDiscoverObject:
		return "DiscoverObject(61)"
	case MsgDeleteObject:
		return "DeleteObject(64)"
	case MsgUpdateAttributes:
		return "UpdateAttributes(254)"
	case MsgSendInteraction:
		return "SendInteraction(255)"
	default:
		return fmt.Sprintf("MessageType(%d)", m)
	}
}

// Flags de controle de cabecalho (Byte 0) - Mapeamento exato de BitHelpers.readBooleanBit(0..7)
const (
	FlagBundle              uint8 = 0x01 // Bit 0: Bundle presente
	FlagEncrypted           uint8 = 0x02 // Bit 1: Encriptado (Nonce/IV presente)
	FlagAuthenticated       uint8 = 0x04 // Bit 2: Autenticado (Token presente)
	FlagManualMarshal       uint8 = 0x08 // Bit 3: Serializacao manual sem reflexao Java
	FlagFiltering           uint8 = 0x10 // Bit 4: Filtro ativo (RequestId vira FilteringId)
	FlagFilteringObjClass   uint8 = 0x20 // Bit 5: 1=Objeto, 0=Interacao
	FlagReserved            uint8 = 0x40 // Bit 6: Reservado Portico
	FlagValhallaExtended    uint8 = 0x80 // Bit 7: Extensao Valhalla (SSZ / Merkle / KEM)
)

var (
	ErrBufferTooShort  = errors.New("portico wire: buffer menor que 12 bytes do cabecalho")
	ErrPayloadTooLarge = errors.New("portico wire: tamanho do payload excede 24 bits (16 MB)")
	ErrInvalidCallType = errors.New("portico wire: calltype invalido")
)

// Header representa com fidelidade absoluta de bits o cabecalho de 12 bytes do Portico (Header.java)
type Header struct {
	Flags         uint8       // Byte 0 (8 bits)
	PayloadLength uint32      // Bytes 1-3 (24 bits Big-Endian, 0 a 16.777.215 bytes)
	FederationID  uint8       // Byte 4 High Nibble (bits 4-7, 0..15)
	CallType      CallType    // Byte 4 Low Nibble (bits 0-3, 0..4)
	MessageType   MessageType // Byte 5 (8 bits)
	RequestId     uint16      // Bytes 6-7 (16 bits Big-Endian) ou FilteringId
	SourceHandle  uint16      // Bytes 8-9 (16 bits Big-Endian)
	TargetHandle  uint16      // Bytes 10-11 (16 bits Big-Endian)
}

// NewControlRequest cria um cabecalho para requisicoes de controle síncronas/assíncronas
func NewControlRequest(msgType MessageType, reqId uint16, source, target uint16, payloadLen uint32) Header {
	return Header{
		Flags:         0,
		PayloadLength: payloadLen,
		FederationID:  0,
		CallType:      CallTypeControlRequest,
		MessageType:   msgType,
		RequestId:     reqId,
		SourceHandle:  source,
		TargetHandle:  target,
	}
}

// NewDataMessage cria um cabecalho para mensagens de simulacao (ex: UpdateAttributes, SendInteraction)
func NewDataMessage(msgType MessageType, fedId uint8, source, target uint16, payloadLen uint32) Header {
	return Header{
		Flags:         FlagManualMarshal,
		PayloadLength: payloadLen,
		FederationID:  fedId,
		CallType:      CallTypeDataMessage,
		MessageType:   msgType,
		RequestId:     0,
		SourceHandle:  source,
		TargetHandle:  target,
	}
}

// MarshalTo codifica os 12 bytes diretamente no buffer de destino sem alocar memoria (Zero-Alloc)
func (h Header) MarshalTo(buf []byte) error {
	if len(buf) < HeaderLength {
		return ErrBufferTooShort
	}
	if h.PayloadLength > 0x00FFFFFF {
		return ErrPayloadTooLarge
	}

	// Byte 0: Flags
	buf[0] = h.Flags

	// Bytes 1-3: PayloadLength (24-bit uint Big-Endian)
	buf[1] = byte((h.PayloadLength >> 16) & 0xFF)
	buf[2] = byte((h.PayloadLength >> 8) & 0xFF)
	buf[3] = byte(h.PayloadLength & 0xFF)

	// Byte 4: FederationID (bits 7-4) | CallType (bits 3-0)
	buf[4] = ((h.FederationID & 0x0F) << 4) | (byte(h.CallType) & 0x0F)

	// Byte 5: MessageType (8 bits)
	buf[5] = byte(h.MessageType)

	// Bytes 6-7: RequestId (16 bits Big-Endian)
	binary.BigEndian.PutUint16(buf[6:8], h.RequestId)

	// Bytes 8-9: SourceHandle (16 bits Big-Endian)
	binary.BigEndian.PutUint16(buf[8:10], h.SourceHandle)

	// Bytes 10-11: TargetHandle (16 bits Big-Endian)
	binary.BigEndian.PutUint16(buf[10:12], h.TargetHandle)

	return nil
}

// MarshalBinary implementa encoding.BinaryMarshaler
func (h Header) MarshalBinary() ([]byte, error) {
	buf := make([]byte, HeaderLength)
	err := h.MarshalTo(buf)
	return buf, err
}

// UnmarshalBinary implementa encoding.BinaryUnmarshaler
func (h *Header) UnmarshalBinary(data []byte) error {
	if len(data) < HeaderLength {
		return ErrBufferTooShort
	}

	// Byte 0: Flags
	h.Flags = data[0]

	// Bytes 1-3: PayloadLength (24 bits Big-Endian)
	h.PayloadLength = uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3])

	// Byte 4: FederationID (high nibble) e CallType (low nibble)
	h.FederationID = (data[4] >> 4) & 0x0F
	h.CallType = CallType(data[4] & 0x0F)

	// Byte 5: MessageType
	h.MessageType = MessageType(data[5])

	// Bytes 6-7: RequestId
	h.RequestId = binary.BigEndian.Uint16(data[6:8])

	// Bytes 8-9: SourceHandle
	h.SourceHandle = binary.BigEndian.Uint16(data[8:10])

	// Bytes 10-11: TargetHandle
	h.TargetHandle = binary.BigEndian.Uint16(data[10:12])

	return nil
}

// String provê representacao legivel e descritiva para logging e auditoria
func (h Header) String() string {
	return fmt.Sprintf("PorticoHeader{Msg: %s, Call: %s, Fed: %d, ReqID: %d, Len: %d, Src: %d, Tgt: %d, Flags: 0x%02X}",
		h.MessageType, h.CallType, h.FederationID, h.RequestId, h.PayloadLength, h.SourceHandle, h.TargetHandle, h.Flags)
}
