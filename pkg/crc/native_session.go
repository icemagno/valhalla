package crc

import (
	"encoding/binary"
	"net"
	"sync"

	"github.com/icemagno/valhalla/pkg/common"
)

const (
	NativeHeaderMagic  = common.NativeHeaderMagic
	NativeProtocolVer  = common.NativeProtocolVer
	NativeHeaderLength = common.NativeHeaderLength
)

type NativeHeader = common.NativeHeader

var ReadNativeHeader = common.ReadNativeHeader
var WriteNativeHeader = common.WriteNativeHeader

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
