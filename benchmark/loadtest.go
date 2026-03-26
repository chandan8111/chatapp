package benchmark

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// BenchmarkConfig holds configuration for benchmarking
type BenchmarkConfig struct {
	TargetURL           string
	ConcurrentUsers     int
	Duration            time.Duration
	RampUpTime          time.Duration
	MessageInterval     time.Duration
	MessageSize         int
	EnableMetrics       bool
	EnableLatencyStats  bool
}

// BenchmarkResult holds the results of a benchmark run
type BenchmarkResult struct {
	TotalConnections    int64
	SuccessfulMessages  int64
	FailedMessages      int64
	TotalDuration       time.Duration
	AvgLatency          time.Duration
	P50Latency          time.Duration
	P95Latency          time.Duration
	P99Latency          time.Duration
	MaxLatency          time.Duration
	MessagesPerSecond   float64
	ConnectionsPerSecond float64
	Errors              []error
}

// LoadTester represents a load testing instance
type LoadTester struct {
	config     *BenchmarkConfig
	logger     *zap.Logger
	results    *BenchmarkResult
	latencies  []time.Duration
	latMu      sync.RWMutex
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

// NewLoadTester creates a new load tester
func NewLoadTester(config *BenchmarkConfig, logger *zap.Logger) *LoadTester {
	return &LoadTester{
		config:    config,
		logger:    logger,
		results:   &BenchmarkResult{},
		latencies: make([]time.Duration, 0, 100000),
		stopCh:    make(chan struct{}),
	}
}

// Run starts the load test
func (lt *LoadTester) Run() *BenchmarkResult {
	lt.logger.Info("Starting load test",
		zap.Int("concurrent_users", lt.config.ConcurrentUsers),
		zap.Duration("duration", lt.config.Duration),
		zap.String("target", lt.config.TargetURL),
	)

	startTime := time.Now()
	
	// Ramp up connections
	lt.rampUpConnections()
	
	// Wait for test duration
	time.Sleep(lt.config.Duration)
	
	// Stop all workers
	close(lt.stopCh)
	lt.wg.Wait()
	
	// Calculate results
	lt.results.TotalDuration = time.Since(startTime)
	lt.calculateResults()
	
	lt.logger.Info("Load test completed",
		zap.Int64("total_connections", lt.results.TotalConnections),
		zap.Int64("successful_messages", lt.results.SuccessfulMessages),
		zap.Float64("messages_per_second", lt.results.MessagesPerSecond),
	)
	
	return lt.results
}

// rampUpConnections gradually establishes connections
func (lt *LoadTester) rampUpConnections() {
	connectionsPerSecond := float64(lt.config.ConcurrentUsers) / lt.config.RampUpTime.Seconds()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	
	created := 0
	startTime := time.Now()
	
	for created < lt.config.ConcurrentUsers {
		select {
		case <-ticker.C:
			target := int(connectionsPerSecond * time.Since(startTime).Seconds())
			if target > lt.config.ConcurrentUsers {
				target = lt.config.ConcurrentUsers
			}
			
			toCreate := target - created
			for i := 0; i < toCreate; i++ {
				lt.wg.Add(1)
				go lt.worker(created + i)
			}
			created = target
		}
	}
}

// worker simulates a single user
func (lt *LoadTester) worker(id int) {
	defer lt.wg.Done()
	
	userID := fmt.Sprintf("benchmark-user-%d", id)
	deviceID := fmt.Sprintf("benchmark-device-%d", id)
	
	// Connect to WebSocket
	wsURL := fmt.Sprintf("%s/ws?user_id=%s&device_id=%s&node_id=benchmark", 
		lt.config.TargetURL, userID, deviceID)
	
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		lt.logger.Error("Failed to connect", 
			zap.String("user_id", userID),
			zap.Error(err),
		)
		return
	}
	defer conn.Close()
	
	atomic.AddInt64(&lt.results.TotalConnections, 1)
	
	// Message loop
	ticker := time.NewTicker(lt.config.MessageInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-lt.stopCh:
			return
		case <-ticker.C:
			lt.sendMessage(conn, userID)
		}
	}
}

// sendMessage sends a message and measures latency
func (lt *LoadTester) sendMessage(conn *websocket.Conn, userID string) {
	start := time.Now()
	
	// Create benchmark message
	message := map[string]interface{}{
		"type":       "benchmark",
		"sender_id":  userID,
		"timestamp":  time.Now().UnixNano(),
		"content":    lt.generateRandomContent(),
		"message_id": fmt.Sprintf("msg-%d", time.Now().UnixNano()),
	}
	
	if err := conn.WriteJSON(message); err != nil {
		atomic.AddInt64(&lt.results.FailedMessages, 1)
		lt.logger.Error("Failed to send message",
			zap.String("user_id", userID),
			zap.Error(err),
		)
		return
	}
	
	latency := time.Since(start)
	
	atomic.AddInt64(&lt.results.SuccessfulMessages, 1)
	
	// Record latency
	if lt.config.EnableLatencyStats {
		lt.latMu.Lock()
		lt.latencies = append(lt.latencies, latency)
		lt.latMu.Unlock()
	}
}

