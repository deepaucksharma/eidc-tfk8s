package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/newrelic/nrdot-internal-devlab/internal/common/logging"
	"github.com/newrelic/nrdot-internal-devlab/pkg/fb"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Build information, injected at build time
var (
	Version   string = "2.1.2-dev"
	BuildTime string
	CommitSHA string
)

// Command line flags
var (
	dlqPath         = flag.String("dlq-path", "/data/dlq", "Path to the DLQ storage")
	dlqBackend      = flag.String("dlq-backend", "leveldb", "DLQ backend (leveldb or kafka)")
	fbRxAddr        = flag.String("fb-rx-addr", "fb-rx:5000", "Address of the FB-RX service")
	dryRun          = flag.Bool("dry-run", false, "Dry run (don't actually replay)")
	sinceStr        = flag.String("since", "", "Replay messages since (e.g. 1h, 2d, etc)")
	untilStr        = flag.String("until", "", "Replay messages until (e.g. 1h, 2d, etc)")
	errorCode       = flag.String("error-code", "", "Replay only messages with this error code")
	fbSender        = flag.String("fb-sender", "", "Replay only messages from this FB")
	concurrency     = flag.Int("concurrency", 5, "Number of concurrent replays")
	batchSize       = flag.Int("batch-size", 100, "Number of messages to replay in a batch")
	waitMs          = flag.Int("wait-ms", 0, "Milliseconds to wait between batches")
	deleteReplayed  = flag.Bool("delete-replayed", false, "Delete messages after replay")
)

// DLQMessage is the structure of a message stored in the DLQ
type DLQMessage struct {
	BatchID        string            `json:"batch_id"`
	Data           []byte            `json:"data"`
	Format         string            `json:"format"`
	Timestamp      time.Time         `json:"timestamp"`
	ErrorCode      string            `json:"error_code"`
	ErrorMessage   string            `json:"error_message"`
	FBSender       string            `json:"fb_sender"`
	InternalLabels map[string]string `json:"internal_labels"`
	Metadata       map[string]string `json:"metadata"`
}

// ReplayStats tracks replay statistics
type ReplayStats struct {
	mu             sync.Mutex
	total          int
	filtered       int
	replayed       int
	errors         int
	errorsByReason map[string]int
}

func main() {
	// Parse command line flags
	flag.Parse()

	// Set up logging
	logger := logging.NewLogger("dlq-replay")
	logger.Info("Starting DLQ replay tool", map[string]interface{}{
		"version":     Version,
		"build_time":  BuildTime,
		"commit":      CommitSHA,
		"dlq_path":    *dlqPath,
		"dlq_backend": *dlqBackend,
		"fb_rx_addr":  *fbRxAddr,
		"dry_run":     *dryRun,
	})

	// Set up context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logger.Info("Received signal", map[string]interface{}{"signal": sig.String()})
		cancel()
	}()

	// Parse time filters
	var since, until time.Time
	var err error
	if *sinceStr != "" {
		since, err = parseTimeFilter(*sinceStr)
		if err != nil {
			logger.Fatal("Invalid --since value", err, nil)
		}
	}
	if *untilStr != "" {
		until, err = parseTimeFilter(*untilStr)
		if err != nil {
			logger.Fatal("Invalid --until value", err, nil)
		}
	}

	// Connect to FB-RX
	var fbRxConn *grpc.ClientConn
	var fbRxClient fb.ChainPushServiceClient

	if !*dryRun {
		logger.Info("Connecting to FB-RX", map[string]interface{}{"addr": *fbRxAddr})
		fbRxConn, err = grpc.DialContext(ctx, *fbRxAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
			grpc.WithTimeout(5*time.Second),
		)
		if err != nil {
			logger.Fatal("Failed to connect to FB-RX", err, nil)
		}
		defer fbRxConn.Close()
		fbRxClient = fb.NewChainPushServiceClient(fbRxConn)
	}

	// Initialize stats
	stats := &ReplayStats{
		errorsByReason: make(map[string]int),
	}

	// Open DLQ storage
	if *dlqBackend == "leveldb" {
		err := replayFromLevelDB(ctx, logger, fbRxClient, stats, since, until)
		if err != nil {
			logger.Fatal("Failed to replay from LevelDB", err, nil)
		}
	} else if *dlqBackend == "kafka" {
		logger.Error("Kafka backend not implemented", fmt.Errorf("not implemented"), nil)
	} else {
		logger.Fatal("Unknown DLQ backend", fmt.Errorf("unknown DLQ backend: %s", *dlqBackend), nil)
	}

	logger.Info("DLQ replay complete", map[string]interface{}{
		"total":    stats.total,
		"filtered": stats.filtered,
		"replayed": stats.replayed,
		"errors":   stats.errors,
		"dry_run":  *dryRun,
	})

	// Print error counts by reason
	if stats.errors > 0 {
		for reason, count := range stats.errorsByReason {
			logger.Info("Replay errors", map[string]interface{}{
				"reason": reason,
				"count":  count,
			})
		}
	}
}

