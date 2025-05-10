package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"eidc-tfk8s/internal/common/logging"
	"eidc-tfk8s/internal/common/metrics"
	"eidc-tfk8s/internal/common/tracing"
	"eidc-tfk8s/pkg/fb/cl"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Build information, injected at build time
var (
	Version   string = "2.1.2-dev"
	BuildTime string
	CommitSHA string
)

func main() {
	// Parse command-line flags
	var (
		grpcPort           = flag.Int("grpc-port", 5000, "gRPC service port")
		metricsPort        = flag.Int("metrics-port", 2112, "Prometheus metrics port")
		configServiceAddr  = flag.String("config-service", "config-controller:5000", "Config controller gRPC service address")
		nextFB             = flag.String("next-fb", "fb-dp:5000", "Next Function Block in the chain")
		dlqServiceAddr     = flag.String("dlq-service", "fb-dlq:5000", "DLQ service address")
		otlpExporterAddr   = flag.String("otlp-exporter", "otel-collector:4317", "OTLP exporter address for traces")
		traceSamplingRatio = flag.Float64("trace-sampling-ratio", 0.1, "Sampling ratio for traces (0.0-1.0)")
		saltSecretName     = flag.String("salt-secret-name", "pii-salt", "Name of the secret containing the PII salt")
		saltSecretKey      = flag.String("salt-secret-key", "salt", "Key in the secret containing the PII salt value")
	)
	flag.Parse()

	// Set up logging
	logger := logging.NewLogger("fb-cl")
	logger.Info("Starting FB-CL", map[string]interface{}{
		"version":    Version,
		"build_time": BuildTime,
		"commit":     CommitSHA,
	})

	// Set up tracing
	shutdown, err := tracing.InitTracer(context.Background(), "fb-cl", Version, "dev-lab", *otlpExporterAddr, *traceSamplingRatio)
	if err != nil {
		logger.Fatal("Failed to initialize tracer", err, nil)
	}
	defer shutdown()

	// Create context that listens for termination signals
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

	// Set up metrics server
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("healthy"))
	})
	http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		// TODO: Implement proper readiness check
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})

	metricsServer := &http.Server{
		Addr:    ":" + string(*metricsPort),
		Handler: nil,
	}

	go func() {
		logger.Info("Starting metrics server", map[string]interface{}{"port": *metricsPort})
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Metrics server failed", err, nil)
			cancel()
		}
	}()

	// Create and initialize FB-CL
	fbMetrics := metrics.NewFBMetrics("fb-cl")
	tracer := tracing.NewTracer("fb-cl")
	
	classifier := cl.NewClassifier(logger, fbMetrics, tracer, *saltSecretName, *saltSecretKey)
	
	// Initialize the classifier
	if err := classifier.Initialize(ctx); err != nil {
		logger.Fatal("Failed to initialize classifier", err, nil)
	}

	// Start the gRPC server for ChainPushService
	grpcServer, err := cl.StartGRPCServer(ctx, classifier, *grpcPort)
	if err != nil {
		logger.Fatal("Failed to start gRPC server", err, nil)
	}

	// Connect to config service, next FB, and DLQ
	if err := classifier.ConnectServices(ctx, *configServiceAddr, *nextFB, *dlqServiceAddr); err != nil {
		logger.Error("Failed to connect to services", err, nil)
		// Continue anyway, we'll retry connections as needed
	}

	// Wait for termination
	<-ctx.Done()
	logger.Info("Shutting down", nil)

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Shutdown FB-CL
	if err := classifier.Shutdown(shutdownCtx); err != nil {
		logger.Error("Error shutting down classifier", err, nil)
	}

	// Shutdown metrics server
	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("Error shutting down metrics server", err, nil)
	}

	// Gracefully stop the gRPC server
	grpcServer.GracefulStop()

	logger.Info("Shutdown complete", nil)
}

