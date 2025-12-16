// +build mixed_benchmark

package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tensorthoughts25/allidb/core/cluster"
	"github.com/tensorthoughts25/allidb/core/cluster/ring"
	"github.com/tensorthoughts25/allidb/core/observability"
)

var (
	concurrency = flag.Int("concurrency", 10, "Number of concurrent workers")
	duration    = flag.Duration("duration", 60*time.Second, "Benchmark duration")
	readRatio   = flag.Float64("read-ratio", 0.7, "Ratio of read operations (0.0-1.0)")
	keyCount    = flag.Int("key-count", 10000, "Number of unique keys")
	keySize     = flag.Int("key-size", 32, "Size of keys in bytes")
	valueSize   = flag.Int("value-size", 1024, "Size of values in bytes")
	verbose     = flag.Bool("verbose", false, "Verbose output")
)

func main() {
	flag.Parse()

	// Validate read ratio
	if *readRatio < 0.0 || *readRatio > 1.0 {
		fmt.Fprintf(os.Stderr, "read-ratio must be between 0.0 and 1.0\n")
		os.Exit(1)
	}

	// Setup mock coordinator with pre-populated data
	coordinator, keys := setupMockCoordinatorWithData(*keyCount)

	// Setup metrics
	registry := observability.DefaultMetricsRegistry()
	writesCounter := registry.Counter("writes_total", nil)
	readsCounter := registry.Counter("reads_total", nil)
	writeErrorsCounter := registry.Counter("write_errors_total", nil)
	readErrorsCounter := registry.Counter("read_errors_total", nil)
	writeLatencyHistogram := registry.Histogram("write_latency_ms", nil, []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000})
	readLatencyHistogram := registry.Histogram("read_latency_ms", nil, []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000})

	// Setup logger
	logger := observability.DefaultLogger()
	logger.Info("Starting mixed RAG workload benchmark", observability.Fields{
		"concurrency": *concurrency,
		"duration":    duration.String(),
		"read_ratio":  *readRatio,
		"key_count":   *keyCount,
		"key_size":    *keySize,
		"value_size":  *valueSize,
	})

	// Run benchmark
	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()

	var totalWrites int64
	var totalReads int64
	var totalWriteErrors int64
	var totalReadErrors int64
	var wg sync.WaitGroup

	startTime := time.Now()

	// Start concurrent workers
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
					// Determine operation based on read ratio
					if rng.Float64() < *readRatio {
						// Read operation
						key := keys[rng.Intn(len(keys))]

						readStart := time.Now()
						_, err := coordinator.Read(ctx, key)
						latency := time.Since(readStart)

						if err != nil {
							atomic.AddInt64(&totalReadErrors, 1)
							readErrorsCounter.Inc()
							if *verbose {
								logger.Error("Read failed", err, observability.Fields{
									"key": key,
								})
							}
						} else {
							atomic.AddInt64(&totalReads, 1)
							readsCounter.Inc()
							readLatencyHistogram.Observe(float64(latency.Milliseconds()))
						}
					} else {
						// Write operation
						key := generateKey(keyPrefix, rng, *keySize)
						value := generateValue(rng, *valueSize)

						writeStart := time.Now()
						err := coordinator.Write(ctx, key, value)
						latency := time.Since(writeStart)

						if err != nil {
							atomic.AddInt64(&totalWriteErrors, 1)
							writeErrorsCounter.Inc()
							if *verbose {
								logger.Error("Write failed", err, observability.Fields{
									"key": key,
								})
							}
						} else {
							atomic.AddInt64(&totalWrites, 1)
							writesCounter.Inc()
							writeLatencyHistogram.Observe(float64(latency.Milliseconds()))
						}
					}
				}
			}
		}(i)
	}

	// Wait for all workers to finish
	wg.Wait()

	elapsed := time.Since(startTime)

	// Print results
	fmt.Printf("\n=== Mixed RAG Workload Benchmark Results ===\n")
	fmt.Printf("Duration: %v\n", elapsed)
	fmt.Printf("Read Ratio: %.1f%%\n", *readRatio*100)
	fmt.Printf("Total Reads: %d\n", atomic.LoadInt64(&totalReads))
	fmt.Printf("Total Writes: %d\n", atomic.LoadInt64(&totalWrites))
	fmt.Printf("Total Read Errors: %d\n", atomic.LoadInt64(&totalReadErrors))
	fmt.Printf("Total Write Errors: %d\n", atomic.LoadInt64(&totalWriteErrors))
	fmt.Printf("Read Throughput: %.2f reads/sec\n", float64(atomic.LoadInt64(&totalReads))/elapsed.Seconds())
	fmt.Printf("Write Throughput: %.2f writes/sec\n", float64(atomic.LoadInt64(&totalWrites))/elapsed.Seconds())
	fmt.Printf("Total Throughput: %.2f ops/sec\n", float64(atomic.LoadInt64(&totalReads)+atomic.LoadInt64(&totalWrites))/elapsed.Seconds())
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
	if len(key) > size {
		return key[:size]
	}
	return key
}

func generateValue(rng *rand.Rand, size int) []byte {
	value := make([]byte, size)
	rng.Read(value)
	return value
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

