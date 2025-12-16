// +build write_benchmark

package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tensorthoughts25/allidb/core/cluster"
	"github.com/tensorthoughts25/allidb/core/cluster/ring"
	"github.com/tensorthoughts25/allidb/core/observability"
)

var (
	concurrency = flag.Int("concurrency", 10, "Number of concurrent writers")
	duration    = flag.Duration("duration", 30*time.Second, "Benchmark duration")
	keySize     = flag.Int("key-size", 32, "Size of keys in bytes")
	valueSize   = flag.Int("value-size", 1024, "Size of values in bytes")
	verbose     = flag.Bool("verbose", false, "Verbose output")
)

func main() {
	flag.Parse()

	// Setup mock coordinator for benchmarking
	coordinator := setupMockCoordinator()

	// Setup metrics
	registry := observability.DefaultMetricsRegistry()
	writesCounter := registry.Counter("writes_total", nil)
	errorsCounter := registry.Counter("write_errors_total", nil)
	latencyHistogram := registry.Histogram("write_latency_ms", nil, []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000})

	// Setup logger
	logger := observability.DefaultLogger()
	logger.Info("Starting write-only benchmark", observability.Fields{
		"concurrency": *concurrency,
		"duration":    duration.String(),
		"key_size":    *keySize,
		"value_size":  *valueSize,
	})

	// Run benchmark
	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()

	var totalWrites int64
	var totalErrors int64
	var wg sync.WaitGroup

	startTime := time.Now()

	// Start concurrent writers
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID)))
			keyPrefix := fmt.Sprintf("key-%d-", workerID)

			for {
				select {
				case <-ctx.Done():
					return
				default:
					// Generate random key and value
					key := generateKey(keyPrefix, rng, *keySize)
					value := generateValue(rng, *valueSize)

					// Measure latency
					writeStart := time.Now()
					err := coordinator.Write(ctx, key, value)
					latency := time.Since(writeStart)

					if err != nil {
						atomic.AddInt64(&totalErrors, 1)
						errorsCounter.Inc()
						if *verbose {
							logger.Error("Write failed", err, observability.Fields{
								"key": key,
							})
						}
					} else {
						atomic.AddInt64(&totalWrites, 1)
						writesCounter.Inc()
						latencyHistogram.Observe(float64(latency.Milliseconds()))
					}
				}
			}
		}(i)
	}

	// Wait for all workers to finish
	wg.Wait()

	elapsed := time.Since(startTime)

	// Print results
	fmt.Printf("\n=== Write-Only Benchmark Results ===\n")
	fmt.Printf("Duration: %v\n", elapsed)
	fmt.Printf("Total Writes: %d\n", atomic.LoadInt64(&totalWrites))
	fmt.Printf("Total Errors: %d\n", atomic.LoadInt64(&totalErrors))
	fmt.Printf("Throughput: %.2f writes/sec\n", float64(atomic.LoadInt64(&totalWrites))/elapsed.Seconds())
	fmt.Printf("Concurrency: %d\n", *concurrency)
	fmt.Printf("\n")

	// Print metrics
	printMetrics(registry)
}

func generateKey(prefix string, rng *rand.Rand, size int) string {
	key := prefix
	remaining := size - len(key)
	if remaining > 0 {
		chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		for i := 0; i < remaining; i++ {
			key += string(chars[rng.Intn(len(chars))])
		}
	}
	return key[:size]
}

func generateValue(rng *rand.Rand, size int) []byte {
	value := make([]byte, size)
	rng.Read(value)
	return value
}

func setupMockCoordinator() *cluster.Coordinator {
	// Create a mock ring
	r := ring.New(ring.DefaultConfig())
	r.AddNode(ring.NodeID("node1"))
	r.AddNode(ring.NodeID("node2"))
	r.AddNode(ring.NodeID("node3"))

	// Create mock replica client
	client := &MockReplicaClient{
		data: make(map[cluster.NodeID]map[string][]byte),
	}

	// Create coordinator
	coordinator := cluster.NewCoordinator(r, nil, client, cluster.DefaultConfig())
	return coordinator
}

// MockReplicaClient is a simple in-memory mock for benchmarking.
type MockReplicaClient struct {
	mu   sync.RWMutex
	data map[cluster.NodeID]map[string][]byte
}

func (m *MockReplicaClient) Read(ctx context.Context, nodeID cluster.NodeID, key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.data[nodeID] != nil {
		return m.data[nodeID][key], nil
	}
	return nil, nil
}

func (m *MockReplicaClient) Write(ctx context.Context, nodeID cluster.NodeID, key string, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data[nodeID] == nil {
		m.data[nodeID] = make(map[string][]byte)
	}
	m.data[nodeID][key] = value
	return nil
}

func printMetrics(registry *observability.MetricsRegistry) {
	metrics := registry.GetAllMetrics()
	fmt.Printf("=== Metrics ===\n")
	for _, metric := range metrics {
		fmt.Printf("%s: %.2f", metric.Name, metric.Value)
		if len(metric.Labels) > 0 {
			fmt.Printf(" (labels: %v)", metric.Labels)
		}
		fmt.Printf("\n")
	}
}

