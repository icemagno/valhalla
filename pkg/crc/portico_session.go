package crc

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"

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
	return atomic.LoadInt32(&s.id)
}

func (s *PorticoSession) SetFederateID(id int32) {
	atomic.StoreInt32(&s.id, id)
}

func (s *PorticoSession) SetFederationHandle(fedHandle int32) {
	atomic.StoreInt32(&s.fedHandle, fedHandle)
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
		if err := disc.UnmarshalBinary(payload); err != nil {
			return fmt.Errorf("portico session: unmarshal DiscoverObject failed: %w", err)
		}
		p, err := wire.BuildDiscoverObjectPayload(int32(disc.ObjectHandle), int32(disc.ClassHandle), disc.ObjectName)
		if err != nil {
			return fmt.Errorf("portico session: build DiscoverObject payload failed: %w", err)
		}
		wireMsgType = wire.MsgDiscoverObject
		wirePayload = p

	case common.NativeMsgRemoveObjectInstance:
		var rem common.RemoveObjectInstancePayload
		if err := rem.UnmarshalBinary(payload); err != nil {
			return fmt.Errorf("portico session: unmarshal RemoveObjectInstance failed: %w", err)
		}
		p, err := wire.BuildDeleteObjectPayload(int32(rem.ObjectHandle), rem.Tag)
		if err != nil {
			return fmt.Errorf("portico session: build DeleteObject payload failed: %w", err)
		}
		wireMsgType = wire.MsgDeleteObject
		wirePayload = p

	case common.NativeMsgReflectAttributes:
		var uav common.UpdateAttributesPayload
		if err := uav.UnmarshalBinary(payload); err != nil {
			return fmt.Errorf("portico session: unmarshal ReflectAttributes failed: %w", err)
		}
		attrsMap := make(map[int32][]byte, len(uav.Attributes))
		for k, v := range uav.Attributes {
			attrsMap[int32(k)] = v
		}
		p, err := wire.BuildReflectAttributesPayload(int32(uav.ObjectHandle), attrsMap, uav.Tag)
		if err != nil {
			return fmt.Errorf("portico session: build ReflectAttributes payload failed: %w", err)
		}
		wireMsgType = wire.MsgReflectAttributes
		wirePayload = p

	case common.NativeMsgReceiveInteraction:
		var inter common.SendInteractionPayload
		if err := inter.UnmarshalBinary(payload); err != nil {
			return fmt.Errorf("portico session: unmarshal ReceiveInteraction failed: %w", err)
		}
		paramsMap := make(map[int32][]byte, len(inter.Parameters))
		for k, v := range inter.Parameters {
			paramsMap[int32(k)] = v
		}
		p, err := wire.BuildReceiveInteractionPayload(int32(inter.ClassHandle), paramsMap, inter.Tag)
		if err != nil {
			return fmt.Errorf("portico session: build ReceiveInteraction payload failed: %w", err)
		}
		wireMsgType = wire.MsgReceiveInteraction
		wirePayload = p

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
		p, err := wire.BuildAnnounceSyncPointPayload(label)
		if err != nil {
			return fmt.Errorf("portico session: build AnnounceSyncPoint payload failed: %w", err)
		}
		wireMsgType = wire.MsgAnnounceSyncPoint
		wirePayload = p

	case common.NativeMsgFederationSynchronized:
		var label string
		var sp common.SyncPointLabelPayload
		if err := sp.UnmarshalBinary(payload); err == nil && sp.Label != "" {
			label = sp.Label
		} else {
			label = string(payload)
		}
		p, err := wire.BuildFederationSynchronizedPayload(label)
		if err != nil {
			return fmt.Errorf("portico session: build FederationSynchronized payload failed: %w", err)
		}
		wireMsgType = wire.MsgFederationSynchronized
		wirePayload = p

	case common.NativeMsgTimeAdvanceGrant:
		var t common.TimeFloat64Payload
		if err := t.UnmarshalBinary(payload); err != nil {
			return fmt.Errorf("portico session: unmarshal TimeAdvanceGrant failed: %w", err)
		}
		p, err := wire.BuildTimeAdvanceGrantPayload(atomic.LoadInt32(&s.id), t.Time)
		if err != nil {
			return fmt.Errorf("portico session: build TimeAdvanceGrant payload failed: %w", err)
		}
		wireMsgType = wire.MsgTimeAdvanceGrant
		wirePayload = p
	}

	return s.sendWireNotification(wireMsgType, wirePayload)
}

