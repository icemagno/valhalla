package crc

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
)

const (
	NativeHeaderMagic   uint16 = 0x5641 // "VA" (Valhalla)
	NativeProtocolVer   uint8  = 4      // HLA-4 (IEEE 1516-2025)
	NativeHeaderLength         = 56     // 24 bytes cabecalho alinhado + 32 bytes Merkle Root
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

// NativeSession gerencia a conexão de um federado moderno nativo do Valhalla
type NativeSession struct {
	id       int32
	fedID    int32
	conn     net.Conn
	writeMu  sync.Mutex
	isClosed bool
}

// NewNativeSession instancia uma nova sessão nativa
func NewNativeSession(conn net.Conn) *NativeSession {
	return &NativeSession{
		conn: conn,
	}
}

func (s *NativeSession) FederateID() int32 {
	return s.id
}

func (s *NativeSession) SetFederateID(id int32) {
	s.id = id
}

func (s *NativeSession) SetFederationID(fedID int32) {
	s.fedID = fedID
}

func (s *NativeSession) Protocol() ProtocolType {
	return ProtocolNative
}

// SendMessage envia uma mensagem nativa enquadrada com o cabeçalho de 48 bytes
func (s *NativeSession) SendMessage(hdr NativeHeader, payload []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if s.isClosed {
		return net.ErrClosed
	}

	var buf [NativeHeaderLength]byte
	binary.BigEndian.PutUint16(buf[0:2], hdr.Magic)
	buf[2] = hdr.Version
	buf[3] = hdr.Flags
	binary.BigEndian.PutUint32(buf[4:8], uint32(len(payload)))
	buf[8] = hdr.MsgType
	buf[9] = hdr.Reserved
	binary.BigEndian.PutUint16(buf[10:12], hdr.ReqID)
	binary.BigEndian.PutUint32(buf[12:16], hdr.FedID)
	binary.BigEndian.PutUint32(buf[16:20], hdr.FederateID)
	binary.BigEndian.PutUint32(buf[20:24], 0) // Reserved2
	copy(buf[24:56], hdr.MerkleRoot[:])

	if _, err := s.conn.Write(buf[:]); err != nil {
		return err
	}
	if len(payload) > 0 {
		if _, err := s.conn.Write(payload); err != nil {
			return err
		}
	}
	return nil
}

// SendNotification envia uma notificação nativa para o federado
func (s *NativeSession) SendNotification(msgType uint8, payload []byte) error {
	hdr := NativeHeader{
		Magic:      NativeHeaderMagic,
		Version:    NativeProtocolVer,
		Flags:      0x01, // SSZ
		PayloadLen: uint32(len(payload)),
		MsgType:    msgType,
		FedID:      uint32(s.fedID),
		FederateID: uint32(s.id),
	}
	return s.SendMessage(hdr, payload)
}

// Close encerra a sessão nativa
func (s *NativeSession) Close() error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.isClosed = true
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

// ReadHeader le e decodifica o cabecalho nativo de 48 bytes
func ReadNativeHeader(r io.Reader) (NativeHeader, error) {
	var buf [NativeHeaderLength]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return NativeHeader{}, err
	}

	magic := binary.BigEndian.Uint16(buf[0:2])
	if magic != NativeHeaderMagic {
		return NativeHeader{}, fmt.Errorf("magic nativo invalido: 0x%04X (esperado 0x%04X)", magic, NativeHeaderMagic)
	}

	hdr := NativeHeader{
		Magic:      magic,
		Version:    buf[2],
		Flags:      buf[3],
		PayloadLen: binary.BigEndian.Uint32(buf[4:8]),
		MsgType:    buf[8],
		Reserved:   buf[9],
		ReqID:      binary.BigEndian.Uint16(buf[10:12]),
		FedID:      binary.BigEndian.Uint32(buf[12:16]),
		FederateID: binary.BigEndian.Uint32(buf[16:20]),
		Reserved2:  binary.BigEndian.Uint32(buf[20:24]),
	}
	copy(hdr.MerkleRoot[:], buf[24:56])

	return hdr, nil
}
