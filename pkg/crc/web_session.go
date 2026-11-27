// web_session.go - RFC 6455 WebSocket Session for Valhalla HLA-4 Web Profile
package crc

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"strconv"
	"sync"

	"github.com/icemagno/valhalla/pkg/common"
)

// WebSession represents an active WebSocket client session in the Valhalla CRC
type WebSession struct {
	mu         sync.Mutex
	conn       net.Conn
	federateID int32
	closed     bool
}

// NewWebSession creates a new WebSession wrapping a hijacked net.Conn
func NewWebSession(conn net.Conn) *WebSession {
	return &WebSession{
		conn: conn,
	}
}

// FederateID returns the assigned federate handle ID
func (s *WebSession) FederateID() int32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.federateID
}

// SetFederateID assigns the federate ID once joined
func (s *WebSession) SetFederateID(id int32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.federateID = id
}

// Protocol returns ProtocolWeb (HLA-4 Web Profile)
func (s *WebSession) Protocol() ProtocolType {
	return ProtocolWeb
}

// SendNotification formats standard HLA callbacks and dispatches them over WebSocket as JSON
func (s *WebSession) SendNotification(msgType uint8, payload []byte) error {
	return s.SendNotificationWithMerkle(msgType, payload, [32]byte{})
}

// SendNotificationWithMerkle formats standard HLA callbacks with Merkle root over WebSocket
func (s *WebSession) SendNotificationWithMerkle(msgType uint8, payload []byte, merkleRoot [32]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed || s.conn == nil {
		return io.ErrClosedPipe
	}

	var notif map[string]interface{}

	switch msgType {
	case common.NativeMsgDiscoverObject:
		var disc common.DiscoverObjectInstancePayload
		if err := disc.UnmarshalBinary(payload); err != nil {
			return err
		}
		notif = map[string]interface{}{
			"callback": "discoverObjectInstance",
			"params": map[string]interface{}{
				"object": int32(disc.ObjectHandle),
				"class":  int32(disc.ClassHandle),
				"name":   disc.ObjectName,
			},
		}

	case common.NativeMsgReflectAttributes:
		var uav common.UpdateAttributesPayload
		if err := uav.UnmarshalBinary(payload); err != nil {
			return err
		}
		strAttrs := make(map[string]string)
		for k, v := range uav.Attributes {
			strAttrs[strconv.Itoa(int(k))] = string(v)
		}
		notif = map[string]interface{}{
			"callback": "reflectAttributeValues",
			"params": map[string]interface{}{
				"object":     int32(uav.ObjectHandle),
				"attributes": strAttrs,
				"tag":        string(uav.Tag),
			},
		}

	case common.NativeMsgRemoveObjectInstance:
		var rem common.RemoveObjectInstancePayload
		if err := rem.UnmarshalBinary(payload); err != nil {
			return err
		}
		notif = map[string]interface{}{
			"callback": "removeObjectInstance",
			"params": map[string]interface{}{
				"object": int32(rem.ObjectHandle),
				"tag":    string(rem.Tag),
			},
		}

	case common.NativeMsgReceiveInteraction:
		var inter common.SendInteractionPayload
		if err := inter.UnmarshalBinary(payload); err != nil {
			return err
		}
		strParams := make(map[string]string)
		for k, v := range inter.Parameters {
			strParams[strconv.Itoa(int(k))] = string(v)
		}
		notif = map[string]interface{}{
			"callback": "receiveInteraction",
			"params": map[string]interface{}{
				"interactionClass": int32(inter.ClassHandle),
				"parameters":       strParams,
				"tag":              string(inter.Tag),
			},
		}

	case common.NativeMsgTimeAdvanceGrant:
		var t common.TimeFloat64Payload
		if err := t.UnmarshalBinary(payload); err != nil {
			return err
		}
		notif = map[string]interface{}{
			"callback": "timeAdvanceGrant",
			"params": map[string]interface{}{
				"time": t.Time,
			},
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
		notif = map[string]interface{}{
			"callback": "announceSynchronizationPoint",
			"params": map[string]interface{}{
				"label": label,
			},
		}

	case common.NativeMsgFederationSynchronized:
		var label string
		var sp common.SyncPointLabelPayload
		if err := sp.UnmarshalBinary(payload); err == nil && sp.Label != "" {
			label = sp.Label
		} else {
			label = string(payload)
		}
		notif = map[string]interface{}{
			"callback": "federationSynchronized",
			"params": map[string]interface{}{
				"label": label,
			},
		}

	default:
		return nil
	}

	data, err := json.Marshal(notif)
	if err != nil {
		return err
	}

	return writeWebSocketTextFrame(s.conn, data)
}

// SendJSON sends arbitrary JSON object to client
func (s *WebSession) SendJSON(v interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed || s.conn == nil {
		return io.ErrClosedPipe
	}

	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return writeWebSocketTextFrame(s.conn, data)
}

// Close closes the WebSocket connection
func (s *WebSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.conn != nil {
		_ = writeWebSocketCloseFrame(s.conn)
		return s.conn.Close()
	}
	return nil
}

// -----------------------------------------------------------------------------
// RFC 6455 WebSocket Framing & Handshake Helpers (Pure Go Standard Library)
// -----------------------------------------------------------------------------

func computeWebSocketAccept(key string) string {
	h := sha1.New()
	h.Write([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func writeWebSocketTextFrame(w io.Writer, payload []byte) error {
	n := len(payload)
	var header []byte
	if n < 126 {
		header = []byte{0x81, byte(n)}
	} else if n <= 65535 {
		header = []byte{0x81, 126, byte(n >> 8), byte(n)}
	} else {
		header = make([]byte, 10)
		header[0] = 0x81
		header[1] = 127
		binary.BigEndian.PutUint64(header[2:], uint64(n))
	}
	if _, err := w.Write(header); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

func writeWebSocketCloseFrame(w io.Writer) error {
	_, err := w.Write([]byte{0x88, 0x00})
	return err
}

func readWebSocketFrame(r io.Reader) (opcode byte, payload []byte, err error) {
	var hdr [2]byte
	if _, err = io.ReadFull(r, hdr[:]); err != nil {
		return 0, nil, err
	}

	opcode = hdr[0] & 0x0F
	masked := (hdr[1] & 0x80) != 0
	lenByte := hdr[1] & 0x7F

	var payloadLen uint64
	if lenByte < 126 {
		payloadLen = uint64(lenByte)
	} else if lenByte == 126 {
		var lBuf [2]byte
		if _, err = io.ReadFull(r, lBuf[:]); err != nil {
			return 0, nil, err
		}
		payloadLen = uint64(binary.BigEndian.Uint16(lBuf[:]))
	} else {
		var lBuf [8]byte
		if _, err = io.ReadFull(r, lBuf[:]); err != nil {
			return 0, nil, err
		}
		payloadLen = binary.BigEndian.Uint64(lBuf[:])
	}

	var mask [4]byte
	if masked {
		if _, err = io.ReadFull(r, mask[:]); err != nil {
			return 0, nil, err
		}
	}

	payload = make([]byte, payloadLen)
	if _, err = io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}

	if masked {
		for i := uint64(0); i < payloadLen; i++ {
			payload[i] ^= mask[i%4]
		}
	}

	return opcode, payload, nil
}
