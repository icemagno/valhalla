package lrc

import (
	"testing"
	"time"

	"github.com/icemagno/valhalla/pkg/common"
)

func BenchmarkEvokeCallback_ImmediateReturn(b *testing.B) {
	fedAmb := &countingTimeAmbassador{}
	amb := &RTIambassadorImpl{
		fedAmb: fedAmb,
		client: &Client{
			callbacks:  make(chan callbackItem, b.N+10),
			closedChan: make(chan struct{}),
		},
	}

	validPayload, err := common.TimeFloat64Payload{Time: 10.0}.MarshalBinary()
	if err != nil {
		b.Fatalf("MarshalBinary failed: %v", err)
	}

	// Pre-fill callbacks so EvokeCallback executes real SSZ unmarshal + FederateAmbassador dispatch
	for i := 0; i < b.N; i++ {
		amb.client.callbacks <- callbackItem{
			MsgType: common.NativeMsgTimeAdvanceGrant,
			Payload: validPayload,
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = amb.EvokeCallback(10 * time.Millisecond)
	}
}

func BenchmarkEvokeCallback_ZeroTimeout(b *testing.B) {
	fedAmb := &countingTimeAmbassador{}
	amb := &RTIambassadorImpl{
		fedAmb: fedAmb,
		client: &Client{
			callbacks:  make(chan callbackItem, b.N+10),
			closedChan: make(chan struct{}),
		},
	}

	validPayload, err := common.TimeFloat64Payload{Time: 10.0}.MarshalBinary()
	if err != nil {
		b.Fatalf("MarshalBinary failed: %v", err)
	}

	for i := 0; i < b.N; i++ {
		amb.client.callbacks <- callbackItem{
			MsgType: common.NativeMsgTimeAdvanceGrant,
			Payload: validPayload,
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
