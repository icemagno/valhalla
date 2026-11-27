package metamodel_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/icemagno/valhalla/pkg/crc/portico/wire"
	"github.com/icemagno/valhalla/pkg/crc/portico/wire/metamodel"
)

func roundTrip(t *testing.T, obj *metamodel.JavaObject) *metamodel.JavaObject {
	t.Helper()
	raw, err := wire.SerializePorticoControlMessage(obj)
	if err != nil {
		t.Fatalf("SerializePorticoControlMessage(%s) failed: %v", obj.Class.Name, err)
	}
	if len(raw) < 6 {
		t.Fatalf("serialized %s too short: %d bytes", obj.Class.Name, len(raw))
	}
	dec, err := wire.DeserializePorticoControlMessage(raw)
	if err != nil {
		t.Fatalf("DeserializePorticoControlMessage(%s) failed: %v", obj.Class.Name, err)
	}
	if dec == nil || dec.Class == nil {
		t.Fatalf("deserialized %s returned nil object/class", obj.Class.Name)
	}
	return dec
}

func TestMetamodel_RealJOSRoundTrip(t *testing.T) {
	t.Run("CreateFederation", func(t *testing.T) {
		dec := roundTrip(t, metamodel.NewCreateFederation("TestFed"))
		if got := dec.Get("federationName"); got != "TestFed" {
			t.Fatalf("federationName = %v, want TestFed", got)
		}
	})

	t.Run("JoinFederation", func(t *testing.T) {
		jf := metamodel.NewJoinFederation("TestFed", "FedAlpha", "DroneType")
		dec := roundTrip(t, jf)
		if dec.Get("federationName") != "TestFed" || dec.Get("federateName") != "FedAlpha" || dec.Get("federateType") != "DroneType" {
			t.Fatalf("JoinFederation roundtrip mismatch: %+v", dec.Fields)
		}
		// Verify deterministic String() across repeated calls
		s := jf.String()
		for i := 0; i < 50; i++ {
			if jf.String() != s {
				t.Fatalf("JavaObject.String() non-deterministic")
			}
		}
		if !strings.Contains(s, "JoinFederation") {
			t.Fatalf("expected JoinFederation in String(), got %q", s)
		}
	})

	t.Run("ResignFederation", func(t *testing.T) {
		dec := roundTrip(t, metamodel.NewResignFederation("TestFed", "FedAlpha", "DroneType", "NO_ACTION"))
		if dec.Get("federateName") != "FedAlpha" || dec.Get("federationName") != "TestFed" {
			t.Fatalf("ResignFederation roundtrip mismatch: %+v", dec.Fields)
		}
	})

	t.Run("WelcomePack", func(t *testing.T) {
		raw, err := wire.SerializePorticoControlMessage(metamodel.NewWelcomePack(7, 42, "TestFed", "FedAlpha", "DroneType"))
		if err != nil {
			t.Fatalf("Serialize WelcomePack failed: %v", err)
		}
		wp, err := wire.ParseWelcomePack(raw)
		if err != nil {
			t.Fatalf("ParseWelcomePack failed: %v", err)
		}
		if wp.FederationHandle != 7 || wp.FederateHandle != 42 || wp.FederateName != "FedAlpha" {
			t.Fatalf("WelcomePack mismatch: %+v", wp)
		}
	})

	t.Run("PublishAndSubscribeObjectClass", func(t *testing.T) {
		pubDec := roundTrip(t, metamodel.NewPublishObjectClass(1001, []any{metamodel.NewJavaInteger(1002), metamodel.NewJavaInteger(1003)}))
		if pubDec.Get("classHandle") != int32(1001) {
			t.Fatalf("PublishObjectClass classHandle = %v, want 1001", pubDec.Get("classHandle"))
		}
		subDec := roundTrip(t, metamodel.NewSubscribeObjectClass(1001, []any{metamodel.NewJavaInteger(1002)}))
		if subDec.Get("classHandle") != int32(1001) {
			t.Fatalf("SubscribeObjectClass classHandle = %v, want 1001", subDec.Get("classHandle"))
		}
	})

	t.Run("RegisterDiscoverDeleteObject", func(t *testing.T) {
		regDec := roundTrip(t, metamodel.NewRegisterObject(1001, "Tank01"))
		if regDec.Get("classHandle") != int32(1001) || regDec.Get("objectName") != "Tank01" {
			t.Fatalf("RegisterObject roundtrip mismatch: %+v", regDec.Fields)
		}
		discDec := roundTrip(t, metamodel.NewDiscoverObject(55, 1001, "Tank01"))
		if discDec.Get("objectHandle") != int32(55) || discDec.Get("classHandle") != int32(1001) || discDec.Get("objectName") != "Tank01" {
			t.Fatalf("DiscoverObject roundtrip mismatch: %+v", discDec.Fields)
		}
		delDec := roundTrip(t, metamodel.NewDeleteObject(55, []byte("bye")))
		if delDec.Get("objectHandle") != int32(55) {
			t.Fatalf("DeleteObject roundtrip mismatch: %+v", delDec.Fields)
		}
	})

	t.Run("UpdateAndReflectAttributes_HashMapRoundTrip", func(t *testing.T) {
		attrs := map[any]any{
			metamodel.NewJavaInteger(1002): []byte("pos-123"),
			metamodel.NewJavaInteger(1003): []byte("vel-456"),
		}
		uavDec := roundTrip(t, metamodel.NewUpdateAttributes(55, attrs, []byte("tag1")))
		if uavDec.Get("objectId") != int32(55) {
			t.Fatalf("UpdateAttributes objectId = %v, want 55", uavDec.Get("objectId"))
		}
		if tag, ok := uavDec.Get("tag").([]byte); !ok || !bytes.Equal(tag, []byte("tag1")) {
			t.Fatalf("UpdateAttributes tag = %v, want tag1", uavDec.Get("tag"))
		}
		ravDec := roundTrip(t, metamodel.NewReflectAttributes(55, attrs, []byte("tag2")))
		if ravDec.Get("objectId") != int32(55) {
			t.Fatalf("ReflectAttributes objectId = %v, want 55", ravDec.Get("objectId"))
		}
	})

	t.Run("SendAndReceiveInteraction", func(t *testing.T) {
		params := map[any]any{metamodel.NewJavaInteger(2002): []byte("boom")}
		siDec := roundTrip(t, metamodel.NewSendInteraction(2001, params, []byte("fire")))
		if siDec.Get("interactionId") != int32(2001) {
			t.Fatalf("SendInteraction interactionId = %v, want 2001", siDec.Get("interactionId"))
		}
		riDec := roundTrip(t, metamodel.NewReceiveInteraction(2001, params, []byte("fire")))
		if riDec.Get("interactionId") != int32(2001) {
			t.Fatalf("ReceiveInteraction interactionId = %v, want 2001", riDec.Get("interactionId"))
		}
	})

	t.Run("SyncPointsAndTimeAdvanceGrant", func(t *testing.T) {
		if dec := roundTrip(t, metamodel.NewRegisterSyncPoint("Ready", 2)); dec.Get("label") != "Ready" {
			t.Fatalf("RegisterSyncPoint mismatch: %+v", dec.Fields)
		}
		if dec := roundTrip(t, metamodel.NewAnnounceSyncPoint("Ready")); dec.Get("label") != "Ready" {
			t.Fatalf("AnnounceSyncPoint mismatch: %+v", dec.Fields)
		}
		if dec := roundTrip(t, metamodel.NewSyncPointAchieved("Ready", 2)); dec.Get("label") != "Ready" {
			t.Fatalf("SyncPointAchieved mismatch: %+v", dec.Fields)
		}
		if dec := roundTrip(t, metamodel.NewFederationSynchronized("Ready")); dec.Get("label") != "Ready" {
			t.Fatalf("FederationSynchronized mismatch: %+v", dec.Fields)
		}
		if dec := roundTrip(t, metamodel.NewTimeAdvanceGrant(2, 12.5)); dec.Get("time") != 12.5 {
			t.Fatalf("TimeAdvanceGrant mismatch: %+v", dec.Fields)
		}
	})

	t.Run("ResponsesAndProbe", func(t *testing.T) {
		if dec := roundTrip(t, metamodel.NewRtiProbe()); dec.Class.Name != "org.portico.impl.hla1516e.types.RtiProbe" && !strings.Contains(dec.Class.Name, "RtiProbe") {
			t.Fatalf("RtiProbe class mismatch: %s", dec.Class.Name)
		}
		errResp := metamodel.NewResponseMessage(true)
		errResp.Set("exception", "ObjectInstanceNotKnown")
		decErr := roundTrip(t, errResp)
		if decErr.Get("isError") != true || decErr.Get("exception") != "ObjectInstanceNotKnown" {
			t.Fatalf("ResponseMessage error roundtrip mismatch: %+v", decErr.Fields)
		}
		esrDec := roundTrip(t, metamodel.NewExtendedSuccessResponse(metamodel.NewJavaInteger(99)))
		resultsMap, ok := esrDec.Get("results").(*metamodel.JavaObject)
		if !ok || resultsMap == nil {
			t.Fatalf("ExtendedSuccessResponse missing results HashMap: %+v", esrDec.Fields)
		}
		entries, ok := resultsMap.Get("entries").(map[any]any)
		if !ok || entries["result"] == nil {
			t.Fatalf("ExtendedSuccessResponse results missing 'result' entry: %+v", resultsMap.Fields)
		}
	})
}
