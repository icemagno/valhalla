package core

import (
	"testing"

	"github.com/icemagno/valhalla/pkg/common"
)

func TestOwnershipManager_Lifecycle(t *testing.T) {
	om := NewOwnershipManager()

	obj := common.ObjectInstanceHandle(10)
	attrs := []common.AttributeHandle{101, 102}

	// 1. Initial registration by Federate 1
	om.RegisterInitialOwnership(obj, attrs, 1)

	for _, attr := range attrs {
		owner, owned := om.QueryAttributeOwnership(obj, attr)
		if !owned || owner != 1 {
			t.Fatalf("expected owner 1 for attr %d, got %d (owned=%v)", attr, owner, owned)
		}
	}

	// 2. Non-owner tries to divest
	err := om.UnconditionalDivestiture(obj, attrs, 2)
	if err != ErrNotAttributeOwner {
		t.Fatalf("expected ErrNotAttributeOwner, got %v", err)
	}

	// 3. Owner divests unconditionally
	err = om.UnconditionalDivestiture(obj, attrs, 1)
	if err != nil {
		t.Fatalf("failed to divest: %v", err)
	}

	for _, attr := range attrs {
		owner, owned := om.QueryAttributeOwnership(obj, attr)
		if owned || owner != 0 {
			t.Fatalf("expected unowned for attr %d, got %d (owned=%v)", attr, owner, owned)
		}
	}

	// 4. Federate 2 acquires ownership of divested attributes
	err = om.AcquireAttributeOwnership(obj, attrs, 2)
	if err != nil {
		t.Fatalf("failed to acquire ownership: %v", err)
	}

	for _, attr := range attrs {
		owner, owned := om.QueryAttributeOwnership(obj, attr)
		if !owned || owner != 2 {
			t.Fatalf("expected owner 2 for attr %d, got %d (owned=%v)", attr, owner, owned)
		}
	}

	// 5. Federate 3 attempts acquisition of already-owned attribute
	err = om.AcquireAttributeOwnership(obj, attrs, 3)
	if err != ErrAttributeAlreadyOwned {
		t.Fatalf("expected ErrAttributeAlreadyOwned, got %v", err)
	}

	// 6. Resign of Federate 2 divests attributes
	om.RemoveFederate(2)
	owner, owned := om.QueryAttributeOwnership(obj, 101)
	if owned || owner != 0 {
		t.Fatalf("expected unowned after resign, got %d", owner)
	}

	// 7. Object deletion cleans state (re-acquire first to ensure deletion actually clears owners)
	if err := om.AcquireAttributeOwnership(obj, attrs, 3); err != nil {
		t.Fatalf("expected federate 3 to acquire unowned attrs before deletion test: %v", err)
	}
	om.DeleteObjectOwnership(obj)
	for _, attr := range attrs {
		if owner, owned := om.QueryAttributeOwnership(obj, attr); owned || owner != 0 {
			t.Fatalf("expected attr %d to be unowned after DeleteObjectOwnership, got owner=%d owned=%v", attr, owner, owned)
		}
	}
}
