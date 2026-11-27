package crc

import (
	"net"
	"sync"
	"sync/atomic"
	"time"

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
	return atomic.LoadInt32(&s.id)
}

func (s *NativeSession) SetFederateID(id int32) {
	atomic.StoreInt32(&s.id, id)
}

func (s *NativeSession) SetFederationID(fedID int32) {
	atomic.StoreInt32(&s.fedID, fedID)
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

	_ = s.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	defer func() { _ = s.conn.SetWriteDeadline(time.Time{}) }()

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
	return s.SendNotificationWithMerkle(msgType, payload, [32]byte{})
}

// SendNotificationWithMerkle sends an asynchronous notification with an optional 32-byte Merkle state root
func (s *NativeSession) SendNotificationWithMerkle(msgType uint8, payload []byte, merkleRoot [32]byte) error {
	flags := common.FlagNativeSSZ
	if merkleRoot != [32]byte{} {
		flags |= common.FlagNativeMerkle
	}
	hdr := NativeHeader{
		Magic:      NativeHeaderMagic,
		Version:    NativeProtocolVer,
		Flags:      flags,
		PayloadLen: uint32(len(payload)),
		MsgType:    msgType,
		FedID:      uint32(atomic.LoadInt32(&s.fedID)),
		FederateID: uint32(atomic.LoadInt32(&s.id)),
		MerkleRoot: merkleRoot,
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

// DomainEventSink implementation (single-pass SSZ serialization)

func (s *NativeSession) SendDiscoverObject(objHandle, classHandle int32, objName string) error {
	payload, err := common.DiscoverObjectInstancePayload{
		ObjectHandle: common.ObjectInstanceHandle(objHandle),
		ClassHandle:  common.ObjectClassHandle(classHandle),
		ObjectName:   objName,
	}.MarshalBinary()
	if err != nil {
		return err
	}
	return s.SendNotification(common.NativeMsgDiscoverObject, payload)
}

func (s *NativeSession) SendRemoveObject(objHandle int32, tag []byte) error {
	payload, err := common.RemoveObjectInstancePayload{
		ObjectHandle: common.ObjectInstanceHandle(objHandle),
		Tag:          tag,
	}.MarshalBinary()
	if err != nil {
		return err
	}
	return s.SendNotification(common.NativeMsgRemoveObjectInstance, payload)
}

func (s *NativeSession) SendReflectAttributes(objHandle int32, attrs map[common.AttributeHandle][]byte, tag []byte, hasTime bool, timestamp float64, merkleRoot [32]byte) error {
	payload, err := common.UpdateAttributesPayload{
		ObjectHandle: common.ObjectInstanceHandle(objHandle),
		Attributes:   attrs,
		Tag:          tag,
		HasTime:      hasTime,
		Time:         timestamp,
	}.MarshalBinary()
	if err != nil {
		return err
	}
	return s.SendNotificationWithMerkle(common.NativeMsgReflectAttributes, payload, merkleRoot)
}

func (s *NativeSession) SendReceiveInteraction(classHandle int32, params map[common.ParameterHandle][]byte, tag []byte, hasTime bool, timestamp float64) error {
	payload, err := common.SendInteractionPayload{
		ClassHandle: common.InteractionClassHandle(classHandle),
		Parameters:  params,
		Tag:         tag,
		HasTime:     hasTime,
		Time:        timestamp,
	}.MarshalBinary()
	if err != nil {
		return err
	}
	return s.SendNotification(common.NativeMsgReceiveInteraction, payload)
}

func (s *NativeSession) SendTimeAdvanceGrant(grantedTime float64) error {
	payload, err := common.TimeFloat64Payload{Time: grantedTime}.MarshalBinary()
	if err != nil {
		return err
	}
	return s.SendNotification(common.NativeMsgTimeAdvanceGrant, payload)
}

func (s *NativeSession) SendAnnounceSyncPoint(label string, tag []byte) error {
	payload, err := common.RegisterSyncPointPayload{Label: label, Tag: tag}.MarshalBinary()
	if err != nil {
		return err
	}
	return s.SendNotification(common.NativeMsgAnnounceSyncPoint, payload)
}

func (s *NativeSession) SendFederationSynchronized(label string) error {
	payload, err := common.SyncPointLabelPayload{Label: label}.MarshalBinary()
	if err != nil {
		return err
	}
	return s.SendNotification(common.NativeMsgFederationSynchronized, payload)
}
