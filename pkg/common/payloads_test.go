package common

import (
	"bytes"
	"math"
	"reflect"
	"testing"
)

func TestPayloads_RoundTrip(t *testing.T) {
	t.Run("FederationExecutionPayload", func(t *testing.T) {
		orig := FederationExecutionPayload{
			FederationName: "ValhallaWarfare",
		}
		data, err := orig.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary failed: %v", err)
		}
		var decoded FederationExecutionPayload
		if err := decoded.UnmarshalBinary(data); err != nil {
			t.Fatalf("UnmarshalBinary failed: %v", err)
		}
		if !reflect.DeepEqual(orig, decoded) {
			t.Errorf("Roundtrip mismatch: got %+v, want %+v", decoded, orig)
		}
	})

	t.Run("JoinFederationPayload", func(t *testing.T) {
		orig := JoinFederationPayload{
			FederateName:   "ThorValhalla",
			FederateType:   "native-hla4",
			FederationName: "NordicSimulation",
		}
		data, err := orig.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary failed: %v", err)
		}
		var decoded JoinFederationPayload
		if err := decoded.UnmarshalBinary(data); err != nil {
			t.Fatalf("UnmarshalBinary failed: %v", err)
		}
		if !reflect.DeepEqual(orig, decoded) {
			t.Errorf("Roundtrip mismatch: got %+v, want %+v", decoded, orig)
		}
	})

	t.Run("JoinFederationResponsePayload", func(t *testing.T) {
		orig := JoinFederationResponsePayload{
			FederateHandle:   1001,
			FederationHandle: 42,
		}
		data, err := orig.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary failed: %v", err)
		}
		var decoded JoinFederationResponsePayload
		if err := decoded.UnmarshalBinary(data); err != nil {
			t.Fatalf("UnmarshalBinary failed: %v", err)
		}
		if !reflect.DeepEqual(orig, decoded) {
			t.Errorf("Roundtrip mismatch: got %+v, want %+v", decoded, orig)
		}
	})

	t.Run("RegisterSyncPointPayload", func(t *testing.T) {
		orig := RegisterSyncPointPayload{
			Label: "ReadyToRun",
			Tag:   []byte{0xDE, 0xAD, 0xBE, 0xEF},
		}
		data, err := orig.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary failed: %v", err)
		}
		var decoded RegisterSyncPointPayload
		if err := decoded.UnmarshalBinary(data); err != nil {
			t.Fatalf("UnmarshalBinary failed: %v", err)
		}
		if !reflect.DeepEqual(orig, decoded) {
			t.Errorf("Roundtrip mismatch: got %+v, want %+v", decoded, orig)
		}
	})

	t.Run("PublishObjectClassPayload", func(t *testing.T) {
		orig := PublishObjectClassPayload{
			ClassHandle:      101,
			AttributeHandles: []AttributeHandle{1, 2, 5, 9, 12},
		}
		data, err := orig.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary failed: %v", err)
		}
		var decoded PublishObjectClassPayload
		if err := decoded.UnmarshalBinary(data); err != nil {
			t.Fatalf("UnmarshalBinary failed: %v", err)
		}
		if !reflect.DeepEqual(orig, decoded) {
			t.Errorf("Roundtrip mismatch: got %+v, want %+v", decoded, orig)
		}
	})

	t.Run("SubscribeObjectClassPayload", func(t *testing.T) {
		orig := SubscribeObjectClassPayload{
			ClassHandle:      202,
			AttributeHandles: []AttributeHandle{10, 20},
			Active:           true,
		}
		data, err := orig.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary failed: %v", err)
		}
		var decoded SubscribeObjectClassPayload
		if err := decoded.UnmarshalBinary(data); err != nil {
			t.Fatalf("UnmarshalBinary failed: %v", err)
		}
		if !reflect.DeepEqual(orig, decoded) {
			t.Errorf("Roundtrip mismatch: got %+v, want %+v", decoded, orig)
		}
	})

	t.Run("RegisterObjectInstancePayload", func(t *testing.T) {
		orig := RegisterObjectInstancePayload{
			ClassHandle: 50,
			ObjectName:  "Vessel_001",
		}
		data, err := orig.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary failed: %v", err)
		}
		var decoded RegisterObjectInstancePayload
		if err := decoded.UnmarshalBinary(data); err != nil {
			t.Fatalf("UnmarshalBinary failed: %v", err)
		}
		if !reflect.DeepEqual(orig, decoded) {
			t.Errorf("Roundtrip mismatch: got %+v, want %+v", decoded, orig)
		}
	})

	t.Run("DiscoverObjectInstancePayload", func(t *testing.T) {
		orig := DiscoverObjectInstancePayload{
			ObjectHandle: 777,
			ClassHandle:  50,
			ObjectName:   "Vessel_001",
		}
		data, err := orig.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary failed: %v", err)
		}
		var decoded DiscoverObjectInstancePayload
		if err := decoded.UnmarshalBinary(data); err != nil {
			t.Fatalf("UnmarshalBinary failed: %v", err)
		}
		if !reflect.DeepEqual(orig, decoded) {
			t.Errorf("Roundtrip mismatch: got %+v, want %+v", decoded, orig)
		}
	})

	t.Run("UpdateAttributesPayload_WithTime", func(t *testing.T) {
		orig := UpdateAttributesPayload{
			ObjectHandle: 888,
			Attributes: map[AttributeHandle][]byte{
				1: []byte("Position:X=10,Y=20"),
				2: []byte{0x00, 0x01, 0x02, 0x03},
			},
			Tag:     []byte("tag-val"),
			HasTime: true,
			Time:    123.456,
		}
		data, err := orig.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary failed: %v", err)
		}
		var decoded UpdateAttributesPayload
		if err := decoded.UnmarshalBinary(data); err != nil {
			t.Fatalf("UnmarshalBinary failed: %v", err)
		}
		if decoded.ObjectHandle != orig.ObjectHandle ||
			decoded.HasTime != orig.HasTime ||
			math.Abs(decoded.Time-orig.Time) > 1e-9 ||
			!bytes.Equal(decoded.Tag, orig.Tag) ||
			len(decoded.Attributes) != len(orig.Attributes) {
			t.Fatalf("Decoded attributes mismatch: got %+v, want %+v", decoded, orig)
		}
		for k, v := range orig.Attributes {
			if !bytes.Equal(decoded.Attributes[k], v) {
				t.Errorf("Attribute %d mismatch: got %v, want %v", k, decoded.Attributes[k], v)
			}
		}
	})

	t.Run("SendInteractionPayload", func(t *testing.T) {
		orig := SendInteractionPayload{
			ClassHandle: 999,
			Parameters: map[ParameterHandle][]byte{
				10: []byte("ExplosionPower=100"),
			},
			Tag:     []byte("interaction-tag"),
			HasTime: false,
		}
		data, err := orig.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary failed: %v", err)
		}
		var decoded SendInteractionPayload
		if err := decoded.UnmarshalBinary(data); err != nil {
			t.Fatalf("UnmarshalBinary failed: %v", err)
		}
		if decoded.ClassHandle != orig.ClassHandle ||
			decoded.HasTime != orig.HasTime ||
			!bytes.Equal(decoded.Tag, orig.Tag) ||
			len(decoded.Parameters) != len(orig.Parameters) {
			t.Fatalf("Decoded interaction mismatch: got %+v, want %+v", decoded, orig)
		}
		for k, v := range orig.Parameters {
			if !bytes.Equal(decoded.Parameters[k], v) {
				t.Errorf("Parameter %d mismatch: got %v, want %v", k, decoded.Parameters[k], v)
			}
		}
	})

	t.Run("TimeFloat64Payload", func(t *testing.T) {
		orig := TimeFloat64Payload{Time: 3.1415926535}
		data, err := orig.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary failed: %v", err)
		}
		var decoded TimeFloat64Payload
		if err := decoded.UnmarshalBinary(data); err != nil {
			t.Fatalf("UnmarshalBinary failed: %v", err)
		}
		if math.Abs(decoded.Time-orig.Time) > 1e-9 {
			t.Errorf("Time mismatch: got %f, want %f", decoded.Time, orig.Time)
		}
	})

	t.Run("ErrorResponsePayload", func(t *testing.T) {
		orig := ErrorResponsePayload{
			ErrorCode: 404,
			Message:   "ObjectInstanceNotFoundException",
		}
		data, err := orig.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary failed: %v", err)
		}
		var decoded ErrorResponsePayload
		if err := decoded.UnmarshalBinary(data); err != nil {
			t.Fatalf("UnmarshalBinary failed: %v", err)
		}
		if !reflect.DeepEqual(orig, decoded) {
			t.Errorf("Roundtrip mismatch: got %+v, want %+v", decoded, orig)
		}
	})
}

func BenchmarkPayload_UpdateAttributes(b *testing.B) {
	orig := UpdateAttributesPayload{
		ObjectHandle: 888,
		Attributes: map[AttributeHandle][]byte{
			1: []byte("X=100.5,Y=200.3,Z=10.0"),
			2: []byte("Heading=180.0,Speed=25.0"),
			3: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
		},
		Tag:     []byte("benchmark-tag"),
		HasTime: true,
		Time:    10.0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, err := orig.MarshalBinary()
		if err != nil {
			b.Fatal(err)
		}
		var decoded UpdateAttributesPayload
		if err := decoded.UnmarshalBinary(data); err != nil {
			b.Fatal(err)
		}
	}
}
