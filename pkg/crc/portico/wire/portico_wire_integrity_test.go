package wire

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/icemagno/valhalla/pkg/crc/portico/wire/metamodel"
)

func TestBuildErrorResponsePayload_PreservesExceptionMessageOnWire(t *testing.T) {
	errMsg := "FederationExecutionDoesNotExist: AlphaFed"
	errPayload := BuildErrorResponsePayload(errMsg)
	okPayload := BuildSuccessResponsePayload()

	if len(errPayload) == 0 {
		t.Fatalf("BuildErrorResponsePayload returned empty payload")
	}
	if bytes.Equal(errPayload, okPayload) || len(errPayload) == len(okPayload) {
		t.Fatalf("error payload (%d bytes) did not encode exception message (same length as OK payload %d bytes)", len(errPayload), len(okPayload))
	}

	decoded, err := DeserializePorticoControlMessage(errPayload)
	if err != nil {
		t.Fatalf("DeserializePorticoControlMessage failed: %v", err)
	}
	isErr, _ := decoded.Get("isError").(bool)
	if !isErr {
		t.Fatalf("expected isError=true, got false")
	}
	gotMsg, _ := decoded.Get("exception").(string)
	if gotMsg != errMsg {
		t.Fatalf("expected exception=%q, got %q", errMsg, gotMsg)
	}
}

func TestPorticoMessages_IsAsyncFieldSerializedAndDecoded(t *testing.T) {
	cases := []struct {
		name      string
		obj       *JavaObject
		wantAsync bool
	}{
		{"JoinFederation", NewJoinFederationObject("Fed", "Node", "Type"), true},
		{"AnnounceSyncPoint", metamodel.NewAnnounceSyncPoint("SP1"), true},
		{"FederationSynchronized", metamodel.NewFederationSynchronized("SP1"), true},
		{"RegisterSyncPoint", metamodel.NewRegisterSyncPoint("SP1", 1), true},
		{"ResignFederation", NewResignFederationObject("Fed", "Node", "Type", "NO_ACTION"), true},
		{"SyncPointAchieved", NewSyncPointAchievedObject("SP1", 1), true},
		{"TimeAdvanceGrant", metamodel.NewTimeAdvanceGrant(1, 10.0), true},
		{"RtiProbe", NewRtiProbeObject(), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := SerializePorticoControlMessage(tc.obj)
			if err != nil {
				t.Fatalf("SerializePorticoControlMessage failed: %v", err)
			}
			decoded, err := DeserializePorticoControlMessage(raw)
			if err != nil {
				t.Fatalf("DeserializePorticoControlMessage failed: %v", err)
			}
			got, _ := decoded.Get("isAsync").(bool)
			if got != tc.wantAsync {
				t.Fatalf("%s: expected isAsync=%v on wire, got %v", tc.name, tc.wantAsync, got)
			}
		})
	}
}

func TestDeserializePorticoControlMessage_SendInteraction_NonNilClassDesc(t *testing.T) {
	// Manual-marshal buffer: JOS header + TC_BLOCKDATA + [0x01][0x00][255 (SendInteraction)] + 26 bytes fixed + 0 params
	var inner bytes.Buffer
	inner.Write([]byte{0x01, 0x00, 255})
	_ = binary.Write(&inner, binary.BigEndian, int32(42))    // interactionId
	_ = binary.Write(&inner, binary.BigEndian, int32(1))     // sourceFederate
	_ = binary.Write(&inner, binary.BigEndian, int32(2))     // targetFederation
	_ = binary.Write(&inner, binary.BigEndian, float64(5.0)) // timestamp
	_ = binary.Write(&inner, binary.BigEndian, uint16(0))    // tagLen
	_ = binary.Write(&inner, binary.BigEndian, int32(0))     // regionToken
	_ = binary.Write(&inner, binary.BigEndian, uint32(0))    // paramCount

	var buf bytes.Buffer
	buf.Write([]byte{0xAC, 0xED, 0x00, 0x05})
	buf.WriteByte(TC_BLOCKDATA)
	buf.WriteByte(byte(inner.Len()))
	buf.Write(inner.Bytes())

	decoded, err := DeserializePorticoControlMessage(buf.Bytes())
	if err != nil {
		t.Fatalf("DeserializePorticoControlMessage(SendInteraction) failed: %v", err)
	}
	if decoded.Class == nil {
		t.Fatalf("expected non-nil ClassDesc for SendInteraction, got nil (ClassDescObject)")
	}
	if decoded.Class.Name != metamodel.ClassDescSendInteraction.Name {
		t.Fatalf("expected ClassDesc %q, got %q", metamodel.ClassDescSendInteraction.Name, decoded.Class.Name)
	}
}

func TestParseWelcomePack_HandlesLongBlockDataViaJOSDecoder(t *testing.T) {
	// Long strings (> 255 bytes total) force TC_BLOCKDATALONG (0x7A + 4-byte length),
	// which breaks any hardcoded offset parser assuming TC_BLOCKDATA (0x77 + 1-byte length).
	longFedName := "Fed_" + strings.Repeat("X", 150)
	longNodeName := "Node_" + strings.Repeat("Y", 150)
	fedType := "IEEE1516e"

	wp := metamodel.NewWelcomePack(7, 42, longFedName, longNodeName, fedType)
	resp := metamodel.NewExtendedSuccessResponse(wp)
	raw, err := SerializePorticoControlMessage(resp)
	if err != nil {
		t.Fatalf("SerializePorticoControlMessage failed: %v", err)
	}

	info, err := ParseWelcomePack(raw)
	if err != nil {
		t.Fatalf("ParseWelcomePack failed on TC_BLOCKDATALONG WelcomePack: %v", err)
	}
	if info.FederationHandle != 7 || info.FederateHandle != 42 {
		t.Fatalf("unexpected handles: fed=%d federate=%d", info.FederationHandle, info.FederateHandle)
	}
	if info.FederationName != longFedName || info.FederateName != longNodeName || info.FederateType != fedType {
		t.Fatalf("decoded strings mismatch: fed=%q node=%q type=%q", info.FederationName, info.FederateName, info.FederateType)
	}
}
