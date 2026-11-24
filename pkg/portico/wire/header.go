package wire

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// MagicPortico define o número mágico Big-Endian "PORT" (0x504F5254)
const MagicPortico uint32 = 0x504F5254

// HeaderSize define o tamanho fixo do cabeçalho binário em bytes
const HeaderSize = 24

// Flags de controle de mensagem
const (
	FlagTimestampOrder uint16 = 0x0001
	FlagCompressed     uint16 = 0x0002
	FlagSyncResponse   uint16 = 0x0004
	FlagValhallaExt    uint16 = 0x0080 // Extensão Valhalla (SSZ / KEM / HLA-4)
)

// MessageType enumera os identificadores de operação compatíveis com Portico
type MessageType uint16

const (
	MsgSuccessResp         MessageType = 0x0001
	MsgErrorResp           MessageType = 0x0002
	MsgCreateFederation    MessageType = 0x0010
	MsgDestroyFederation   MessageType = 0x0011
	MsgJoinFederation      MessageType = 0x0012
	MsgResignFederation    MessageType = 0x0013
	MsgRegisterObject      MessageType = 0x0020
	MsgDiscoverObject      MessageType = 0x0021
	MsgUpdateAttributes    MessageType = 0x0022
	MsgReflectAttributes   MessageType = 0x0023
	MsgSendInteraction     MessageType = 0x0030
	MsgReceiveInteraction  MessageType = 0x0031
	MsgTimeAdvanceRequest  MessageType = 0x0040
	MsgTimeAdvanceGrant    MessageType = 0x0041
	MsgSyncPointRegister   MessageType = 0x0050
	MsgSyncPointAchieved   MessageType = 0x0051
)

var (
	ErrInvalidMagic      = errors.New("portico wire: magic number invalido")
	ErrBufferTooShort    = errors.New("portico wire: buffer menor que o tamanho minimo do cabecalho")
	ErrPayloadSizeExceed = errors.New("portico wire: tamanho do payload excede o limite permitido")
)

// Header representa o cabeçalho binário de 24 bytes que prefixa mensagens Portico
type Header struct {
	Magic         uint32
	Version       uint16
	Type          MessageType
	SourceHandle  int32
	TargetHandle  int32
	Flags         uint16
	Reserved      uint16
	PayloadLength uint32
}

// NewHeader cria um cabeçalho válido inicializado com Magic e Versão padrão
func NewHeader(msgType MessageType, source, target int32, payloadLen uint32) Header {
	return Header{
		Magic:         MagicPortico,
		Version:       0x0002, // Portico v2.x
		Type:          msgType,
		SourceHandle:  source,
		TargetHandle:  target,
		Flags:         0,
		Reserved:      0,
		PayloadLength: payloadLen,
	}
}

// MarshalTo codifica o cabeçalho diretamente em um slice pré-alocado (Zero-Alloc)
func (h Header) MarshalTo(buf []byte) error {
	if len(buf) < HeaderSize {
		return ErrBufferTooShort
	}

	binary.BigEndian.PutUint32(buf[0:4], h.Magic)
	binary.BigEndian.PutUint16(buf[4:6], h.Version)
	binary.BigEndian.PutUint16(buf[6:8], uint16(h.Type))
	binary.BigEndian.PutUint32(buf[8:12], uint32(h.SourceHandle))
	binary.BigEndian.PutUint32(buf[12:16], uint32(h.TargetHandle))
	binary.BigEndian.PutUint16(buf[16:18], h.Flags)
	binary.BigEndian.PutUint16(buf[18:20], h.Reserved)
	binary.BigEndian.PutUint32(buf[20:24], h.PayloadLength)

	return nil
}

// MarshalBinary serializa o cabeçalho em um novo slice de 24 bytes
func (h Header) MarshalBinary() ([]byte, error) {
	buf := make([]byte, HeaderSize)
	err := h.MarshalTo(buf)
	return buf, err
}

// UnmarshalBinary decodifica um cabeçalho a partir de um slice de bytes
func (h *Header) UnmarshalBinary(data []byte) error {
	if len(data) < HeaderSize {
		return ErrBufferTooShort
	}

	magic := binary.BigEndian.Uint32(data[0:4])
	if magic != MagicPortico {
		return fmt.Errorf("%w: esperado 0x%X, obtido 0x%X", ErrInvalidMagic, MagicPortico, magic)
	}

	h.Magic = magic
	h.Version = binary.BigEndian.Uint16(data[4:6])
	h.Type = MessageType(binary.BigEndian.Uint16(data[6:8]))
	h.SourceHandle = int32(binary.BigEndian.Uint32(data[8:12]))
	h.TargetHandle = int32(binary.BigEndian.Uint32(data[12:16]))
	h.Flags = binary.BigEndian.Uint16(data[16:18])
	h.Reserved = binary.BigEndian.Uint16(data[18:20])
	h.PayloadLength = binary.BigEndian.Uint32(data[20:24])

	return nil
}
