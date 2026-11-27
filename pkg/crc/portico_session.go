package crc

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/icemagno/valhalla/pkg/common"
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

// SendNotification sends an asynchronous notification (e.g., AnnounceSyncPoint, FederationSynchronized, TAG).
// If msgType is a canonical common.NativeMsg*, it translates the payload and wire code to Portico's
// official binary serialization (JOS) and wire MessageType.
func (s *PorticoSession) SendNotification(msgType uint8, payload []byte) error {
	wireMsgType := wire.MessageType(msgType)
	wirePayload := payload

	switch msgType {
	case common.NativeMsgDiscoverObject:
		var disc common.DiscoverObjectInstancePayload
		if err := disc.UnmarshalBinary(payload); err == nil {
			if p, err := wire.BuildDiscoverObjectPayload(int32(disc.ObjectHandle), int32(disc.ClassHandle), disc.ObjectName); err == nil {
				wireMsgType = wire.MsgDiscoverObject
				wirePayload = p
			}
		}

	case common.NativeMsgRemoveObjectInstance:
		var rem common.RemoveObjectInstancePayload
		if err := rem.UnmarshalBinary(payload); err == nil {
			if p, err := wire.BuildDeleteObjectPayload(int32(rem.ObjectHandle), rem.Tag); err == nil {
				wireMsgType = wire.MsgDeleteObject
				wirePayload = p
			}
		}

	case common.NativeMsgReflectAttributes:
		var uav common.UpdateAttributesPayload
		if err := uav.UnmarshalBinary(payload); err == nil {
			attrsMap := make(map[int32][]byte, len(uav.Attributes))
			for k, v := range uav.Attributes {
				attrsMap[int32(k)] = v
			}
			if p, err := wire.BuildReflectAttributesPayload(int32(uav.ObjectHandle), attrsMap, uav.Tag); err == nil {
				wireMsgType = wire.MsgReflectAttributes
				wirePayload = p
			}
		}

	case common.NativeMsgReceiveInteraction:
		var inter common.SendInteractionPayload
		if err := inter.UnmarshalBinary(payload); err == nil {
			paramsMap := make(map[int32][]byte, len(inter.Parameters))
			for k, v := range inter.Parameters {
				paramsMap[int32(k)] = v
			}
			if p, err := wire.BuildReceiveInteractionPayload(int32(inter.ClassHandle), paramsMap, inter.Tag); err == nil {
				wireMsgType = wire.MsgReceiveInteraction
				wirePayload = p
			}
		}

	case common.NativeMsgAnnounceSyncPoint:
		var label string
		var reg common.RegisterSyncPointPayload
		if err := reg.UnmarshalBinary(payload); err == nil && reg.Label != "" {
			label = reg.Label
		} else {
			var sp common.SyncPointLabelPayload
			if err := sp.UnmarshalBinary(payload); err == nil {
				label = sp.Label
			} else {
				label = string(payload)
			}
		}
		if p, err := wire.BuildAnnounceSyncPointPayload(label); err == nil {
			wireMsgType = wire.MsgAnnounceSyncPoint
			wirePayload = p
		}

	case common.NativeMsgFederationSynchronized:
		var label string
		var sp common.SyncPointLabelPayload
		if err := sp.UnmarshalBinary(payload); err == nil && sp.Label != "" {
			label = sp.Label
		} else {
			label = string(payload)
		}
		if p, err := wire.BuildFederationSynchronizedPayload(label); err == nil {
			wireMsgType = wire.MsgFederationSynchronized
			wirePayload = p
		}

	case common.NativeMsgTimeAdvanceGrant:
		var t common.TimeFloat64Payload
		if err := t.UnmarshalBinary(payload); err == nil {
			if p, err := wire.BuildTimeAdvanceGrantPayload(s.id, t.Time); err == nil {
				wireMsgType = wire.MsgTimeAdvanceGrant
				wirePayload = p
			}
		}
	}

	hdr := wire.Header{
		Flags:         0,
		PayloadLength: uint32(len(wirePayload)),
		FederationID:  uint8(s.fedHandle),
		CallType:      wire.CallTypeNotification,
		MessageType:   wireMsgType,
		RequestId:     0,
		SourceHandle:  0,
		TargetHandle:  uint16(s.id),
	}
	return s.SendPacket(hdr, wirePayload)
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