// generateRandomContent generates random message content
func (lt *LoadTester) generateRandomContent() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, lt.config.MessageSize)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// calculateResults calculates final statistics
func (lt *LoadTester) calculateResults() {
	// Messages per second
	lt.results.MessagesPerSecond = float64(lt.results.SuccessfulMessages) / lt.results.TotalDuration.Seconds()
	
	// Connections per second
	lt.results.ConnectionsPerSecond = float64(lt.results.TotalConnections) / lt.config.RampUpTime.Seconds()
	
	// Calculate latency percentiles
	if lt.config.EnableLatencyStats && len(lt.latencies) > 0 {
		lt.calculateLatencyStats()
	}
}

// calculateLatencyStats calculates latency statistics
func (lt *LoadTester) calculateLatencyStats() {
	lt.latMu.RLock()
	defer lt.latMu.RUnlock()
	
	if len(lt.latencies) == 0 {
		return
	}
	
	// Sort latencies
	sorted := make([]time.Duration, len(lt.latencies))
	copy(sorted, lt.latencies)
	
	// Quick sort implementation for durations
	quickSortDuration(sorted)
	
	// Calculate statistics
	n := len(sorted)
	total := time.Duration(0)
	for _, d := range sorted {
		total += d
	}
	
	lt.results.AvgLatency = total / time.Duration(n)
	lt.results.P50Latency = sorted[n*50/100]
	lt.results.P95Latency = sorted[n*95/100]
	lt.results.P99Latency = sorted[n*99/100]
	lt.results.MaxLatency = sorted[n-1]
}

// quickSortDuration sorts durations using quicksort
func quickSortDuration(arr []time.Duration) {
	if len(arr) <= 1 {
		return
	}
	
	quickSortDurationHelper(arr, 0, len(arr)-1)
}

func quickSortDurationHelper(arr []time.Duration, low, high int) {
	if low < high {
		pi := partitionDuration(arr, low, high)
		quickSortDurationHelper(arr, low, pi-1)
		quickSortDurationHelper(arr, pi+1, high)
	}
}

func partitionDuration(arr []time.Duration, low, high int) int {
	pivot := arr[high]
	i := low - 1
	
	for j := low; j < high; j++ {
		if arr[j] < pivot {
			i++
			arr[i], arr[j] = arr[j], arr[i]
		}
	}
	
	arr[i+1], arr[high] = arr[high], arr[i+1]
	return i + 1
}

// PrintResults prints benchmark results
func (r *BenchmarkResult) PrintResults() {
	fmt.Println("\n=== Benchmark Results ===")
	fmt.Printf("Total Connections: %d\n", r.TotalConnections)
	fmt.Printf("Successful Messages: %d\n", r.SuccessfulMessages)
	fmt.Printf("Failed Messages: %d\n", r.FailedMessages)
	fmt.Printf("Total Duration: %v\n", r.TotalDuration)
	fmt.Printf("Messages Per Second: %.2f\n", r.MessagesPerSecond)
	fmt.Printf("Connections Per Second: %.2f\n", r.ConnectionsPerSecond)
	
	if r.AvgLatency > 0 {
		fmt.Println("\n=== Latency Statistics ===")
		fmt.Printf("Average Latency: %v\n", r.AvgLatency)
		fmt.Printf("P50 Latency: %v\n", r.P50Latency)
		fmt.Printf("P95 Latency: %v\n", r.P95Latency)
		fmt.Printf("P99 Latency: %v\n", r.P99Latency)
		fmt.Printf("Max Latency: %v\n", r.MaxLatency)
	}
	
	if len(r.Errors) > 0 {
		fmt.Printf("\nErrors: %d\n", len(r.Errors))
		for i, err := range r.Errors {
			if i < 5 { // Only print first 5 errors
				fmt.Printf("  - %v\n", err)
			}
		}
	}
}

// ConnectionBenchmark benchmarks connection establishment
type ConnectionBenchmark struct {
	targetURL      string
	targetConnections int
	rampUpTime     time.Duration
	logger         *zap.Logger
}

// NewConnectionBenchmark creates a new connection benchmark
func NewConnectionBenchmark(targetURL string, targetConnections int, rampUpTime time.Duration, logger *zap.Logger) *ConnectionBenchmark {
	return &ConnectionBenchmark{
		targetURL:         targetURL,
		targetConnections: targetConnections,
		rampUpTime:        rampUpTime,
		logger:            logger,
	}
}

