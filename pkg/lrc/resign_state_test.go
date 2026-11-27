package lrc_test

import (
	"sync"
	"testing"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc"
	"github.com/icemagno/valhalla/pkg/lrc"
)

func TestRTIambassador_ResignAndDisconnect_ClearStateViaPublicAPI(t *testing.T) {
	srv := crc.NewServer("127.0.0.1:0")
	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start CRC server: %v", err)
	}
	defer func() { _ = srv.Stop() }()

	amb := lrc.NewRTIambassador()
	if err := amb.Connect(&lrc.NullFederateAmbassador{}, srv.Addr().String()); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	if err := amb.CreateFederationExecution("StateResetFed"); err != nil {
		t.Fatalf("CreateFederationExecution failed: %v", err)
	}
	if _, err := amb.JoinFederationExecution("FedReset1", "StateResetFed"); err != nil {
		t.Fatalf("JoinFederationExecution failed: %v", err)
	}

	const (
		rootClass = int32(common.HLAobjectRootHandle)
		privAttr  = int32(common.HLAprivilegeToDeleteObjectHandle)
	)
	// Publish, register, and update so the local Merkle stateTree becomes non-zero.
	if err := amb.PublishObjectClassAttributes(rootClass, []int32{privAttr}); err != nil {
		t.Fatalf("PublishObjectClassAttributes failed: %v", err)
	}
	objH, err := amb.RegisterObjectInstance(rootClass, "ObjReset1")
	if err != nil {
		t.Fatalf("RegisterObjectInstance failed: %v", err)
	}
	if err := amb.UpdateAttributeValues(objH, map[int32][]byte{privAttr: []byte("state")}, []byte("t")); err != nil {
		t.Fatalf("UpdateAttributeValues failed: %v", err)
	}
	if amb.GetLocalStateRoot() == ([32]byte{}) {
		t.Fatal("expected non-zero local Merkle root after object update")
	}

	// Resign via public API must clear local Merkle stateTree and membership.
	if err := amb.ResignFederationExecution(); err != nil {
		t.Fatalf("ResignFederationExecution failed: %v", err)
	}
	if root := amb.GetLocalStateRoot(); root != ([32]byte{}) {
		t.Fatalf("expected zero local Merkle root after ResignFederationExecution, got %x", root)
	}
	if amb.LastDesyncDetected() != nil {
		t.Fatalf("expected nil LastDesyncDetected after Resign, got %v", amb.LastDesyncDetected())
	}

	// Rejoin and test concurrent GetLocalStateRoot vs Disconnect (regression test for stateTree data race).
	if _, err := amb.JoinFederationExecution("FedReset2", "StateResetFed"); err != nil {
		t.Fatalf("re-join after resign failed: %v", err)
	}
	if err := amb.PublishObjectClassAttributes(rootClass, []int32{privAttr}); err != nil {
		t.Fatalf("PublishObjectClassAttributes failed: %v", err)
	}
	objH2, err := amb.RegisterObjectInstance(rootClass, "ObjReset2")
	if err != nil {
		t.Fatalf("RegisterObjectInstance 2 failed: %v", err)
	}
	if err := amb.UpdateAttributeValues(objH2, map[int32][]byte{privAttr: []byte("state2")}, nil); err != nil {
		t.Fatalf("UpdateAttributeValues 2 failed: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			_ = amb.GetLocalStateRoot()
		}
	}()
	if err := amb.Disconnect(); err != nil {
		t.Fatalf("Disconnect failed: %v", err)
	}
	wg.Wait()

	if root := amb.GetLocalStateRoot(); root != ([32]byte{}) {
		t.Fatalf("expected zero local Merkle root after Disconnect, got %x", root)
	}
}
