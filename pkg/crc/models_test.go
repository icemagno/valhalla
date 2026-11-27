package crc

import (
	"testing"
)

type mockSession struct {
	id    int32
	proto ProtocolType
}

func (m *mockSession) FederateID() int32                                  { return m.id }
func (m *mockSession) Protocol() ProtocolType                             { return m.proto }
func (m *mockSession) SendNotification(msgType uint8, payload []byte) error { return nil }
func (m *mockSession) SendNotificationWithMerkle(msgType uint8, payload []byte, merkleRoot [32]byte) error { return nil }
func (m *mockSession) Close() error                                       { return nil }

func TestFederationExecution_Lifecycle(t *testing.T) {
	fed := NewFederationExecution(1, "TestFed")

	// 1. Ingressar dois federados
	f1, err := fed.Join("Thor", "ieee1516e", ProtocolPortico, &mockSession{id: 1, proto: ProtocolPortico})
	if err != nil {
		t.Fatalf("falha ao ingressar Thor: %v", err)
	}
	if f1.ID != 1 {
		t.Errorf("esperado handle 1, obtido %d", f1.ID)
	}

	f2, err := fed.Join("ValhallaNode", "native", ProtocolNative, &mockSession{id: 2, proto: ProtocolNative})
	if err != nil {
		t.Fatalf("falha ao ingressar ValhallaNode: %v", err)
	}
	if f2.ID != 2 {
		t.Errorf("esperado handle 2, obtido %d", f2.ID)
	}

	// 2. Tentar ingressar com mesmo nome deve falhar
	_, err = fed.Join("Thor", "ieee1516e", ProtocolPortico, &mockSession{})
	if err != ErrFederateNameAlreadyInUse {
		t.Errorf("esperava ErrFederateNameAlreadyInUse, obtido %v", err)
	}

	// 3. Registrar SyncPoint
	sp, err := fed.RegisterSyncPoint("ReadyToRun", []byte("tag"), 1)
	if err != nil {
		t.Fatalf("falha ao registrar SyncPoint: %v", err)
	}
	if sp.Label != "ReadyToRun" {
		t.Errorf("label incorreto: %s", sp.Label)
	}

	// Registro duplicado deve falhar
	_, err = fed.RegisterSyncPoint("ReadyToRun", nil, 1)
	if err != ErrSyncPointLabelNotUnique {
		t.Errorf("esperava ErrSyncPointLabelNotUnique, obtido %v", err)
	}

	// 4. Conquistar SyncPoint
	synced, err := fed.AchieveSyncPoint("ReadyToRun", 1)
	if err != nil {
		t.Fatalf("erro ao conquistar ponto: %v", err)
	}
	if synced {
		t.Error("nao deveria sincronizar com apenas 1 federado de 2")
	}

	synced, err = fed.AchieveSyncPoint("ReadyToRun", 2)
	if err != nil {
		t.Fatalf("erro ao conquistar ponto: %v", err)
	}
	if !synced {
		t.Error("deveria sincronizar com ambos os federados")
	}

	// 5. Resign
	_, err = fed.Resign(1)
	if err != nil {
		t.Fatalf("erro ao sair: %v", err)
	}

	if _, exists := fed.GetFederate(1); exists {
		t.Error("Thor ainda existe apos resign")
	}
}

func TestFederationExecution_MerkleAuditToggleAndDecoupling(t *testing.T) {
	fed := NewFederationExecution(1, "AuditTestFed")
	if !fed.IsMerkleAuditEnabled() {
		t.Fatalf("expected Merkle audit to be enabled by default")
	}

	// Register an object
	obj, err := fed.RegisterObjectInstance(10, "TestEntity", 1)
	if err != nil {
		t.Fatalf("failed to register object: %v", err)
	}

	// 1. Update with audit enabled -> stateTree is updated
	attrs1 := map[int32][]byte{1: []byte("AuditActiveValue")}
	if err := fed.UpdateObjectAttributes(obj.Handle, attrs1, [32]byte{}); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	rootWithAudit := fed.StateTree().GetGlobalRoot()
	if rootWithAudit == [32]byte{} {
		t.Fatalf("expected non-empty root with audit enabled")
	}

	// 2. Toggle audit off
	fed.SetMerkleAuditEnabled(false)
	if fed.IsMerkleAuditEnabled() {
		t.Fatalf("expected Merkle audit to be disabled")
	}

	// Update with audit disabled -> does not recalculate state tree
	attrs2 := map[int32][]byte{1: []byte("BypassedAuditValue")}
	if err := fed.UpdateObjectAttributes(obj.Handle, attrs2, [32]byte{}); err != nil {
		t.Fatalf("update with audit disabled failed: %v", err)
	}

	rootAfterDisabledUpdate := fed.StateTree().GetGlobalRoot()
	if rootAfterDisabledUpdate != rootWithAudit {
		t.Fatalf("expected state tree root to remain unchanged when audit is disabled: got %x, expected %x", rootAfterDisabledUpdate, rootWithAudit)
	}

	// 3. Re-enable audit -> state tree updates normally again
	fed.SetMerkleAuditEnabled(true)
	attrs3 := map[int32][]byte{1: []byte("ReEnabledAuditValue")}
	if err := fed.UpdateObjectAttributes(obj.Handle, attrs3, [32]byte{}); err != nil {
		t.Fatalf("update with re-enabled audit failed: %v", err)
	}

	rootReEnabled := fed.StateTree().GetGlobalRoot()
	if rootReEnabled == rootWithAudit || rootReEnabled == [32]byte{} {
		t.Fatalf("expected state tree root to change when audit is re-enabled")
	}
}
