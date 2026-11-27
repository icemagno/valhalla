package main

import (
	"errors"
	"fmt"
	"testing"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/lrc"
)

func TestCABI_DefaultSuite_ErrorMappingAndRegistry(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		expected int32
	}{
		{"nil", nil, 0},
		{"not connected", lrc.ErrNotConnected, 1},
		{"connection closed", fmt.Errorf("wrap: %w", lrc.ErrConnectionClosed), 1},
		{"invalid logical time", common.ErrInvalidLogicalTime, 2},
		{"invalid lookahead", common.ErrInvalidLookahead, 2},
		{"not owner", common.ErrAttributeNotOwned, 4},
		{"delete priv not held", common.ErrDeletePrivilegeNotHeld, 17},
		{"federation exists", common.ErrFederationExecutionAlreadyExists, 6},
		{"federation not found", common.ErrFederationExecutionDoesNotExist, 7},
		{"class not found", common.ErrObjectClassNotDefined, 8},
		{"attr not found", common.ErrAttributeNotDefined, 9},
		{"object not found", common.ErrObjectInstanceNotKnown, 10},
		{"not member", common.ErrFederateNotExecutionMember, 11},
		{"already member", common.ErrFederateAlreadyExecutionMember, 15},
		{"object name in use", common.ErrObjectInstanceNameInUse, 16},
		{"federates joined", common.ErrFederatesCurrentlyJoined, 12},
		{"federate name in use", common.ErrFederateNameAlreadyInUse, 13},
		{"interaction not found", common.ErrInteractionClassNotDefined, 14},
		{"unknown error", errors.New("unexpected"), 3},
	}

	for _, tc := range cases {
		got := int32(mapErrorToStatusCode(tc.err))
		if got != tc.expected {
			t.Errorf("%s: got %d, want %d", tc.name, got, tc.expected)
		}
	}

	// Verify nil ambassador lookup is safely rejected without panicking
	if ctx, ok := getContext(nil); ok || ctx != nil {
		t.Fatalf("expected getContext(nil) to return (nil, false)")
	}
}

func TestCABI_AntiOOMBoundsValidation(t *testing.T) {
	if validateCABICount(-1) || validateCABICount(maxCABICount+1) || validateCABICount(0x7FFFFFFF) {
		t.Fatalf("validateCABICount accepted out-of-bounds value")
	}
	if !validateCABICount(0) || !validateCABICount(maxCABICount) {
		t.Fatalf("validateCABICount rejected valid boundary value")
	}
	if validateCABITagLen(-1) || validateCABITagLen(maxCABITagLen+1) || validateCABITagLen(0x7FFFFFFF) {
		t.Fatalf("validateCABITagLen accepted out-of-bounds value")
	}
	if !validateCABITagLen(0) || !validateCABITagLen(maxCABITagLen) {
		t.Fatalf("validateCABITagLen rejected valid boundary value")
	}
	if validateCABIValueLen(-1) || validateCABIValueLen(maxCABIValueLen+1) || validateCABIValueLen(0x7FFFFFFF) {
		t.Fatalf("validateCABIValueLen accepted out-of-bounds value")
	}
	if !validateCABIValueLen(0) || !validateCABIValueLen(maxCABIValueLen) {
		t.Fatalf("validateCABIValueLen rejected valid boundary value")
	}
}
