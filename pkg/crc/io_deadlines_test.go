package crc

import (
	"io"
	"net"
	"testing"
	"time"
)

func TestWebSession_SlowPeerWriteDeadline_DoesNotHang(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	sess := NewWebSession(c1)
	sess.writeTimeout = 100 * time.Millisecond

	start := time.Now()
	err := sess.SendJSON(map[string]string{"ping": "pong"})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatalf("expected write timeout error on unconsumed net.Pipe, got nil")
	}
	if elapsed > 1*time.Second {
		t.Fatalf("SendJSON took %v, expected <= 1s", elapsed)
	}

	closeStart := time.Now()
	_ = sess.Close()
	if time.Since(closeStart) > 1*time.Second {
		t.Fatalf("Close hung on unconsumed net.Pipe")
	}
}

func TestServer_DedicatedNative_SlowlorisTimeout(t *testing.T) {
	srv := NewServer("127.0.0.1:0")
	srv.SetNativeAddr("127.0.0.1:0")
	srv.SetPeekTimeout(150 * time.Millisecond)

	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start failed: %v", err)
	}
	defer srv.Stop()

	conn, err := net.DialTimeout("tcp", srv.NativeAddr().String(), time.Second)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	// Send zero bytes (Slowloris attack) and verify server closes socket promptly
	_ = conn.SetReadDeadline(time.Now().Add(1500 * time.Millisecond))
	var buf [1]byte
	_, err = conn.Read(buf[:])
	if err == nil {
		t.Fatalf("expected server to close idle Slowloris connection, but read succeeded")
	}
	if err != io.EOF {
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			t.Fatalf("server did NOT close idle Slowloris connection within 1.5s")
		}
	}
}