// replayFromLevelDB replays messages from a LevelDB DLQ
func replayFromLevelDB(ctx context.Context, logger *logging.Logger, client fb.ChainPushServiceClient, stats *ReplayStats, since, until time.Time) error {
	// Ensure directory exists
	if err := os.MkdirAll(*dlqPath, 0755); err != nil {
		return fmt.Errorf("failed to create DLQ directory: %w", err)
	}

	// Open LevelDB
	db, err := leveldb.OpenFile(*dlqPath, nil)
	if err != nil {
		return fmt.Errorf("failed to open LevelDB: %w", err)
	}
	defer db.Close()

	// Count total messages
	count := 0
	iter := db.NewIterator(nil, nil)
	for iter.Next() {
		count++
	}
	iter.Release()
	if err := iter.Error(); err != nil {
		return fmt.Errorf("error counting messages: %w", err)
	}

	stats.total = count
	logger.Info("Opened DLQ database", map[string]interface{}{"count": count, "path": *dlqPath})

	// Create a channel to receive messages to replay
	messageCh := make(chan struct {
		key   []byte
		value []byte
	}, *batchSize)

	// Create a wait group for worker goroutines
	var wg sync.WaitGroup

	// Start worker goroutines
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range messageCh {
				err := processMessage(ctx, logger, client, db, item.key, item.value, stats, since, until)
				if err != nil {
					logger.Error("Error processing message", err, nil)
				}

				// Wait if requested
				if *waitMs > 0 {
					time.Sleep(time.Duration(*waitMs) * time.Millisecond)
				}
			}
		}()
	}

	// Iterate through messages and send to workers
	iter = db.NewIterator(nil, nil)
	defer iter.Release()

	for iter.Next() {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			logger.Info("Context cancelled, stopping replay", nil)
			break
		default:
		}

		// Queue message for processing
		messageCh <- struct {
			key   []byte
			value []byte
		}{
			key:   append([]byte{}, iter.Key()...),
			value: append([]byte{}, iter.Value()...),
		}
	}

	// Close channel and wait for workers to finish
	close(messageCh)
	wg.Wait()

	if err := iter.Error(); err != nil {
		return fmt.Errorf("error iterating DLQ: %w", err)
	}

	return nil
}

// processMessage processes a single message from the DLQ
func processMessage(ctx context.Context, logger *logging.Logger, client fb.ChainPushServiceClient, db *leveldb.DB, key, value []byte, stats *ReplayStats, since, until time.Time) error {
	// Parse message
	var message DLQMessage
	if err := json.Unmarshal(value, &message); err != nil {
		stats.mu.Lock()
		stats.errors++
		stats.errorsByReason["unmarshal-error"]++
		stats.mu.Unlock()
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	// Apply filters
	if !matchesFilters(message, since, until) {
		stats.mu.Lock()
		stats.filtered++
		stats.mu.Unlock()
		return nil
	}

	// Extract batch info
	if !*dryRun && client != nil {
		// Add replay indicator to internal labels
		if message.InternalLabels == nil {
			message.InternalLabels = make(map[string]string)
		}
		message.InternalLabels["replay"] = "true"
		message.InternalLabels["replay_timestamp"] = time.Now().Format(time.RFC3339)

		// Create replay request
		req := &fb.MetricBatchRequest{
			BatchId:          message.BatchID,
			Data:             message.Data,
			Format:           message.Format,
			Replay:           true,
			ConfigGeneration: 0, // Will be determined by the receiving FB
			Metadata:         message.Metadata,
			InternalLabels:   message.InternalLabels,
		}

		// Send to FB-RX
		resp, err := client.PushMetrics(ctx, req)
		if err != nil {
			stats.mu.Lock()
			stats.errors++
			stats.errorsByReason["grpc-error"]++
			stats.mu.Unlock()
			return fmt.Errorf("failed to send message to FB-RX: %w", err)
		}

		if resp.Status != fb.StatusSuccess {
			stats.mu.Lock()
			stats.errors++
			stats.errorsByReason[string(resp.ErrorCode)]++
			stats.mu.Unlock()
			return fmt.Errorf("FB-RX returned error: %s (code: %s)", resp.ErrorMessage, resp.ErrorCode)
		}

		// Delete if requested
		if *deleteReplayed {
			if err := db.Delete(key, nil); err != nil {
				logger.Error("Failed to delete replayed message", err, map[string]interface{}{
					"batch_id": message.BatchID,
				})
			}
		}

		stats.mu.Lock()
		stats.replayed++
		stats.mu.Unlock()
	} else {
		// Dry run mode
		stats.mu.Lock()
		stats.replayed++
		stats.mu.Unlock()

		logger.Info("Dry run: would replay message", map[string]interface{}{
			"batch_id":   message.BatchID,
			"fb_sender":  message.FBSender,
			"error_code": message.ErrorCode,
			"timestamp":  message.Timestamp,
		})
	}

	return nil
}

// matchesFilters checks if a message matches the specified filters
func matchesFilters(message DLQMessage, since, until time.Time) bool {
	// Time filter
	if !since.IsZero() && message.Timestamp.Before(since) {
		return false
	}
	if !until.IsZero() && message.Timestamp.After(until) {
		return false
	}

	// Error code filter
	if *errorCode != "" && message.ErrorCode != *errorCode {
		return false
	}

	// FB sender filter
	if *fbSender != "" && message.FBSender != *fbSender {
		return false
	}

	return true
}

// parseTimeFilter parses a time filter string (e.g. "1h", "2d") into a time.Time
func parseTimeFilter(filter string) (time.Time, error) {
	duration, err := time.ParseDuration(filter)
	if err != nil {
		return time.Time{}, err
	}
	return time.Now().Add(-duration), nil
}