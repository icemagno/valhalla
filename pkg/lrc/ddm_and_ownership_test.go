package lrc_test

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc"
	"github.com/icemagno/valhalla/pkg/lrc"
)

type spatialAmbassador struct {
	lrc.NullFederateAmbassador
	mu             sync.Mutex
	reflectedAttrs map[int32][]byte
	reflectedTag   []byte
	reflectCount   int
}

func (m *spatialAmbassador) ReflectAttributeValues(theObject int32, theAttributes map[int32][]byte, tag []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reflectedAttrs = make(map[int32][]byte, len(theAttributes))
	for k, v := range theAttributes {
		m.reflectedAttrs[k] = v
	}
	m.reflectedTag = tag
	m.reflectCount++
}

func (m *spatialAmbassador) GetLastReflection() (map[int32][]byte, []byte, int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.reflectedAttrs, m.reflectedTag, m.reflectCount
}

func TestSprint6_OwnershipHandover(t *testing.T) {
	server := crc.NewServer("127.0.0.1:0")
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start CRC server: %v", err)
	}
	defer server.Stop()
	addr := server.Addr().String()

	fedName := "Sprint6OwnershipFed"

	// 1. Federate A setup
	ambA := lrc.NewRTIambassador()
	fedAmbA := &mockAmbassador{}
	if err := ambA.Connect(fedAmbA, addr); err != nil {
		t.Fatalf("AmbA Connect failed: %v", err)
	}
	defer ambA.Disconnect()

	if err := ambA.CreateFederationExecution(fedName); err != nil {
		t.Fatalf("CreateFederation failed: %v", err)
	}

	fedAHandle, err := ambA.JoinFederationExecution("FederateAlpha", fedName)
	if err != nil {
		t.Fatalf("AmbA Join failed: %v", err)
	}

	// 2. Federate B setup
	ambB := lrc.NewRTIambassador()
	fedAmbB := &mockAmbassador{}
	if err := ambB.Connect(fedAmbB, addr); err != nil {
		t.Fatalf("AmbB Connect failed: %v", err)
	}
	defer ambB.Disconnect()

	fedBHandle, err := ambB.JoinFederationExecution("FederateBravo", fedName)
	if err != nil {
		t.Fatalf("AmbB Join failed: %v", err)
	}

	// 3. Fed A publishes object root class and registers object
	clsHandle := int32(common.HLAobjectRootHandle)
	attrHandle := int32(common.HLAprivilegeToDeleteObjectHandle)
	if err := ambA.PublishObjectClassAttributes(clsHandle, []int32{attrHandle}); err != nil {
		t.Fatalf("PublishObjectClass failed: %v", err)
	}

	objHandle, err := ambA.RegisterObjectInstance(clsHandle, "TacticalAsset_01")
	if err != nil {
		t.Fatalf("RegisterObjectInstance failed: %v", err)
	}

	// 4. Initial ownership verification: Fed A owns attrHandle
	owner, isOwned, err := ambA.QueryAttributeOwnership(objHandle, attrHandle)
	if err != nil {
		t.Fatalf("QueryAttributeOwnership failed: %v", err)
	}
	if !isOwned || owner != fedAHandle {
		t.Fatalf("Expected owner %d and isOwned=true, got owner=%d isOwned=%v", fedAHandle, owner, isOwned)
	}

	// 5. Fed A divests ownership unconditionally
	if err := ambA.UnconditionalAttributeOwnershipDivestiture(objHandle, []int32{attrHandle}); err != nil {
		t.Fatalf("UnconditionalDivestiture failed: %v", err)
	}

	// 6. Query after divestiture: attribute is now unowned
	owner, isOwned, err = ambA.QueryAttributeOwnership(objHandle, attrHandle)
	if err != nil {
		t.Fatalf("QueryAttributeOwnership after divest failed: %v", err)
	}
	if isOwned {
		t.Fatalf("Expected attribute to be unowned after divest, but got isOwned=true owner=%d", owner)
	}

	// 7. Fed B acquires ownership
	if err := ambB.AcquireAttributeOwnership(objHandle, []int32{attrHandle}); err != nil {
		t.Fatalf("Fed B AcquireAttributeOwnership failed: %v", err)
	}

	// 8. Query after acquisition: Fed B is now owner
	owner, isOwned, err = ambB.QueryAttributeOwnership(objHandle, attrHandle)
	if err != nil {
		t.Fatalf("QueryAttributeOwnership after acquisition failed: %v", err)
	}
	if !isOwned || owner != fedBHandle {
		t.Fatalf("Expected owner %d (Fed B), got owner=%d isOwned=%v", fedBHandle, owner, isOwned)
	}

	// 9. Fed A attempts to divest attribute owned by Fed B -> must fail
	if err := ambA.UnconditionalAttributeOwnershipDivestiture(objHandle, []int32{attrHandle}); err == nil {
		t.Fatalf("Expected divestiture by non-owner to fail, but succeeded")
	}

	// 10. Fed B divests and resigns cleanly
	if err := ambB.UnconditionalAttributeOwnershipDivestiture(objHandle, []int32{attrHandle}); err != nil {
		t.Fatalf("Fed B divest failed: %v", err)
	}
	_ = ambA.ResignFederationExecution()
	_ = ambB.ResignFederationExecution()
}

