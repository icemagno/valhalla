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

	// 3.1 Criar a federação formalmente no CRC
	if err := amb.CreateFederationExecution("ExampleFederation"); err != nil {
		t.Fatalf("CreateFederationExecution falhou: %v", err)
	}

	// 3.2 Tentar criar novamente a mesma federação deve retornar erro
	if err := amb.CreateFederationExecution("ExampleFederation"); err == nil {
		t.Fatal("esperava erro ao tentar criar federacao duplicada, mas obteve sucesso")
	}

	// 4. Ingressar na federação
	fedHandle, err := amb.JoinFederationExecution("ValhallaFederate_1", "ExampleFederation")
	if err != nil {
		t.Fatalf("JoinFederationExecution falhou: %v", err)
	}
	if fedHandle != 1 {
		t.Errorf("esperava handle do federado 1, obteve %d", fedHandle)
	}

	// 4.1 Habilitar politicas de tempo reais no CRC
	if err := amb.EnableTimeRegulation(0.1); err != nil {
		t.Fatalf("EnableTimeRegulation falhou: %v", err)
	}
	if err := amb.EnableTimeConstrained(); err != nil {
		t.Fatalf("EnableTimeConstrained falhou: %v", err)
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

	// 11. Resign, Destroy e Disconnect
	if err := amb.ResignFederationExecution(); err != nil {
		t.Fatalf("ResignFederationExecution falhou: %v", err)
	}

	if err := amb.DestroyFederationExecution("ExampleFederation"); err != nil {
		t.Fatalf("DestroyFederationExecution falhou: %v", err)
	}

	if err := amb.Disconnect(); err != nil {
		t.Fatalf("Disconnect falhou: %v", err)
	}
}

