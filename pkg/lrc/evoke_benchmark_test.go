package lrc

import (
	"testing"
	"time"
)

func BenchmarkEvokeCallback_ImmediateReturn(b *testing.B) {
	amb := &RTIambassadorImpl{
		client: &Client{
			callbacks:  make(chan callbackItem, b.N+10),
			closedChan: make(chan struct{}),
		},
	}

	// Pre-fill callbacks so EvokeCallback returns immediately without waiting
	for i := 0; i < b.N; i++ {
		amb.client.callbacks <- callbackItem{
			MsgType: 43,
			Payload: []byte("bench"),
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = amb.EvokeCallback(10 * time.Millisecond)
	}
}

func BenchmarkEvokeCallback_ZeroTimeout(b *testing.B) {
	amb := &RTIambassadorImpl{
		client: &Client{
			callbacks:  make(chan callbackItem, b.N+10),
			closedChan: make(chan struct{}),
		},
	}

	for i := 0; i < b.N; i++ {
		amb.client.callbacks <- callbackItem{
			MsgType: 43,
			Payload: []byte("bench"),
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = amb.EvokeCallback(0)
	}
}

func BenchmarkEvokeCallback_TimeoutExpiration(b *testing.B) {
	amb := &RTIambassadorImpl{
		client: &Client{
			callbacks:  make(chan callbackItem, 10),
			closedChan: make(chan struct{}),
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = amb.EvokeCallback(time.Nanosecond)
	}
}
