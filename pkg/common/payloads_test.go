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

	t.Run("DeleteObjectInstancePayload", func(t *testing.T) {
		orig := DeleteObjectInstancePayload{
			ObjectHandle: 777,
			Tag:          []byte("DeleteOrder_Confirmed"),
		}
		data, err := orig.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary failed: %v", err)
		}
		var decoded DeleteObjectInstancePayload
		if err := decoded.UnmarshalBinary(data); err != nil {
			t.Fatalf("UnmarshalBinary failed: %v", err)
		}
		if !reflect.DeepEqual(orig, decoded) {
			t.Errorf("Roundtrip mismatch: got %+v, want %+v", decoded, orig)
		}
	})

	t.Run("RemoveObjectInstancePayload", func(t *testing.T) {
		orig := RemoveObjectInstancePayload{
			ObjectHandle: 777,
			Tag:          []byte("Entity_Removed_Echo"),
		}
		data, err := orig.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary failed: %v", err)
		}
		var decoded RemoveObjectInstancePayload
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

	t.Run("OwnershipPayloads", func(t *testing.T) {
		divest := DivestOwnershipPayload{
			ObjectHandle: 777,
			Attributes:   []uint32{1, 2, 3},
		}
		dData, err := divest.MarshalBinary()
		if err != nil {
			t.Fatalf("Divest Marshal failed: %v", err)
		}
		var dDecoded DivestOwnershipPayload
		if err := dDecoded.UnmarshalBinary(dData); err != nil {
			t.Fatalf("Divest Unmarshal failed: %v", err)
		}
		if !reflect.DeepEqual(divest, dDecoded) {
			t.Errorf("Divest mismatch: got %+v, want %+v", dDecoded, divest)
		}

		acquire := AcquireOwnershipPayload{
			ObjectHandle: 777,
			Attributes:   []uint32{1, 2, 3},
		}
		aData, err := acquire.MarshalBinary()
		if err != nil {
			t.Fatalf("Acquire Marshal failed: %v", err)
		}
		var aDecoded AcquireOwnershipPayload
		if err := aDecoded.UnmarshalBinary(aData); err != nil {
			t.Fatalf("Acquire Unmarshal failed: %v", err)
		}
		if !reflect.DeepEqual(acquire, aDecoded) {
			t.Errorf("Acquire mismatch: got %+v, want %+v", aDecoded, acquire)
		}

		query := QueryOwnershipPayload{
			ObjectHandle:    777,
			AttributeHandle: 2,
		}
		qData, err := query.MarshalBinary()
		if err != nil {
			t.Fatalf("Query Marshal failed: %v", err)
		}
		var qDecoded QueryOwnershipPayload
		if err := qDecoded.UnmarshalBinary(qData); err != nil {
			t.Fatalf("Query Unmarshal failed: %v", err)
		}
		if !reflect.DeepEqual(query, qDecoded) {
			t.Errorf("Query mismatch: got %+v, want %+v", qDecoded, query)
		}

		qResp := QueryOwnershipResponsePayload{
			ObjectHandle:    777,
			AttributeHandle: 2,
			OwnerHandle:     42,
			IsOwned:         true,
		}
		qrData, err := qResp.MarshalBinary()
		if err != nil {
			t.Fatalf("QueryResp Marshal failed: %v", err)
		}
		var qrDecoded QueryOwnershipResponsePayload
		if err := qrDecoded.UnmarshalBinary(qrData); err != nil {
			t.Fatalf("QueryResp Unmarshal failed: %v", err)
		}
		if !reflect.DeepEqual(qResp, qrDecoded) {
			t.Errorf("QueryResp mismatch: got %+v, want %+v", qrDecoded, qResp)
		}
	})

	t.Run("DDMPayloads", func(t *testing.T) {
		cReg := CreateRegionPayload{
			Dimensions: []uint32{1, 2, 3},
		}
		crData, err := cReg.MarshalBinary()
		if err != nil {
			t.Fatalf("CreateRegion Marshal failed: %v", err)
		}
		var crDecoded CreateRegionPayload
		if err := crDecoded.UnmarshalBinary(crData); err != nil {
			t.Fatalf("CreateRegion Unmarshal failed: %v", err)
		}
		if !reflect.DeepEqual(cReg, crDecoded) {
			t.Errorf("CreateRegion mismatch: got %+v, want %+v", crDecoded, cReg)
		}

		crResp := CreateRegionResponsePayload{
			RegionHandle: 99,
		}
		crrData, err := crResp.MarshalBinary()
		if err != nil {
			t.Fatalf("CreateRegionResp Marshal failed: %v", err)
		}
		var crrDecoded CreateRegionResponsePayload
		if err := crrDecoded.UnmarshalBinary(crrData); err != nil {
			t.Fatalf("CreateRegionResp Unmarshal failed: %v", err)
		}
		if !reflect.DeepEqual(crResp, crrDecoded) {
			t.Errorf("CreateRegionResp mismatch: got %+v, want %+v", crrDecoded, crResp)
		}

		sBounds := SetRangeBoundsPayload{
			RegionHandle:    99,
			DimensionHandle: 1,
			LowerBound:      1000,
			UpperBound:      5000,
		}
		sbData, err := sBounds.MarshalBinary()
		if err != nil {
			t.Fatalf("SetRangeBounds Marshal failed: %v", err)
		}
		var sbDecoded SetRangeBoundsPayload
		if err := sbDecoded.UnmarshalBinary(sbData); err != nil {
			t.Fatalf("SetRangeBounds Unmarshal failed: %v", err)
		}
		if !reflect.DeepEqual(sBounds, sbDecoded) {
			t.Errorf("SetRangeBounds mismatch: got %+v, want %+v", sbDecoded, sBounds)
		}

		mBounds := SetMetricBoundsPayload{
			RegionHandle: 99,
			MinX:         -1000.5,
			MaxX:         1000.5,
			MinY:         -2000.25,
			MaxY:         2000.25,
			MinZ:         0.0,
			MaxZ:         500.0,
		}
		mbData, err := mBounds.MarshalBinary()
		if err != nil {
			t.Fatalf("SetMetricBounds Marshal failed: %v", err)
		}
		var mbDecoded SetMetricBoundsPayload
		if err := mbDecoded.UnmarshalBinary(mbData); err != nil {
			t.Fatalf("SetMetricBounds Unmarshal failed: %v", err)
		}
		if !reflect.DeepEqual(mBounds, mbDecoded) {
			t.Errorf("SetMetricBounds mismatch: got %+v, want %+v", mbDecoded, mBounds)
		}

		subReg := SubscribeWithRegionPayload{
			ObjectClass:  200,
			Attributes:   []uint32{10, 11, 12},
			RegionHandle: 99,
		}
		srData, err := subReg.MarshalBinary()
		if err != nil {
			t.Fatalf("SubWithRegion Marshal failed: %v", err)
		}
		var srDecoded SubscribeWithRegionPayload
		if err := srDecoded.UnmarshalBinary(srData); err != nil {
			t.Fatalf("SubWithRegion Unmarshal failed: %v", err)
		}
		if !reflect.DeepEqual(subReg, srDecoded) {
			t.Errorf("SubWithRegion mismatch: got %+v, want %+v", srDecoded, subReg)
		}

		spatialUp := UpdateAttributesSpatialPayload{
			ObjectHandle: 777,
			Attributes: map[AttributeHandle][]byte{
				1: []byte("val1"),
				2: []byte("val2"),
			},
			Tag: []byte("spatial-tag"),
			X:   12345.67,
			Y:   -9876.54,
			Z:   321.0,
		}
		suData, err := spatialUp.MarshalBinary()
		if err != nil {
			t.Fatalf("UpdateSpatial Marshal failed: %v", err)
		}
		var suDecoded UpdateAttributesSpatialPayload
		if err := suDecoded.UnmarshalBinary(suData); err != nil {
			t.Fatalf("UpdateSpatial Unmarshal failed: %v", err)
		}
		if suDecoded.ObjectHandle != spatialUp.ObjectHandle ||
			math.Abs(suDecoded.X-spatialUp.X) > 1e-5 ||
			math.Abs(suDecoded.Y-spatialUp.Y) > 1e-5 ||
			math.Abs(suDecoded.Z-spatialUp.Z) > 1e-5 ||
			!bytes.Equal(suDecoded.Tag, spatialUp.Tag) {
			t.Errorf("UpdateSpatial mismatch: got %+v, want %+v", suDecoded, spatialUp)
		}
		for k, v := range spatialUp.Attributes {
			if !bytes.Equal(suDecoded.Attributes[k], v) {
				t.Errorf("Attribute %d mismatch: got %v, want %v", k, suDecoded.Attributes[k], v)
			}
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
