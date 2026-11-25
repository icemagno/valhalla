package lrc

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/icemagno/valhalla/pkg/common"
)

type nativeResponse struct {
	Header  common.NativeHeader
	Payload []byte
	Err     error
}

type callbackItem struct {
	MsgType uint8
	Payload []byte
}

// Client gerencia a conexão de transporte e protocolo nativo (VAL4) com o CRC
type Client struct {
	conn       net.Conn
	writeMu    sync.Mutex
	nextReqID  uint32
	pendingMu  sync.Mutex
	pending    map[uint16]chan nativeResponse
	callbacks  chan callbackItem
	isClosed   int32
	closedChan chan struct{}
}

// NewClient conecta ao CRC via TCP e estabelece o preâmbulo nativo VAL4
func NewClient(address string) (*Client, error) {
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to crc at %s: %w", address, err)
	}

	// Transmit VAL4 native preamble ("VAL4")
	preamble := []byte{'V', 'A', 'L', '4'}
	if _, err := conn.Write(preamble); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to send VAL4 preamble: %w", err)
	}

	c := &Client{
		conn:       conn,
		pending:    make(map[uint16]chan nativeResponse),
		callbacks:  make(chan callbackItem, 1024),
		closedChan: make(chan struct{}),
	}

	go c.readLoop()

	return c, nil
}

// Close encerra a conexão de transporte
func (c *Client) Close() error {
	if atomic.CompareAndSwapInt32(&c.isClosed, 0, 1) {
		close(c.closedChan)
		return c.conn.Close()
	}
	return nil
}

// IsClosed indica se o cliente foi desconectado
func (c *Client) IsClosed() bool {
	return atomic.LoadInt32(&c.isClosed) == 1
}

// SendRequest transmite uma mensagem síncrona com timeout e aguarda a resposta
func (c *Client) SendRequest(msgType uint8, fedID, federateID uint32, payload []byte, timeout time.Duration) (nativeResponse, error) {
	if c.IsClosed() {
		return nativeResponse{}, ErrConnectionClosed
	}

	reqID := uint16(atomic.AddUint32(&c.nextReqID, 1))
	if reqID == 0 {
		reqID = uint16(atomic.AddUint32(&c.nextReqID, 1))
	}

	respChan := make(chan nativeResponse, 1)

	c.pendingMu.Lock()
	c.pending[reqID] = respChan
	c.pendingMu.Unlock()

	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, reqID)
		c.pendingMu.Unlock()
	}()

	hdr := common.NativeHeader{
		Magic:      common.NativeHeaderMagic,
		Version:    common.NativeProtocolVer,
		Flags:      0x01, // SSZ
		PayloadLen: uint32(len(payload)),
		MsgType:    msgType,
		ReqID:      reqID,
		FedID:      fedID,
		FederateID: federateID,
	}

	if err := c.writePacket(hdr, payload); err != nil {
		return nativeResponse{}, err
	}

	select {
	case resp := <-respChan:
		return resp, resp.Err
	case <-time.After(timeout):
		return nativeResponse{}, ErrRequestTimeout
	case <-c.closedChan:
		return nativeResponse{}, ErrConnectionClosed
	}
}

func (c *Client) writePacket(hdr common.NativeHeader, payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	var buf [common.NativeHeaderLength]byte
	binary.BigEndian.PutUint16(buf[0:2], hdr.Magic)
	buf[2] = hdr.Version
	buf[3] = hdr.Flags
	binary.BigEndian.PutUint32(buf[4:8], uint32(len(payload)))
	buf[8] = hdr.MsgType
	buf[9] = hdr.Reserved
	binary.BigEndian.PutUint16(buf[10:12], hdr.ReqID)
	binary.BigEndian.PutUint32(buf[12:16], hdr.FedID)
	binary.BigEndian.PutUint32(buf[16:20], hdr.FederateID)
	binary.BigEndian.PutUint32(buf[20:24], 0)
	copy(buf[24:56], hdr.MerkleRoot[:])

	if _, err := c.conn.Write(buf[:]); err != nil {
		return err
	}
	if len(payload) > 0 {
		if _, err := c.conn.Write(payload); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) readLoop() {
	defer c.Close()

	for {
		hdr, err := common.ReadNativeHeader(c.conn)
		if err != nil {
			return
		}

		var payload []byte
		if hdr.PayloadLen > 0 {
			payload = make([]byte, hdr.PayloadLen)
			if _, err := io.ReadFull(c.conn, payload); err != nil {
				return
			}
		}

		// Se tem ReqID associado, entrega para a goroutine que fez a requisição síncrona
		if hdr.ReqID > 0 {
			c.pendingMu.Lock()
			ch, ok := c.pending[hdr.ReqID]
			c.pendingMu.Unlock()
			if ok {
				ch <- nativeResponse{
					Header:  hdr,
					Payload: payload,
				}
				continue
			}
		}

		// Caso contrário, é uma notificação assíncrona do RTI enviada para a fila de callbacks
		select {
		case c.callbacks <- callbackItem{MsgType: hdr.MsgType, Payload: payload}:
		default:
			// Fila saturada
		}
	}
}
