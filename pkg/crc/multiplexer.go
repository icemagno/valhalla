package crc

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"time"
)

// MagicNativeVAL4 defines the 4-byte header magic that identifies a native Valhalla client
var MagicNativeVAL4 = []byte{'V', 'A', 'L', '4'}

// MultiplexedConn wraps the TCP connection and a buffered reader pre-fed by Peek
type MultiplexedConn struct {
	net.Conn
	Reader io.Reader
}

func (m *MultiplexedConn) Read(b []byte) (int, error) {
	return m.Reader.Read(b)
}

// DetectProtocol inspects the initial bytes received on the connection to infer the protocol dialect
func DetectProtocol(conn net.Conn, peekTimeout time.Duration) (ProtocolType, *MultiplexedConn, error) {
	br := bufio.NewReader(conn)

	// Set short read deadline to check if the client initiates actively
	_ = conn.SetReadDeadline(time.Now().Add(peekTimeout))
	peekBuf, err := br.Peek(4)
	_ = conn.SetReadDeadline(time.Time{}) // Reset deadline

	wrappedConn := &MultiplexedConn{
		Conn:   conn,
		Reader: br,
	}

	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			// Timeout: The client did not send bytes immediately.
			// This is Portico's "Server-First" signature (Thor waits for server's 0xBEEF handshake)
			return ProtocolPortico, wrappedConn, nil
		}
		if errors.Is(err, io.EOF) {
			return 0, wrappedConn, err
		}
		// If fewer than 4 bytes were read before timeout
		return ProtocolPortico, wrappedConn, nil
	}

	// Active client sent data immediately:
	if bytes.Equal(peekBuf, MagicNativeVAL4) {
		// Consume the 4 "VAL4" bytes from the buffer for the native pipeline
		_, _ = br.Discard(4)
		return ProtocolNative, wrappedConn, nil
	}

	// By default, if not VAL4, route to the Portico compatibility pipeline
	return ProtocolPortico, wrappedConn, nil
}
