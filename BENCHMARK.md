# Benchmarks e Métricas de Desempenho do Projeto Valhalla

> **Princípio Metodológico:** Jamais comparar microbenchmarks internos de CPU isolados contra latências de rede fim-a-fim. O Projeto Valhalla divide seus resultados em **duas camadas estritas**:

O relatório completo, com o embasamento científico (Gütlein et al., 2020 / SISO), as condições laboratoriais e as instruções para reprodução científica está documentado em:

👉 **[docs/benchmark.md](docs/benchmark.md)**

---

### Camada 1: Microbenchmarks Algorítmicos em Memória (CPU Puro)
Mede o custo isolado das estruturas de dados sem interferência de I/O de rede ou do sistema operacional:

```text
========================================================================================
MICROBENCHMARKS INTERNOS DO MOTOR (CPU / MEMÓRIA) | Go 1.24
========================================================================================
BenchmarkOMTRegistry_IsSubclassOfObject-16        76.062.498 ops     15.40 ns/op
BenchmarkOMTRegistry_ResolveAttribute-16          55.485.427 ops     23.24 ns/op
BenchmarkBitset_Union-16 (1.000 federados)        32.838.655 ops     33.74 ns/op
BenchmarkBitset_ToFederateHandles-16               8.596.258 ops    140.90 ns/op
BenchmarkInterestMatcher_GetAttributeSubscribers   4.349.264 ops    275.70 ns/op
BenchmarkPayload_UpdateAttributes-16               2.906.976 ops    344.00 ns/op
========================================================================================
```

---

### Camada 2: Benchmark de Rede Fim-a-Fim Real (TCP Sockets / Pilha Completa)
Mede o ciclo de vida real idêntico aos testes da SISO: Publicador -> LRC -> TCP -> CRC -> Router -> TCP -> LRC -> Assinante (`UpdateAttributeValues` $\to$ `ReflectAttributeValues`):

```text
========================================================================================
BENCHMARK FIM-A-FIM REAL (PILHA COMPLETA DE REDE TCP / SOCKETS REAIS)
Amostras Coletadas: 31.543 iterações consecutivas | test/benchmark
========================================================================================
Latência Média Fim-a-Fim:       89,67 µs (0,089 milissegundos)
Latência Mediana (P50):         < 1,00 µs (chegada imediata em buffer)
Latência P90:                   507,80 µs (0,507 milissegundos)
Latência P99 (Pior Caso):     1.027,10 µs (1,027 milissegundos)
Throughput Síncrono E2E:       ~11.146 round-trips completos/segundo por conexão
========================================================================================
```

### Linha de Base Comparativa Fim-a-Fim (Mesmo Tipo de Medição):
* **Portico RTI:** 1.000 µs a 60.000 µs (1 a 60 ms) com alta oscilação por pausas de GC da JVM.
* **Pitch pRTI:** 300 µs a 800 µs (0,3 a 0,8 ms).
* **Valhalla RTI (Nativo):** **89,67 µs (0,089 ms)** de latência média real fim-a-fim sobre TCP.

---

### Decomposição de Latência e Avaliação do Runtime (Go vs. Rust/C++):
* **Sobrecarga Interna de Software do Valhalla:** **0,66 µs** (~0,7% do orçamento de tempo).
* **Sobrecarga de Pilha de Rede TCP / Kernel do SO:** **89,01 µs** (99,3% do tempo).
* Pela **Lei de Amdahl**, o gargalo reside no tráfego de rede e no kernel do sistema operacional, e não no runtime Go. O design de zero-alocação do Valhalla no hot path elimina pausas de GC, atingindo determinismo temporal com produtividade de desenvolvimento superior. Detalhamento analítico em [docs/benchmark.md](docs/benchmark.md#6-decomposição-analítica-da-latência-e-avaliação-do-runtime-go-vs-rust--c).
