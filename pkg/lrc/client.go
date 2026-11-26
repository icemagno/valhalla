package lrc

import (
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

const (
	// DefaultMaxCallbackQueueCapacity is the maximum safety ceiling before saturation is declared
	DefaultMaxCallbackQueueCapacity = 262144 // 256K callbacks
	// DefaultInQueueBufferSize is the internal buffer for incoming network callbacks
	DefaultInQueueBufferSize        = 4096
	// DefaultOutQueueBufferSize is the channel buffer exposed to EvokeCallback
	DefaultOutQueueBufferSize       = 1024
)

type callbackItem struct {
	MsgType    uint8
	Payload    []byte
	MerkleRoot [32]byte
	HasMerkle  bool
}

// Client manages the transport connection and native protocol (VAL4) with the CRC
type Client struct {
	conn             net.Conn
	writeMu          sync.Mutex
	nextReqID        uint32
	pendingMu        sync.Mutex
	pending          map[uint16]chan nativeResponse

	// Elastic callback queue pipeline
	inQueue          chan callbackItem
	callbacks        chan callbackItem
	maxQueueCapacity int64
	queueLen         int64
	isSaturated      uint32
	droppedCount     uint64

	isClosed         int32
	closedChan       chan struct{}
}

// NewClient connects to the CRC over TCP and establishes the native VAL4 preamble
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
		conn:             conn,
		pending:          make(map[uint16]chan nativeResponse),
		inQueue:          make(chan callbackItem, DefaultInQueueBufferSize),
		callbacks:        make(chan callbackItem, DefaultOutQueueBufferSize),
		maxQueueCapacity: DefaultMaxCallbackQueueCapacity,
		closedChan:       make(chan struct{}),
	}

	go c.readLoop()
	go c.queueLoop()

	return c, nil
}

// Close shuts down the transport connection
func (c *Client) Close() error {
	if atomic.CompareAndSwapInt32(&c.isClosed, 0, 1) {
		close(c.closedChan)
		return c.conn.Close()
	}
	return nil
}

// IsClosed indicates whether the client was disconnected
func (c *Client) IsClosed() bool {
	return atomic.LoadInt32(&c.isClosed) == 1
}

// SendRequest transmits a synchronous message with timeout and waits for the response
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

	hdr.PayloadLen = uint32(len(payload))
	buf := common.WriteNativeHeader(hdr)

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

// EnqueueCallback injects an asynchronous callback item into the callback processing queue
func (c *Client) EnqueueCallback(item callbackItem) {
	if c.IsClosed() {
		return
	}
	select {
	case c.inQueue <- item:
	case <-c.closedChan:
	}
}

// SetMaxQueueCapacity dynamically adjusts the safety ceiling of the elastic callback queue
func (c *Client) SetMaxQueueCapacity(capacity int) {
	atomic.StoreInt64(&c.maxQueueCapacity, int64(capacity))
}

// MaxQueueCapacity returns the maximum allowed items in the elastic callback queue
func (c *Client) MaxQueueCapacity() int {
	return int(atomic.LoadInt64(&c.maxQueueCapacity))
}

// QueueLength returns the current number of queued callbacks awaiting processing
func (c *Client) QueueLength() int {
	return int(atomic.LoadInt64(&c.queueLen)) + len(c.inQueue)
}

// IsQueueSaturated indicates whether the callback queue reached its safety ceiling
func (c *Client) IsQueueSaturated() bool {
	return atomic.LoadUint32(&c.isSaturated) == 1
}

// DroppedCallbackCount returns the cumulative number of dropped callbacks due to saturation
func (c *Client) DroppedCallbackCount() uint64 {
	return atomic.LoadUint64(&c.droppedCount)
}

// ResetQueueSaturation clears the saturation indicator and dropped counter
func (c *Client) ResetQueueSaturation() {
	atomic.StoreUint32(&c.isSaturated, 0)
	atomic.StoreUint64(&c.droppedCount, 0)
}

// queueLoop manages the elastic buffer between the internal inQueue and the public callbacks channel
func (c *Client) queueLoop() {
	var buffer []callbackItem

	for {
		var outChan chan<- callbackItem
		var nextItem callbackItem
		if len(buffer) > 0 {
			outChan = c.callbacks
			nextItem = buffer[0]
		} else if c.IsClosed() {
			return
		}

		select {
		case item := <-c.inQueue:
			maxCap := atomic.LoadInt64(&c.maxQueueCapacity)
			currentTotal := int64(len(buffer) + len(c.callbacks))
			if maxCap > 0 && currentTotal >= maxCap {
				atomic.StoreUint32(&c.isSaturated, 1)
				atomic.AddUint64(&c.droppedCount, 1)
			} else {
				buffer = append(buffer, item)
				atomic.StoreInt64(&c.queueLen, int64(len(buffer)+len(c.callbacks)))
			}

		case outChan <- nextItem:
			buffer[0] = callbackItem{} // Release reference for GC
			buffer = buffer[1:]
			atomic.StoreInt64(&c.queueLen, int64(len(buffer)+len(c.callbacks)))
			if len(buffer) == 0 && cap(buffer) > 4096 {
				buffer = nil
			}

		case <-c.closedChan:
			// Client was closed: drain inQueue non-blockingly into buffer
		drainInQueue:
			for {
				select {
				case item := <-c.inQueue:
					maxCap := atomic.LoadInt64(&c.maxQueueCapacity)
					currentTotal := int64(len(buffer) + len(c.callbacks))
					if maxCap > 0 && currentTotal >= maxCap {
						atomic.StoreUint32(&c.isSaturated, 1)
						atomic.AddUint64(&c.droppedCount, 1)
					} else {
						buffer = append(buffer, item)
						atomic.StoreInt64(&c.queueLen, int64(len(buffer)+len(c.callbacks)))
					}
				default:
					break drainInQueue
				}
			}

			// Drain buffer into c.callbacks as long as consumer is reading
			for len(buffer) > 0 {
				select {
				case c.callbacks <- buffer[0]:
					buffer[0] = callbackItem{}
					buffer = buffer[1:]
					atomic.StoreInt64(&c.queueLen, int64(len(buffer)+len(c.callbacks)))
				case <-time.After(500 * time.Millisecond):
					// Consumer stopped reading or abandoned queue; terminate
					return
				}
			}
			return
		}
	}
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

		// If ReqID is set, deliver to the goroutine waiting on synchronous response
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

		// Otherwise, it is an asynchronous RTI notification dispatched to the elastic callback queue
		hasMerkle := (hdr.Flags & common.FlagNativeMerkle) != 0
		item := callbackItem{
			MsgType:    hdr.MsgType,
			Payload:    payload,
			MerkleRoot: hdr.MerkleRoot,
			HasMerkle:  hasMerkle,
		}
		select {
		case c.inQueue <- item:
		case <-c.closedChan:
			return
		}
	}
}