func TestSprint6_DDM_Morton3DSpatialFiltering(t *testing.T) {
	server := crc.NewServer("127.0.0.1:0")
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start CRC server: %v", err)
	}
	defer server.Stop()
	addr := server.Addr().String()

	fedName := "Sprint6DDMFed"

	// 1. Publisher Federate (Alpha)
	ambAlpha := lrc.NewRTIambassador()
	if err := ambAlpha.Connect(&lrc.NullFederateAmbassador{}, addr); err != nil {
		t.Fatalf("Alpha Connect failed: %v", err)
	}
	defer ambAlpha.Disconnect()

	if err := ambAlpha.CreateFederationExecution(fedName); err != nil {
		t.Fatalf("CreateFederation failed: %v", err)
	}
	if _, err := ambAlpha.JoinFederationExecution("AlphaPublisher", fedName); err != nil {
		t.Fatalf("Alpha Join failed: %v", err)
	}

	// 2. Regional Subscriber Federate (Bravo)
	ambBravo := lrc.NewRTIambassador()
	fedAmbBravo := &spatialAmbassador{}
	if err := ambBravo.Connect(fedAmbBravo, addr); err != nil {
		t.Fatalf("Bravo Connect failed: %v", err)
	}
	defer ambBravo.Disconnect()

	if _, err := ambBravo.JoinFederationExecution("BravoRegionalWatcher", fedName); err != nil {
		t.Fatalf("Bravo Join failed: %v", err)
	}

	clsHandle := int32(common.HLAobjectRootHandle)
	attrHandle := int32(common.HLAprivilegeToDeleteObjectHandle)

	// Alpha publishes class
	if err := ambAlpha.PublishObjectClassAttributes(clsHandle, []int32{attrHandle}); err != nil {
		t.Fatalf("Alpha PublishObjectClass failed: %v", err)
	}

	objHandle, err := ambAlpha.RegisterObjectInstance(clsHandle, "SpatialRadarTarget_01")
	if err != nil {
		t.Fatalf("Alpha RegisterObjectInstance failed: %v", err)
	}

	// 3. Bravo defines Sector Alpha (bounding box: [-1000, 1000] on X and Y, [0, 500] on Z)
	regHandle, err := ambBravo.CreateRegion([]int32{1, 2, 3})
	if err != nil {
		t.Fatalf("Bravo CreateRegion failed: %v", err)
	}
	if err := ambBravo.SetMetricBounds(regHandle, -1000.0, 1000.0, -1000.0, 1000.0, 0.0, 500.0); err != nil {
		t.Fatalf("Bravo SetMetricBounds failed: %v", err)
	}

	// Bravo subscribes using the 3D Morton DDM region
	if err := ambBravo.SubscribeObjectClassAttributesWithRegion(clsHandle, []int32{attrHandle}, regHandle); err != nil {
		t.Fatalf("Bravo SubscribeWithRegion failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 4. Test Case 1: Point INSIDE Sector Alpha (X=100.0, Y=200.0, Z=150.0)
	insidePayload := []byte("ECEF_TARGET_INSIDE_SECTOR")
	err = ambAlpha.UpdateAttributeValuesWithSpatial(
		objHandle,
		map[int32][]byte{attrHandle: insidePayload},
		100.0, 200.0, 150.0,
		[]byte("Tag-Inside"),
	)
	if err != nil {
		t.Fatalf("Alpha UpdateAttributeValuesWithSpatial (inside) failed: %v", err)
	}

	// Wait and evoke all pending callbacks on Bravo
	time.Sleep(100 * time.Millisecond)
	var anyEvoked bool
	for {
		evoked, err := ambBravo.EvokeCallback(50 * time.Millisecond)
		if err != nil || !evoked {
			break
		}
		anyEvoked = true
	}
	if !anyEvoked {
		t.Fatalf("Expected callbacks to be evoked for inside-region update, but got none")
	}

	attrs, tag, count := fedAmbBravo.GetLastReflection()
	if count != 1 {
		t.Fatalf("Expected 1 reflection received, got %d", count)
	}
	if !bytes.Equal(attrs[attrHandle], insidePayload) {
		t.Errorf("Reflected attribute value mismatch: got %s, want %s", string(attrs[attrHandle]), string(insidePayload))
	}
	if !bytes.Equal(tag, []byte("Tag-Inside")) {
		t.Errorf("Tag mismatch: got %s, want Tag-Inside", string(tag))
	}

	// 5. Test Case 2: Point OUTSIDE Sector Alpha (X=50000.0, Y=50000.0, Z=10000.0)
	outsidePayload := []byte("ECEF_TARGET_OUTSIDE_SECTOR")
	err = ambAlpha.UpdateAttributeValuesWithSpatial(
		objHandle,
		map[int32][]byte{attrHandle: outsidePayload},
		50000.0, 50000.0, 10000.0,
		[]byte("Tag-Outside"),
	)
	if err != nil {
		t.Fatalf("Alpha UpdateAttributeValuesWithSpatial (outside) failed: %v", err)
	}

	// Evoke callback on Bravo: MUST NOT receive any reflection
	time.Sleep(50 * time.Millisecond)
	evokedOutside, err := ambBravo.EvokeCallback(150 * time.Millisecond)
	if err != nil {
		t.Fatalf("Bravo EvokeCallback outside check failed: %v", err)
	}
	if evokedOutside {
		t.Fatalf("CRITICAL: DDM Spatial Filter failed! Bravo received update for point OUTSIDE its region bounds!")
	}

	_, _, countAfter := fedAmbBravo.GetLastReflection()
	if countAfter != 1 {
		t.Fatalf("Expected reflectCount to remain 1, but got %d (leakage detected)", countAfter)
	}

	_ = ambAlpha.ResignFederationExecution()
	_ = ambBravo.ResignFederationExecution()
}

func TestSprint6_DynamicFOMResolution(t *testing.T) {
	server := crc.NewServer("127.0.0.1:0")
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start CRC server: %v", err)
	}
	defer server.Stop()
	addr := server.Addr().String()

	fedName := "FOMHomologFed"

	amb := lrc.NewRTIambassador()
	if err := amb.Connect(&lrc.NullFederateAmbassador{}, addr); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer amb.Disconnect()

	// Create with ValhallaAdvanced.xml FOM module
	if err := amb.CreateFederationExecutionWithFOM(fedName, []string{"ValhallaAdvanced.xml"}); err != nil {
		t.Fatalf("CreateFederationExecutionWithFOM failed: %v", err)
	}

	if _, err := amb.JoinFederationExecution("FOMTester", fedName); err != nil {
		t.Fatalf("Join failed: %v", err)
	}

	// Dynamic FOM lookups
	clsHandle, err := amb.GetObjectClassHandle("TacticalEntity")
	if err != nil {
		t.Fatalf("GetObjectClassHandle('TacticalEntity') failed: %v", err)
	}
	if clsHandle <= 0 {
		t.Fatalf("Expected valid class handle, got %d", clsHandle)
	}

	posHandle, err := amb.GetAttributeHandle(clsHandle, "Position")
	if err != nil {
		t.Fatalf("GetAttributeHandle('Position') failed: %v", err)
	}
	if posHandle <= 0 {
		t.Fatalf("Expected valid attribute handle, got %d", posHandle)
	}

	velHandle, err := amb.GetAttributeHandle(clsHandle, "Velocity")
	if err != nil {
		t.Fatalf("GetAttributeHandle('Velocity') failed: %v", err)
	}
	if velHandle <= 0 {
		t.Fatalf("Expected valid attribute handle, got %d", velHandle)
	}

	dimXHandle, err := amb.GetDimensionHandle("DimensionX")
	if err != nil {
		t.Fatalf("GetDimensionHandle('DimensionX') failed: %v", err)
	}
	if dimXHandle <= 0 {
		t.Fatalf("Expected valid dimension handle, got %d", dimXHandle)
	}

	t.Logf("Dynamic FOM Resolution Succeeded: TacticalEntity=%d, Position=%d, Velocity=%d, DimensionX=%d",
		clsHandle, posHandle, velHandle, dimXHandle)

	_ = amb.ResignFederationExecution()
}
