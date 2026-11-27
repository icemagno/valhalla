package crc

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

// MagicNativeVAL4 defines the 4-byte header magic that identifies a native Valhalla client
var MagicNativeVAL4 = []byte{'V', 'A', 'L', '4'}

var (
	// ErrUnknownProtocol is returned when client sends data that does not match any supported protocol
	ErrUnknownProtocol = errors.New("multiplexer: unknown wire protocol preamble")

	// ErrIncompletePreamble is returned when client sends a partial preamble but fails to complete it
	ErrIncompletePreamble = errors.New("multiplexer: incomplete native protocol preamble")
)

// MultiplexedConn wraps the TCP connection and a buffered reader pre-fed by Peek
type MultiplexedConn struct {
	net.Conn
	Reader io.Reader
}

func (m *MultiplexedConn) Read(b []byte) (int, error) {
	return m.Reader.Read(b)
}

// DetectProtocol inspects the initial bytes received on the connection to infer the protocol dialect.
// Passive clients (e.g. Portico / Thor, which wait for the server's 0xBEEF greeting) send 0 bytes
// within peekTimeout and are identified as ProtocolPortico.
// Active clients sending the "VAL4" preamble are identified as ProtocolNative.
// Any active data not matching "VAL4" is rejected as ProtocolUnknown.
func DetectProtocol(conn net.Conn, peekTimeout time.Duration) (ProtocolType, *MultiplexedConn, error) {
	br := bufio.NewReader(conn)

	wrappedConn := &MultiplexedConn{
		Conn:   conn,
		Reader: br,
	}

	// Set short read deadline to check if the client initiates actively
	_ = conn.SetReadDeadline(time.Now().Add(peekTimeout))
	peekBuf, err := br.Peek(4)
	_ = conn.SetReadDeadline(time.Time{}) // Reset deadline

	if err == nil {
		// Exactly 4 bytes peeked without error
		if bytes.Equal(peekBuf, MagicNativeVAL4) {
			_, _ = br.Discard(4)
			return ProtocolNative, wrappedConn, nil
		}
		// Active client sent 4 bytes that are NOT "VAL4".
		// Portico clients are strictly passive upon connect. This is alien traffic.
		return ProtocolUnknown, wrappedConn, fmt.Errorf("%w: received %q", ErrUnknownProtocol, peekBuf)
	}

	// We got an error peeking 4 bytes (e.g. timeout, EOF, or buffer full)
	if errors.Is(err, io.EOF) {
		return ProtocolUnknown, wrappedConn, err
	}

	buffered := br.Buffered()
	if buffered == 0 {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			// Zero bytes received before timeout.
			// This is Portico's "Server-First" signature (Thor waits for server's 0xBEEF handshake)
			return ProtocolPortico, wrappedConn, nil
		}
		return ProtocolUnknown, wrappedConn, err
	}

	// Some bytes were buffered (< 4 bytes)
	partial, _ := br.Peek(buffered)
	if bytes.Equal(partial, MagicNativeVAL4[:buffered]) {
		// Partial "VAL4" prefix received! Client is a Valhalla Native client whose preamble was segmented.
		// Wait with a grace period for the remainder of the 4 bytes.
		graceTimeout := peekTimeout * 3
		if graceTimeout < 50*time.Millisecond {
			graceTimeout = 50 * time.Millisecond
		}
		_ = conn.SetReadDeadline(time.Now().Add(graceTimeout))
		fullBuf, fullErr := br.Peek(4)
		_ = conn.SetReadDeadline(time.Time{})

		if fullErr == nil && bytes.Equal(fullBuf, MagicNativeVAL4) {
			_, _ = br.Discard(4)
			return ProtocolNative, wrappedConn, nil
		}
		return ProtocolUnknown, wrappedConn, ErrIncompletePreamble
	}

	// Buffered bytes do NOT match the prefix of "VAL4" (e.g. "HT" from "HTTP").
	// Active non-Portico alien data.
	return ProtocolUnknown, wrappedConn, fmt.Errorf("%w: received partial %q", ErrUnknownProtocol, partial)
}