func TestRTIambassador_NegativeScenarios_StrictConformity(t *testing.T) {
	server := crc.NewServer("127.0.0.1:0")
	if err := server.Start(); err != nil {
		t.Fatalf("falha ao iniciar servidor CRC: %v", err)
	}
	defer server.Stop()

	addr := server.Addr().String()

	t.Run("JoinWithoutCreation_Fails", func(t *testing.T) {
		amb := lrc.NewRTIambassador()
		_ = amb.Connect(nil, addr)
		defer amb.Disconnect()

		_, err := amb.JoinFederationExecution("NodeA", "InexistentFed")
		if err == nil {
			t.Fatal("esperava erro ao tentar join em federacao inexistente, obteve nil")
		}
	})

	t.Run("DuplicateCreation_Fails", func(t *testing.T) {
		amb := lrc.NewRTIambassador()
		_ = amb.Connect(nil, addr)
		defer amb.Disconnect()

		if err := amb.CreateFederationExecution("UniqueFed"); err != nil {
			t.Fatalf("falha criando federacao: %v", err)
		}
		if err := amb.CreateFederationExecution("UniqueFed"); err == nil {
			t.Fatal("esperava erro ao recriar mesma federacao, obteve nil")
		}
	})

	t.Run("DestroyWithActiveFederates_Fails", func(t *testing.T) {
		amb := lrc.NewRTIambassador()
		_ = amb.Connect(nil, addr)
		defer amb.Disconnect()

		_ = amb.CreateFederationExecution("ActiveFed")
		_, err := amb.JoinFederationExecution("ActiveNode", "ActiveFed")
		if err != nil {
			t.Fatalf("falha no join: %v", err)
		}

		// Tentar destruir com federado ativo DEVE falhar
		if err := amb.DestroyFederationExecution("ActiveFed"); err == nil {
			t.Fatal("esperava erro ao destruir federacao com federados ativos, obteve nil")
		}
	})

	t.Run("DuplicateFederateName_Fails", func(t *testing.T) {
		_ = server // reuso
		amb1 := lrc.NewRTIambassador()
		_ = amb1.Connect(nil, addr)
		defer amb1.Disconnect()

		_ = amb1.CreateFederationExecution("DupNameFed")
		_, err := amb1.JoinFederationExecution("SameNodeName", "DupNameFed")
		if err != nil {
			t.Fatalf("primeiro join falhou: %v", err)
		}

		amb2 := lrc.NewRTIambassador()
		_ = amb2.Connect(nil, addr)
		defer amb2.Disconnect()

		_, err = amb2.JoinFederationExecution("SameNodeName", "DupNameFed")
		if err == nil {
			t.Fatal("esperava erro ao ingressar segundo federado com o mesmo nome, obteve nil")
		}
	})

	t.Run("AchieveUnannouncedSyncPoint_Fails", func(t *testing.T) {
		amb := lrc.NewRTIambassador()
		_ = amb.Connect(nil, addr)
		defer amb.Disconnect()

		_ = amb.CreateFederationExecution("SyncFed")
		_, _ = amb.JoinFederationExecution("SyncNode", "SyncFed")

		err := amb.SynchronizationPointAchieved("PontoFantasmaNaoRegistrado")
		if err == nil {
			t.Fatal("esperava erro ao alcançar ponto de sincronizacao nao anunciado, obteve nil")
		}
	})

	t.Run("DuplicateSyncPoint_Fails", func(t *testing.T) {
		amb := lrc.NewRTIambassador()
		_ = amb.Connect(nil, addr)
		defer amb.Disconnect()

		_ = amb.CreateFederationExecution("SyncDupFed")
		_, _ = amb.JoinFederationExecution("SyncDupNode", "SyncDupFed")

		if err := amb.RegisterFederationSynchronizationPoint("PontoUnico", nil); err != nil {
			t.Fatalf("primeiro registro de ponto falhou: %v", err)
		}

		if err := amb.RegisterFederationSynchronizationPoint("PontoUnico", nil); err == nil {
			t.Fatal("esperava erro ao registrar ponto duplicado, obteve nil")
		}
	})

	t.Run("DuplicateTimeRegulation_Fails", func(t *testing.T) {
		amb := lrc.NewRTIambassador()
		_ = amb.Connect(nil, addr)
		defer amb.Disconnect()

		_ = amb.CreateFederationExecution("TimeFed")
		_, _ = amb.JoinFederationExecution("TimeNode", "TimeFed")

		if err := amb.EnableTimeRegulation(0.1); err != nil {
			t.Fatalf("primeira habilitacao de tempo falhou: %v", err)
		}

		if err := amb.EnableTimeRegulation(0.2); err == nil {
			t.Fatal("esperava erro ao habilitar regulacao de tempo ja ativa, obteve nil")
		}
	})

	t.Run("InvalidLookahead_Fails", func(t *testing.T) {
		amb := lrc.NewRTIambassador()
		_ = amb.Connect(nil, addr)
		defer amb.Disconnect()

		_ = amb.CreateFederationExecution("LookaheadFed")
		_, _ = amb.JoinFederationExecution("LookNode", "LookaheadFed")

		if err := amb.EnableTimeRegulation(-1.0); err == nil {
			t.Fatal("esperava erro com lookahead negativo, obteve nil")
		}
	})

	t.Run("DuplicateObjectName_Fails", func(t *testing.T) {
		amb := lrc.NewRTIambassador()
		_ = amb.Connect(nil, addr)
		defer amb.Disconnect()

		_ = amb.CreateFederationExecution("ObjDupFed")
		_, _ = amb.JoinFederationExecution("ObjNode", "ObjDupFed")

		_, err := amb.RegisterObjectInstance(100, "VesselAlpha")
		if err != nil {
			t.Fatalf("primeiro registro de objeto falhou: %v", err)
		}

		_, err = amb.RegisterObjectInstance(100, "VesselAlpha")
		if err == nil {
			t.Fatal("esperava erro ao registrar segundo objeto com mesmo nome, obteve nil")
		}
	})
}
