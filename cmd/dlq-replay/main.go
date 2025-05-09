package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/newrelic/nrdot-internal-devlab/internal/common/logging"
	"github.com/syndtr/goleveldb/leveldb"
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
	var fbRxClient interface{} // Would be fb.ChainPushServiceClient in a real implementation

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
		// fbRxClient = fb.NewChainPushServiceClient(fbRxConn)
	}

	// Open DLQ storage
	if *dlqBackend == "leveldb" {
		// Ensure directory exists
		if err := os.MkdirAll(*dlqPath, 0755); err != nil {
			logger.Fatal("Failed to create DLQ directory", err, nil)
		}

		// Open LevelDB
		db, err := leveldb.OpenFile(*dlqPath, nil)
		if err != nil {
			logger.Fatal("Failed to open LevelDB", err, nil)
		}
		defer db.Close()

		// Count messages
		count := 0
		iter := db.NewIterator(nil, nil)
		for iter.Next() {
			count++
		}
		iter.Release()

		logger.Info("Opened DLQ database", map[string]interface{}{"count": count, "path": *dlqPath})

		// Find messages to replay based on filters
		filtered := 0
		iter = db.NewIterator(nil, nil)
		defer iter.Release()

		for iter.Next() {
			// In a real implementation, this would:
			// 1. Check if the message matches the filters (time, error code, FB sender)
			// 2. Parse the message and extract the batch
			// 3. Send the batch to FB-RX if not in dry-run mode
			// 4. Delete the message if deleteReplayed is true
			
			filtered++
		}

		if err := iter.Error(); err != nil {
			logger.Error("Error iterating DLQ", err, nil)
		}

		logger.Info("Replay statistics", map[string]interface{}{
			"total":     count,
			"filtered":  filtered,
			"replayed":  0, // Would be actual count in real implementation
			"errors":    0, // Would be error count in real implementation
			"dry_run":   *dryRun,
		})
	} else if *dlqBackend == "kafka" {
		logger.Error("Kafka backend not implemented", fmt.Errorf("not implemented"), nil)
	} else {
		logger.Fatal("Unknown DLQ backend", fmt.Errorf("unknown DLQ backend: %s", *dlqBackend), nil)
	}

	logger.Info("DLQ replay complete", nil)
}

// parseTimeFilter parses a time filter string (e.g. "1h", "2d") into a time.Time
func parseTimeFilter(filter string) (time.Time, error) {
	duration, err := time.ParseDuration(filter)
	if err != nil {
		return time.Time{}, err
	}
	return time.Now().Add(-duration), nil
}