// Run runs the connection benchmark
func (cb *ConnectionBenchmark) Run() map[string]interface{} {
	results := make(map[string]interface{})
	
	var wg sync.WaitGroup
	successCount := int64(0)
	failCount := int64(0)
	
	startTime := time.Now()
	
	// Ramp up connections
	connectionsPerSecond := float64(cb.targetConnections) / cb.rampUpTime.Seconds()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	
	created := 0
	start := time.Now()
	
	for created < cb.targetConnections {
		select {
		case <-ticker.C:
			target := int(connectionsPerSecond * time.Since(start).Seconds())
			if target > cb.targetConnections {
				target = cb.targetConnections
			}
			
			toCreate := target - created
			for i := 0; i < toCreate; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					
					userID := fmt.Sprintf("conn-bench-user-%d", id)
					deviceID := fmt.Sprintf("conn-bench-device-%d", id)
					
					wsURL := fmt.Sprintf("%s/ws?user_id=%s&device_id=%s&node_id=benchmark", 
						cb.targetURL, userID, deviceID)
					
					conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
					if err != nil {
						atomic.AddInt64(&failCount, 1)
						return
					}
					defer conn.Close()
					
					atomic.AddInt64(&successCount, 1)
				}(created + i)
			}
			created = target
		}
	}
	
	wg.Wait()
	
	duration := time.Since(startTime)
	
	results["target_connections"] = cb.targetConnections
	results["successful_connections"] = successCount
	results["failed_connections"] = failCount
	results["total_duration"] = duration
	results["connections_per_second"] = float64(successCount) / duration.Seconds()
	results["success_rate"] = float64(successCount) / float64(cb.targetConnections) * 100
	
	return results
}

// MessageThroughputBenchmark benchmarks message throughput
type MessageThroughputBenchmark struct {
	targetURL         string
	connections       int
	duration          time.Duration
	messagesPerSecond int
	logger            *zap.Logger
}

// NewMessageThroughputBenchmark creates a new throughput benchmark
func NewMessageThroughputBenchmark(targetURL string, connections int, duration time.Duration, messagesPerSecond int, logger *zap.Logger) *MessageThroughputBenchmark {
	return &MessageThroughputBenchmark{
		targetURL:         targetURL,
		connections:       connections,
		duration:          duration,
		messagesPerSecond: messagesPerSecond,
		logger:            logger,
	}
}

// Run runs the throughput benchmark
func (mtb *MessageThroughputBenchmark) Run() map[string]interface{} {
	results := make(map[string]interface{})
	
	var wg sync.WaitGroup
	messageCount := int64(0)
	errorCount := int64(0)
	
	// Establish connections
	conns := make([]*websocket.Conn, mtb.connections)
	for i := 0; i < mtb.connections; i++ {
		userID := fmt.Sprintf("throughput-user-%d", i)
		deviceID := fmt.Sprintf("throughput-device-%d", i)
		
		wsURL := fmt.Sprintf("%s/ws?user_id=%s&device_id=%s&node_id=benchmark", 
			mtb.targetURL, userID, deviceID)
		
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			mtb.logger.Error("Failed to establish connection", zap.Error(err))
			continue
		}
		conns[i] = conn
	}
	
	// Start sending messages
	startTime := time.Now()
	stopCh := make(chan struct{})
	
	for i, conn := range conns {
		if conn == nil {
			continue
		}
		
		wg.Add(1)
		go func(id int, c *websocket.Conn) {
			defer wg.Done()
			
			ticker := time.NewTicker(time.Second / time.Duration(mtb.messagesPerSecond))
			defer ticker.Stop()
			
			for {
				select {
				case <-stopCh:
					return
				case <-ticker.C:
					message := map[string]interface{}{
						"type":       "benchmark",
						"sender_id":  fmt.Sprintf("throughput-user-%d", id),
						"timestamp":  time.Now().UnixNano(),
						"content":    "test message content",
						"message_id": fmt.Sprintf("msg-%d", time.Now().UnixNano()),
					}
					
					if err := c.WriteJSON(message); err != nil {
						atomic.AddInt64(&errorCount, 1)
					} else {
						atomic.AddInt64(&messageCount, 1)
					}
				}
			}
		}(i, conn)
	}
	
	// Wait for duration
	time.Sleep(mtb.duration)
	close(stopCh)
	wg.Wait()
	
	duration := time.Since(startTime)
	
	// Close all connections
	for _, conn := range conns {
		if conn != nil {
			conn.Close()
		}
	}
	
	results["connections"] = mtb.connections
	results["messages_sent"] = messageCount
	results["errors"] = errorCount
	results["duration"] = duration
	results["messages_per_second"] = float64(messageCount) / duration.Seconds()
	results["error_rate"] = float64(errorCount) / float64(messageCount+errorCount) * 100
	
	return results
}
