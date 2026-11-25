package crc

import (
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

// NativeSession manages the connection of a modern native Valhalla federate
type NativeSession struct {
	id       int32
	fedID    int32
	conn     net.Conn
	writeMu  sync.Mutex
	isClosed bool
}

// NewNativeSession instantiates a new native session
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

// SendMessage sends a framed native message with the 48-byte header
func (s *NativeSession) SendMessage(hdr NativeHeader, payload []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if s.isClosed {
		return net.ErrClosed
	}

	hdr.PayloadLen = uint32(len(payload))
	buf := WriteNativeHeader(hdr)

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

// SendNotification sends an asynchronous native notification to the federate
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

// Close closes the native session and underlying socket
func (s *NativeSession) Close() error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.isClosed = true
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}
