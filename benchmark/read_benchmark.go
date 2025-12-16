// +build read_benchmark

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

var (
	concurrency = flag.Int("concurrency", 10, "Number of concurrent readers")
	duration    = flag.Duration("duration", 30*time.Second, "Benchmark duration")
	keyCount    = flag.Int("key-count", 10000, "Number of unique keys to read from")
	verbose     = flag.Bool("verbose", false, "Verbose output")
)

func main() {
	flag.Parse()

	// Setup mock coordinator with pre-populated data
	coordinator, keys := setupMockCoordinatorWithData(*keyCount)

	// Setup metrics
	registry := observability.DefaultMetricsRegistry()
	readsCounter := registry.Counter("reads_total", nil)
	errorsCounter := registry.Counter("read_errors_total", nil)
	latencyHistogram := registry.Histogram("read_latency_ms", nil, []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000})

	// Setup logger
	logger := observability.DefaultLogger()
	logger.Info("Starting read-only benchmark", observability.Fields{
		"concurrency": *concurrency,
		"duration":    duration.String(),
		"key_count":   *keyCount,
	})

	// Run benchmark
	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()

	var totalReads int64
	var totalErrors int64
	var wg sync.WaitGroup

	startTime := time.Now()

	// Start concurrent readers
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID)))

			for {
				select {
				case <-ctx.Done():
					return
				default:
					// Select random key
					key := keys[rng.Intn(len(keys))]

					// Measure latency
					readStart := time.Now()
					_, err := coordinator.Read(ctx, key)
					latency := time.Since(readStart)

					if err != nil {
						atomic.AddInt64(&totalErrors, 1)
						errorsCounter.Inc()
						if *verbose {
							logger.Error("Read failed", err, observability.Fields{
								"key": key,
							})
						}
					} else {
						atomic.AddInt64(&totalReads, 1)
						readsCounter.Inc()
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
	fmt.Printf("\n=== Read-Only Benchmark Results ===\n")
	fmt.Printf("Duration: %v\n", elapsed)
	fmt.Printf("Total Reads: %d\n", atomic.LoadInt64(&totalReads))
	fmt.Printf("Total Errors: %d\n", atomic.LoadInt64(&totalErrors))
	fmt.Printf("Throughput: %.2f reads/sec\n", float64(atomic.LoadInt64(&totalReads))/elapsed.Seconds())
	fmt.Printf("Concurrency: %d\n", *concurrency)
	fmt.Printf("\n")

	// Print metrics
	printMetrics(registry)
}

func setupMockCoordinatorWithData(keyCount int) (*cluster.Coordinator, []string) {
	// Create a mock ring
	r := ring.New(ring.DefaultConfig())
	r.AddNode(ring.NodeID("node1"))
	r.AddNode(ring.NodeID("node2"))
	r.AddNode(ring.NodeID("node3"))

	// Create mock replica client
	client := &MockReplicaClient{
		data: make(map[cluster.NodeID]map[string][]byte),
	}

	// Pre-populate with data
	keys := make([]string, 0, keyCount)
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < keyCount; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := make([]byte, 1024)
		rng.Read(value)

		// Write to all nodes
		for _, nodeID := range []cluster.NodeID{"node1", "node2", "node3"} {
			client.Write(context.Background(), nodeID, key, value)
		}
		keys = append(keys, key)
	}

	// Create coordinator
	coordinator := cluster.NewCoordinator(r, nil, client, cluster.DefaultConfig())
	return coordinator, keys
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

