package crc

import (
	"errors"
	"testing"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc/core"
)

func TestHLA4_DeletePrivilegeTransfer(t *testing.T) {
	fed := NewFederationExecution(1, "DeletePrivFed")
	cls, err := fed.OMT().AddObjectClass("Tank", core.HLAobjectRootHandle)
	if err != nil {
		t.Fatalf("AddObjectClass failed: %v", err)
	}

	obj, err := fed.RegisterObjectInstance(int32(cls.Handle), "Tank-1", 1)
	if err != nil {
		t.Fatalf("RegisterObjectInstance failed: %v", err)
	}

	// 1. Federate 1 divests HLAprivilegeToDeleteObjectHandle
	if err := fed.OwnershipManager().UnconditionalDivestiture(
		common.ObjectInstanceHandle(obj.Handle),
		[]common.AttributeHandle{core.HLAprivilegeToDeleteObjectHandle},
		1,
	); err != nil {
		t.Fatalf("divest privilege failed: %v", err)
	}

	// While divested, Federate 1 MUST NOT be able to delete
	if _, err := fed.DeleteObjectInstance(obj.Handle, 1, nil); !errors.Is(err, common.ErrDeletePrivilegeNotHeld) {
		t.Fatalf("expected ErrDeletePrivilegeNotHeld for original owner after divesting delete privilege, got: %v", err)
	}

	// 2. Federate 2 acquires HLAprivilegeToDeleteObjectHandle
	if err := fed.OwnershipManager().AcquireAttributeOwnership(
		common.ObjectInstanceHandle(obj.Handle),
		[]common.AttributeHandle{core.HLAprivilegeToDeleteObjectHandle},
		2,
	); err != nil {
		t.Fatalf("acquire privilege failed: %v", err)
	}

	// Federate 1 still cannot delete
	if _, err := fed.DeleteObjectInstance(obj.Handle, 1, nil); !errors.Is(err, common.ErrDeletePrivilegeNotHeld) {
		t.Fatalf("expected ErrDeletePrivilegeNotHeld for Federate 1 after transfer to Federate 2, got: %v", err)
	}

	// Federate 2 (new holder of delete privilege) MUST succeed
	if _, err := fed.DeleteObjectInstance(obj.Handle, 2, nil); err != nil {
		t.Fatalf("expected Federate 2 holding delete privilege to delete object, got: %v", err)
	}
}

func TestHLA4_DivestedAttributeCannotBeUpdated(t *testing.T) {
	fed := NewFederationExecution(1, "DivestUpdateFed")
	cls, err := fed.OMT().AddObjectClass("Drone", core.HLAobjectRootHandle)
	if err != nil {
		t.Fatalf("AddObjectClass failed: %v", err)
	}
	posAttr, err := fed.OMT().AddAttribute(cls.Handle, "Pos", "HLAoctet")
	if err != nil {
		t.Fatalf("AddAttribute failed: %v", err)
	}

	obj, err := fed.RegisterObjectInstance(int32(cls.Handle), "Drone-1", 1)
	if err != nil {
		t.Fatalf("RegisterObjectInstance failed: %v", err)
	}

	// Divest Pos attribute -> now unowned
	if err := fed.OwnershipManager().UnconditionalDivestiture(
		common.ObjectInstanceHandle(obj.Handle),
		[]common.AttributeHandle{posAttr.Handle},
		1,
	); err != nil {
		t.Fatalf("divest failed: %v", err)
	}

	// Neither Federate 1 nor Federate 2 may update an unowned/divested attribute
	for _, sender := range []int32{1, 2} {
		_, err := fed.UpdateAndRouteAttributes(
			sender,
			common.ObjectInstanceHandle(obj.Handle),
			map[common.AttributeHandle][]byte{posAttr.Handle: []byte("hijack")},
			nil, false, 0, nil, [32]byte{},
		)
		if !errors.Is(err, core.ErrNotAttributeOwner) {
			t.Fatalf("sender %d: expected ErrNotAttributeOwner on divested attribute, got: %v", sender, err)
		}
	}
}

func TestHLA4_DDM_RealOMTDimensionHandles1001(t *testing.T) {
	fed := NewFederationExecution(1, "DDMFed")
	dimX, err := fed.OMT().RegisterDimension("X", "HLAinteger32BE", 1000000)
	if err != nil {
		t.Fatalf("RegisterDimension X failed: %v", err)
	}
	dimY, err := fed.OMT().RegisterDimension("Y", "HLAinteger32BE", 1000000)
	if err != nil {
		t.Fatalf("RegisterDimension Y failed: %v", err)
	}
	dimZ, err := fed.OMT().RegisterDimension("Z", "HLAinteger32BE", 1000000)
	if err != nil {
		t.Fatalf("RegisterDimension Z failed: %v", err)
	}
	if dimX < 1000 {
		t.Fatalf("expected real OMT dimension handle >= 1000, got %d", dimX)
	}

	regHandle := fed.CreateRegionWithDimensions(1, []uint32{uint32(dimX), uint32(dimY), uint32(dimZ)})

	if err := fed.SetRegionRangeBounds(regHandle, uint32(dimX), 100, 200); err != nil {
		t.Fatalf("SetRegionRangeBounds(X=%d) failed: %v", dimX, err)
	}
	if err := fed.SetRegionRangeBounds(regHandle, uint32(dimY), 300, 400); err != nil {
		t.Fatalf("SetRegionRangeBounds(Y=%d) failed: %v", dimY, err)
	}
	if err := fed.SetRegionRangeBounds(regHandle, uint32(dimZ), 500, 600); err != nil {
		t.Fatalf("SetRegionRangeBounds(Z=%d) failed: %v", dimZ, err)
	}

	reg, ok := fed.GetRegion(regHandle)
	if !ok {
		t.Fatalf("GetRegion failed")
	}
	if reg.MinX != 100 || reg.MaxX != 200 || reg.MinY != 300 || reg.MaxY != 400 || reg.MinZ != 500 || reg.MaxZ != 600 {
		t.Fatalf("real OMT dimension handles did not update region bounds! got X=[%d,%d] Y=[%d,%d] Z=[%d,%d]",
			reg.MinX, reg.MaxX, reg.MinY, reg.MaxY, reg.MinZ, reg.MaxZ)
	}

	// Unknown dimension handle MUST return an error (not silently succeed!)
	if err := fed.SetRegionRangeBounds(regHandle, 9999, 10, 20); err == nil {
		t.Fatalf("expected error for unknown dimension handle 9999, got nil")
	}

	// DeleteRegion cleans up region
	if err := fed.DeleteRegion(regHandle); err != nil {
		t.Fatalf("DeleteRegion failed: %v", err)
	}
	if _, ok := fed.GetRegion(regHandle); ok {
		t.Fatalf("expected region to be deleted")
	}
}
