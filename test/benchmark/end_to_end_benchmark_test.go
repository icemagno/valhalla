package benchmark_test

import (
	"encoding/binary"
	"sort"
	"testing"
	"time"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc"
	"github.com/icemagno/valhalla/pkg/lrc"
)

type latencyAmbassador struct {
	lrc.NullFederateAmbassador
	chReceive chan int64
}

func (a *latencyAmbassador) ReflectAttributeValues(theObject int32, theAttributes map[int32][]byte, tag []byte) {
	if len(tag) >= 8 {
		sentNano := int64(binary.BigEndian.Uint64(tag[0:8]))
		select {
		case a.chReceive <- sentNano:
		default:
		}
	}
}

// BenchmarkEndToEnd_UpdateAttributeValues_Latency mede o tempo fim-a-fim real (RTT)
// através de sockets TCP reais: Publicador -> LRC -> TCP -> CRC -> TCP -> LRC -> Assinante.
func BenchmarkEndToEnd_UpdateAttributeValues_Latency(b *testing.B) {
	// 1. Iniciar servidor CRC autônomo sobre TCP
	server := crc.NewServer("127.0.0.1:0")
	if err := server.Start(); err != nil {
		b.Fatalf("falha ao iniciar servidor CRC: %v", err)
	}
	defer server.Stop()

	crcAddr := server.Addr().String()

	// 2. Criar federação
	ambAdmin := lrc.NewRTIambassador()
	_ = ambAdmin.Connect(nil, crcAddr)
	_ = ambAdmin.CreateFederationExecution("E2E_BenchmarkFed")
	_ = ambAdmin.Disconnect()

	// 3. Conectar Federado Publicador
	ambPub := lrc.NewRTIambassador()
	if err := ambPub.Connect(nil, crcAddr); err != nil {
		b.Fatalf("falha ao conectar Publisher: %v", err)
	}
	defer ambPub.Disconnect()
	_, _ = ambPub.JoinFederationExecution("E2E_Publisher", "E2E_BenchmarkFed")

	// 4. Conectar Federado Assinante
	chRecv := make(chan int64, 1000)
	ambSubCallback := &latencyAmbassador{chReceive: chRecv}
	ambSub := lrc.NewRTIambassador()
	if err := ambSub.Connect(ambSubCallback, crcAddr); err != nil {
		b.Fatalf("falha ao conectar Subscriber: %v", err)
	}
	defer ambSub.Disconnect()
	_, _ = ambSub.JoinFederationExecution("E2E_Subscriber", "E2E_BenchmarkFed")

	// 5. Configurar assinaturas e publicações
	_ = ambSub.SubscribeObjectClassAttributes(
		int32(common.HLAobjectRootHandle),
		[]int32{int32(common.HLAprivilegeToDeleteObjectHandle)},
	)
	_ = ambPub.PublishObjectClassAttributes(
		int32(common.HLAobjectRootHandle),
		[]int32{int32(common.HLAprivilegeToDeleteObjectHandle)},
	)

	// 6. Registrar objeto
	objHandle, err := ambPub.RegisterObjectInstance(int32(common.HLAobjectRootHandle), "E2E_TargetEntity")
	if err != nil {
		b.Fatalf("falha ao registrar objeto: %v", err)
	}

	// Drenar callback de descoberta inicial
	time.Sleep(30 * time.Millisecond)
	_, _ = ambSub.EvokeCallback(50 * time.Millisecond)

	// Iniciar loop de leitura de callbacks em background no Subscriber
	stopSub := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopSub:
				return
			default:
				_, _ = ambSub.EvokeCallback(10 * time.Millisecond)
			}
		}
	}()
	defer close(stopSub)

	attrs := map[int32][]byte{
		int32(common.HLAprivilegeToDeleteObjectHandle): []byte("TelemetryDataPayloadSample_64BytesPaddingToSimulateRPRFOMEntityState"),
	}

	var tagBuf [8]byte
	latencies := make([]time.Duration, 0, b.N)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nowNano := time.Now().UnixNano()
		binary.BigEndian.PutUint64(tagBuf[0:8], uint64(nowNano))

		if err := ambPub.UpdateAttributeValues(objHandle, attrs, tagBuf[:]); err != nil {
			b.Fatalf("falha no UpdateAttributeValues na iteração %d: %v", i, err)
		}

		select {
		case sentNano := <-chRecv:
			elapsed := time.Duration(time.Now().UnixNano() - sentNano)
			latencies = append(latencies, elapsed)
		case <-time.After(1 * time.Second):
			b.Fatalf("timeout aguardando reflexão de atributo na iteração %d", i)
		}
	}
	b.StopTimer()

	if len(latencies) > 0 {
		var total time.Duration
		for _, l := range latencies {
			total += l
		}
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		avg := total / time.Duration(len(latencies))
		p50 := latencies[len(latencies)*50/100]
		p90 := latencies[len(latencies)*90/100]
		p99 := latencies[len(latencies)*99/100]
		b.Logf("\n--- RESULTADOS FIM-A-FIM REAIS (TCP Network Stack) ---")
		b.Logf("Amostras: %d", len(latencies))
		b.Logf("Latência Média: %v (%.2f µs)", avg, float64(avg.Nanoseconds())/1000.0)
		b.Logf("Latência P50:   %v (%.2f µs)", p50, float64(p50.Nanoseconds())/1000.0)
		b.Logf("Latência P90:   %v (%.2f µs)", p90, float64(p90.Nanoseconds())/1000.0)
		b.Logf("Latência P99:   %v (%.2f µs)", p99, float64(p99.Nanoseconds())/1000.0)
	}
}
