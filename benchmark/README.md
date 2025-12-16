## Benchmarks

Three standalone benchmarks are provided under `benchmark/` (build-tagged to avoid normal builds):

- `write_benchmark.go` (tag `write_benchmark`): write-only workload against a mock coordinator.
- `read_benchmark.go` (tag `read_benchmark`): read-only workload.
- `mixed_rag_benchmark.go` (tag `mixed_benchmark`): mixed vector + graph style workload.

### How to run

From repo root:

- Write-only:  
  `go run -tags write_benchmark ./benchmark/write_benchmark.go -concurrency=32 -duration=30s -key-size=32 -value-size=1024`

- Read-only:  
  `go run -tags read_benchmark ./benchmark/read_benchmark.go -concurrency=64 -duration=30s -key-size=32`

- Mixed (RAG-like):  
  `go run -tags mixed_benchmark ./benchmark/mixed_rag_benchmark.go -concurrency=32 -duration=30s -vector-dim=1536 -graph-fanout=4`

All benchmarks print throughput, error counts, and basic latency histograms via the in-process metrics registry.

### Sample data

`benchmark/data/sample_text.txt` contains a small corpus you can use to seed test entities or embeddings when experimenting locally.


