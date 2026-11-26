package crc

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/icemagno/valhalla/pkg/crc/portico/wire"
)

// PorticoSession implementa Session para conexões de federados legados Portico
type PorticoSession struct {
	id         int32
	fedHandle  int32
	conn       net.Conn
	writer     *bufio.Writer
	writeMu    sync.Mutex
	isClosed   bool
}

// NewPorticoSession instancia uma nova sessão para o cliente Portico
func NewPorticoSession(conn net.Conn) *PorticoSession {
	return &PorticoSession{
		conn:   conn,
		writer: bufio.NewWriter(conn),
	}
}

func (s *PorticoSession) FederateID() int32 {
	return s.id
}

func (s *PorticoSession) SetFederateID(id int32) {
	s.id = id
}

func (s *PorticoSession) SetFederationHandle(fedHandle int32) {
	s.fedHandle = fedHandle
}

func (s *PorticoSession) Protocol() ProtocolType {
	return ProtocolPortico
}

// SendPacket sends a packet using official 0xCAFE framing:
//   [int32: 0xCAFE] [int32: bundleLength] [Header: 12 bytes] [Payload: N bytes]
func (s *PorticoSession) SendPacket(hdr wire.Header, payload []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if s.isClosed {
		return net.ErrClosed
	}

	bundleLen := int32(wire.HeaderLength + len(payload))
	var prefix [8]byte
	binary.BigEndian.PutUint32(prefix[0:4], uint32(wire.MagicTCPBundle))
	binary.BigEndian.PutUint32(prefix[4:8], uint32(bundleLen))

	if _, err := s.writer.Write(prefix[:]); err != nil {
		return err
	}

	var hdrBuf [wire.HeaderLength]byte
	hdr.PayloadLength = uint32(len(payload))
	if err := hdr.MarshalTo(hdrBuf[:]); err != nil {
		return err
	}

	if _, err := s.writer.Write(hdrBuf[:]); err != nil {
		return err
	}

	if len(payload) > 0 {
		if _, err := s.writer.Write(payload); err != nil {
			return err
		}
	}

	return s.writer.Flush()
}

// SendControlResponseOK sends a successful control response for a received request
func (s *PorticoSession) SendControlResponseOK(reqID uint16, msgType wire.MessageType, payload []byte) error {
	target := uint16(s.id)
	if msgType == wire.MsgJoinFederation {
		target = 0xFFFF
	}
	hdr := wire.Header{
		Flags:         0,
		PayloadLength: uint32(len(payload)),
		FederationID:  uint8(s.fedHandle),
		CallType:      wire.CallTypeControlResponseOK,
		MessageType:   msgType,
		RequestId:     reqID,
		SourceHandle:  0,
		TargetHandle:  target,
	}
	return s.SendPacket(hdr, payload)
}

// SendControlResponseErr sends an error control response for a received request
func (s *PorticoSession) SendControlResponseErr(reqID uint16, msgType wire.MessageType, errMsg string) error {
	payload := wire.BuildErrorResponsePayload(errMsg)
	hdr := wire.Header{
		Flags:         0,
		PayloadLength: uint32(len(payload)),
		FederationID:  uint8(s.fedHandle),
		CallType:      wire.CallTypeControlResponseErr,
		MessageType:   msgType,
		RequestId:     reqID,
		SourceHandle:  0,
		TargetHandle:  uint16(s.id),
	}
	return s.SendPacket(hdr, payload)
}

// SendNotification sends an asynchronous notification (e.g., AnnounceSyncPoint, FederationSynchronized, TAG)
func (s *PorticoSession) SendNotification(msgType uint8, payload []byte) error {
	hdr := wire.Header{
		Flags:         0,
		PayloadLength: uint32(len(payload)),
		FederationID:  uint8(s.fedHandle),
		CallType:      wire.CallTypeNotification,
		MessageType:   wire.MessageType(msgType),
		RequestId:     0,
		SourceHandle:  0,
		TargetHandle:  uint16(s.id),
	}
	return s.SendPacket(hdr, payload)
}

// SendNotificationWithMerkle sends notification (Portico ignores Merkle roots)
func (s *PorticoSession) SendNotificationWithMerkle(msgType uint8, payload []byte, merkleRoot [32]byte) error {
	return s.SendNotification(msgType, payload)
}

// Close terminates the session and underlying socket
func (s *PorticoSession) Close() error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	s.isClosed = true
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

// ReadPacket reads the next packet framed with 0xCAFE from the connection
func (s *PorticoSession) ReadPacket(reader io.Reader) (wire.Header, []byte, error) {
	var hdr wire.Header

	for {
		// 1. Read bundle magic (0xCAFE)
		var magic int32
		if err := binary.Read(reader, binary.BigEndian, &magic); err != nil {
			return hdr, nil, err
		}
		if magic != wire.MagicTCPBundle {
			return hdr, nil, fmt.Errorf("invalid bundle magic: 0x%X", magic)
		}

		// 2. Read bundle length
		var bundleLen int32
		if err := binary.Read(reader, binary.BigEndian, &bundleLen); err != nil {
			return hdr, nil, err
		}
		if bundleLen == 0 {
			// Empty keepalive
			continue
		}
		if bundleLen < wire.HeaderLength {
			return hdr, nil, fmt.Errorf("bundle length smaller than 12-byte header: %d", bundleLen)
		}
		const maxPorticoBundleLen = 64 * 1024 * 1024
		if bundleLen > maxPorticoBundleLen {
			return hdr, nil, fmt.Errorf("bundleLen exceeds maximum safety limit: %d", bundleLen)
		}

		// 3. Read 12-byte header
		var hdrBuf [wire.HeaderLength]byte
		if _, err := io.ReadFull(reader, hdrBuf[:]); err != nil {
			return hdr, nil, err
		}
		if err := hdr.UnmarshalBinary(hdrBuf[:]); err != nil {
			return hdr, nil, err
		}

		// 4. Read payload
		payloadLen := int(bundleLen) - wire.HeaderLength
		var payload []byte
		if payloadLen > 0 {
			payload = make([]byte, payloadLen)
			if _, err := io.ReadFull(reader, payload); err != nil {
				return hdr, nil, err
			}
		}

		return hdr, payload, nil
	}
}
