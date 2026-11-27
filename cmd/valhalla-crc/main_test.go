package main

import (
	"os"
	"testing"
	"time"
)

func TestNormalizeListenAddr(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"52295", ":52295"},
		{":52295", ":52295"},
		{"127.0.0.1:0", "127.0.0.1:0"},
		{"0.0.0.0:8080", "0.0.0.0:8080"},
	}
	for _, tc := range cases {
		if got := normalizeListenAddr(tc.in); got != tc.want {
			t.Errorf("normalizeListenAddr(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRunCRC_StartAndGracefulStop(t *testing.T) {
	stopCh := make(chan os.Signal, 1)
	doneCh := make(chan error, 1)

	go func() {
		doneCh <- runCRC("127.0.0.1:0", "127.0.0.1:0", stopCh)
	}()

	time.Sleep(80 * time.Millisecond)
	stopCh <- os.Interrupt

	select {
	case err := <-doneCh:
		if err != nil {
			t.Fatalf("runCRC returned unexpected error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("runCRC did not shut down within 3s")
	}
}

func TestRunCRC_InvalidAddressReturnsError(t *testing.T) {
	stopCh := make(chan os.Signal, 1)
	if err := runCRC("invalid-host-name-:::99999", "", stopCh); err == nil {
		t.Fatal("expected error for invalid listen address")
	}
}