func (s *PorticoSession) sendWireNotification(wireMsgType wire.MessageType, wirePayload []byte) error {
	hdr := wire.Header{
		Flags:         0,
		PayloadLength: uint32(len(wirePayload)),
		FederationID:  uint8(atomic.LoadInt32(&s.fedHandle)),
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

// DomainEventSink implementation (single-pass Portico JOS serialization, zero SSZ round-trip)

func (s *PorticoSession) SendDiscoverObject(objHandle, classHandle int32, objName string) error {
	p, err := wire.BuildDiscoverObjectPayload(objHandle, classHandle, objName)
	if err != nil {
		return fmt.Errorf("portico session: build DiscoverObject payload failed: %w", err)
	}
	return s.sendWireNotification(wire.MsgDiscoverObject, p)
}

func (s *PorticoSession) SendRemoveObject(objHandle int32, tag []byte) error {
	p, err := wire.BuildDeleteObjectPayload(objHandle, tag)
	if err != nil {
		return fmt.Errorf("portico session: build DeleteObject payload failed: %w", err)
	}
	return s.sendWireNotification(wire.MsgDeleteObject, p)
}

func (s *PorticoSession) SendReflectAttributes(objHandle int32, attrs map[common.AttributeHandle][]byte, tag []byte, _ bool, _ float64, _ [32]byte) error {
	attrsMap := make(map[int32][]byte, len(attrs))
	for k, v := range attrs {
		attrsMap[int32(k)] = v
	}
	p, err := wire.BuildReflectAttributesPayload(objHandle, attrsMap, tag)
	if err != nil {
		return fmt.Errorf("portico session: build ReflectAttributes payload failed: %w", err)
	}
	return s.sendWireNotification(wire.MsgReflectAttributes, p)
}

func (s *PorticoSession) SendReceiveInteraction(classHandle int32, params map[common.ParameterHandle][]byte, tag []byte, _ bool, _ float64) error {
	paramsMap := make(map[int32][]byte, len(params))
	for k, v := range params {
		paramsMap[int32(k)] = v
	}
	p, err := wire.BuildReceiveInteractionPayload(classHandle, paramsMap, tag)
	if err != nil {
		return fmt.Errorf("portico session: build ReceiveInteraction payload failed: %w", err)
	}
	return s.sendWireNotification(wire.MsgReceiveInteraction, p)
}

func (s *PorticoSession) SendTimeAdvanceGrant(grantedTime float64) error {
	p, err := wire.BuildTimeAdvanceGrantPayload(atomic.LoadInt32(&s.id), grantedTime)
	if err != nil {
		return fmt.Errorf("portico session: build TimeAdvanceGrant payload failed: %w", err)
	}
	return s.sendWireNotification(wire.MsgTimeAdvanceGrant, p)
}

func (s *PorticoSession) SendAnnounceSyncPoint(label string, _ []byte) error {
	p, err := wire.BuildAnnounceSyncPointPayload(label)
	if err != nil {
		return fmt.Errorf("portico session: build AnnounceSyncPoint payload failed: %w", err)
	}
	return s.sendWireNotification(wire.MsgAnnounceSyncPoint, p)
}

func (s *PorticoSession) SendFederationSynchronized(label string) error {
	p, err := wire.BuildFederationSynchronizedPayload(label)
	if err != nil {
		return fmt.Errorf("portico session: build FederationSynchronized payload failed: %w", err)
	}
	return s.sendWireNotification(wire.MsgFederationSynchronized, p)
}
