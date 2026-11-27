package common

import (
	"bytes"
	"errors"
	"testing"
)

func TestNativeHeader_RoundtripAndGoldenBytes(t *testing.T) {
	var merkle [32]byte
	for i := range merkle {
		merkle[i] = byte(i + 1)
	}
	in := NativeHeader{
		Magic:      NativeHeaderMagic,
		Version:    NativeProtocolVer,
		Flags:      FlagNativeSSZ | FlagNativeMerkle,
		PayloadLen: 1024,
		MsgType:    NativeMsgUpdateAttributes,
		Reserved:   0,
		ReqID:      77,
		FedID:      3,
		FederateID: 9,
		Reserved2:  0,
		MerkleRoot: merkle,
	}

	raw := WriteNativeHeader(in)
	if len(raw) != NativeHeaderLength {
		t.Fatalf("expected header length %d, got %d", NativeHeaderLength, len(raw))
	}
	// Golden check: Magic "VA" (0x5641) little-endian -> 0x41, 0x56, Version 4
	if raw[0] != 0x41 || raw[1] != 0x56 || raw[2] != 4 {
		t.Fatalf("golden header prefix mismatch: got %x", raw[:3])
	}

	out, err := ReadNativeHeader(bytes.NewReader(raw[:]))
	if err != nil {
		t.Fatalf("ReadNativeHeader failed: %v", err)
	}
	if out != in {
		t.Fatalf("header roundtrip mismatch:\ngot:  %+v\nwant: %+v", out, in)
	}
}

func TestNativeHeader_AdversarialRejection(t *testing.T) {
	base := WriteNativeHeader(NativeHeader{
		Magic:      NativeHeaderMagic,
		Version:    NativeProtocolVer,
		PayloadLen: 16,
		MsgType:    NativeMsgHeartbeat,
	})

	// 1. Invalid magic -> wraps ErrNativeInvalidMagic
	badMagic := base
	badMagic[0] = 0xFF
	if _, err := ReadNativeHeader(bytes.NewReader(badMagic[:])); !errors.Is(err, ErrNativeInvalidMagic) {
		t.Fatalf("expected ErrNativeInvalidMagic, got %v", err)
	}

	// 2. Invalid version -> wraps ErrNativeInvalidVersion
	badVer := base
	badVer[2] = 99
	if _, err := ReadNativeHeader(bytes.NewReader(badVer[:])); !errors.Is(err, ErrNativeInvalidVersion) {
		t.Fatalf("expected ErrNativeInvalidVersion, got %v", err)
	}

	// 3. Oversized payload -> wraps ErrNativePayloadTooLarge
	badLen := WriteNativeHeader(NativeHeader{
		Magic:      NativeHeaderMagic,
		Version:    NativeProtocolVer,
		PayloadLen: MaxNativePayloadLen + 1,
		MsgType:    NativeMsgHeartbeat,
	})
	if _, err := ReadNativeHeader(bytes.NewReader(badLen[:])); !errors.Is(err, ErrNativePayloadTooLarge) {
		t.Fatalf("expected ErrNativePayloadTooLarge, got %v", err)
	}

	// 4. Truncated header
	if _, err := ReadNativeHeader(bytes.NewReader(base[:10])); err == nil {
		t.Fatalf("expected error on truncated header, got nil")
	}
}
