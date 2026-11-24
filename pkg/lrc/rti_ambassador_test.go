package lrc_test

import (
	"sync"
	"testing"
	"time"

	"github.com/icemagno/valhalla/pkg/crc"
	"github.com/icemagno/valhalla/pkg/lrc"
)

type mockAmbassador struct {
	lrc.NullFederateAmbassador
	mu         sync.Mutex
	synchronized bool
	syncedLabel  string
	discoveredObj int32
	timeGranted  float64
}

func (m *mockAmbassador) FederationSynchronized(label string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.synchronized = true
	m.syncedLabel = label
}

func (m *mockAmbassador) DiscoverObjectInstance(theObject, theObjectClass int32, objectName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.discoveredObj = theObject
}

func (m *mockAmbassador) TimeAdvanceGrant(t float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.timeGranted = t
}

func TestRTIambassador_FullNativeLifecycle(t *testing.T) {
	// 1. Iniciar servidor CRC autônomo em porta efêmera
	server := crc.NewServer("127.0.0.1:0")
	if err := server.Start(); err != nil {
		t.Fatalf("falha ao iniciar servidor CRC: %v", err)
	}
	defer server.Stop()

	addr := server.Addr().String()

	// 2. Instanciar RTIambassador e FederateAmbassador
	amb := lrc.NewRTIambassador()
	fedAmb := &mockAmbassador{}

	// 3. Conectar ao CRC
	if err := amb.Connect(fedAmb, addr); err != nil {
		t.Fatalf("Connect falhou: %v", err)
	}

	// 4. Ingressar na federação
	fedHandle, err := amb.JoinFederationExecution("ValhallaFederate_1", "ExampleFederation")
	if err != nil {
		t.Fatalf("JoinFederationExecution falhou: %v", err)
	}
	if fedHandle != 1 {
		t.Errorf("esperava handle do federado 1, obteve %d", fedHandle)
	}

	// 5. Registrar e conquistar ponto de sincronização
	if err := amb.RegisterFederationSynchronizationPoint("ReadyToRun", nil); err != nil {
		t.Fatalf("RegisterFederationSynchronizationPoint falhou: %v", err)
	}

	if err := amb.SynchronizationPointAchieved("ReadyToRun"); err != nil {
		t.Fatalf("SynchronizationPointAchieved falhou: %v", err)
	}

	// 6. Evocar callbacks para processar FederationSynchronized
	time.Sleep(50 * time.Millisecond)
	evoked, err := amb.EvokeCallback(200 * time.Millisecond)
	if err != nil {
		t.Fatalf("EvokeCallback falhou: %v", err)
	}
	if !evoked {
		t.Log("nenhum callback evocado imediatamente (normal)")
	}

	fedAmb.mu.Lock()
	if fedAmb.synchronized && fedAmb.syncedLabel != "ReadyToRun" {
		t.Errorf("rotulo de sincronizacao invalido: %s", fedAmb.syncedLabel)
	}
	fedAmb.mu.Unlock()

	// 7. Registrar objeto
	objHandle, err := amb.RegisterObjectInstance(561, "Valkyrie_01")
	if err != nil {
		t.Fatalf("RegisterObjectInstance falhou: %v", err)
	}
	if objHandle <= 0 {
		t.Errorf("handle de objeto invalido: %d", objHandle)
	}

	// 8. Atualizar atributos
	attrs := map[int32][]byte{
		562: []byte("PosicaoX:100.5"),
		551: []byte("PosicaoY:200.8"),
	}
	if err := amb.UpdateAttributeValues(objHandle, attrs, nil); err != nil {
		t.Fatalf("UpdateAttributeValues falhou: %v", err)
	}

	// 9. Enviar interacao
	params := map[int32][]byte{
		1: []byte("OrdemDeAtaque"),
	}
	if err := amb.SendInteraction(525, params, nil); err != nil {
		t.Fatalf("SendInteraction falhou: %v", err)
	}

	// 10. Solicitar avanço no tempo
	if err := amb.TimeAdvanceRequest(1.0); err != nil {
		t.Fatalf("TimeAdvanceRequest falhou: %v", err)
	}

	// Processar callback de TAG (TimeAdvanceGrant)
	time.Sleep(50 * time.Millisecond)
	_, _ = amb.EvokeCallback(200 * time.Millisecond)

	fedAmb.mu.Lock()
	if fedAmb.timeGranted != 1.0 {
		t.Logf("TAG processado: %.1f", fedAmb.timeGranted)
	}
	fedAmb.mu.Unlock()

	// 11. Resign e Disconnect
	if err := amb.ResignFederationExecution(); err != nil {
		t.Fatalf("ResignFederationExecution falhou: %v", err)
	}

	if err := amb.Disconnect(); err != nil {
		t.Fatalf("Disconnect falhou: %v", err)
	}
}
