package agg

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/newrelic/nrdot-internal-devlab/pkg/config"
	"github.com/newrelic/nrdot-internal-devlab/pkg/metrics"
	"github.com/newrelic/nrdot-internal-devlab/pkg/telemetry"
)

// Version is the version of the FB-AGG function block
var Version = "1.0.0"

func Main() {
	// Initialize logging
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})

	// Parse command line flags
	fbName := config.ParseFlags("fb-agg")

	// Create context that cancels on SIGINT or SIGTERM
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalCh
		log.Info().Msg("Received shutdown signal")
		cancel()
	}()

	// Create metrics factory
	metricsFactory := metrics.NewPrometheusFactory()

	// Create forwarder for sending aggregated metrics to the next function block
	forwarder, err := telemetry.NewGRPCForwarder(config.GetNextFunctionBlockAddr())
	if err != nil {
		log.Error().Err(err).Msg("Failed to create forwarder")
		os.Exit(1)
	}

	// Create the aggregation function block
	agg := NewAggregationFunctionBlock(fbName, forwarder, metricsFactory)

	// Initialize the function block
	if err := agg.Initialize(ctx); err != nil {
		log.Error().Err(err).Msg("Failed to initialize function block")
		os.Exit(1)
	}

	// Connect to the configuration controller
	configClient, err := config.NewConfigClient(config.GetConfigControllerAddr())
	if err != nil {
		log.Error().Err(err).Msg("Failed to create config client")
		os.Exit(1)
	}

	// Start a goroutine to watch for configuration updates
	go func() {
		if err := configClient.WatchConfig(ctx, "fb-agg", fbName, func(configBytes []byte, generation int64) error {
			return agg.UpdateConfig(ctx, configBytes, generation)
		}); err != nil {
			log.Error().Err(err).Msg("Config watch failed")
			cancel()
		}
	}()

	// Start the HTTP server for metrics and healthchecks
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if agg.Ready() {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "OK")
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "Not Ready")
		}
	})
	http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		if agg.Ready() {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "Ready")
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "Not Ready")
		}
	})

	// Create the gRPC server for receiving metric batches
	server, err := telemetry.NewGRPCServer(fbName, agg)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create gRPC server")
		os.Exit(1)
	}

	// Start the gRPC server
	go func() {
		if err := server.Start(ctx); err != nil {
			log.Error().Err(err).Msg("gRPC server failed")
			cancel()
		}
	}()

	// Start the HTTP server
	go func() {
		if err := http.ListenAndServe(":2112", nil); err != nil {
			log.Error().Err(err).Msg("HTTP server failed")
			cancel()
		}
	}()

	log.Info().Str("version", Version).Str("name", fbName).Msg("FB-AGG started")

	// Wait for context cancellation (shutdown signal)
	<-ctx.Done()

	// Perform graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	log.Info().Msg("Shutting down...")

	// Shutdown the function block
	if err := agg.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Error during function block shutdown")
	}

	// Shutdown the gRPC server
	if err := server.Stop(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Error during gRPC server shutdown")
	}

	log.Info().Msg("Shutdown complete")
}
