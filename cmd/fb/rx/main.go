package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
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
		grpcPort           = flag.Int("grpc-port", 4317, "OTLP gRPC port")
		httpPort           = flag.Int("http-port", 4318, "OTLP HTTP port")
		promPort           = flag.Int("prom-port", 9009, "Prometheus remote-write port")
		metricsPort        = flag.Int("metrics-port", 2112, "Prometheus metrics port")
		configServiceAddr  = flag.String("config-service", "config-controller:5000", "Config controller gRPC service address")
		nextFB             = flag.String("next-fb", "fb-en-host:5000", "Next Function Block in the chain")
		dlqServiceAddr     = flag.String("dlq-service", "fb-dlq:5000", "DLQ service address")
		otlpExporterAddr   = flag.String("otlp-exporter", "otel-collector:4317", "OTLP exporter address for traces")
		traceSamplingRatio = flag.Float64("trace-sampling-ratio", 0.1, "Sampling ratio for traces (0.0-1.0)")
	)
	flag.Parse()

	// Set up logging with JSON format
	logger := log.New(os.Stdout, "", 0)
	logger.Printf(`{"level":"info","timestamp":"%s","message":"Starting FB-RX","version":"%s","build_time":"%s","commit":"%s"}`,
		time.Now().Format(time.RFC3339), Version, BuildTime, CommitSHA)

	// Set up tracing
	shutdown, err := initTracer(context.Background(), *otlpExporterAddr, *traceSamplingRatio)
	if err != nil {
		logger.Printf(`{"level":"error","timestamp":"%s","message":"Failed to initialize tracer","error":"%s"}`,
			time.Now().Format(time.RFC3339), err)
		os.Exit(1)
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
		logger.Printf(`{"level":"info","timestamp":"%s","message":"Received signal","signal":"%s"}`,
			time.Now().Format(time.RFC3339), sig)
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
		Addr:    fmt.Sprintf(":%d", *metricsPort),
		Handler: nil,
	}

	go func() {
		logger.Printf(`{"level":"info","timestamp":"%s","message":"Starting metrics server","port":%d}`,
			time.Now().Format(time.RFC3339), *metricsPort)
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Printf(`{"level":"error","timestamp":"%s","message":"Metrics server failed","error":"%s"}`,
				time.Now().Format(time.RFC3339), err)
			cancel()
		}
	}()

	// TODO: Initialize config watcher
	logger.Printf(`{"level":"info","timestamp":"%s","message":"Connecting to config service","address":"%s"}`,
		time.Now().Format(time.RFC3339), *configServiceAddr)

	// TODO: Initialize receivers
	logger.Printf(`{"level":"info","timestamp":"%s","message":"Starting OTLP/gRPC receiver","port":%d}`,
		time.Now().Format(time.RFC3339), *grpcPort)
	logger.Printf(`{"level":"info","timestamp":"%s","message":"Starting OTLP/HTTP receiver","port":%d}`,
		time.Now().Format(time.RFC3339), *httpPort)
	logger.Printf(`{"level":"info","timestamp":"%s","message":"Starting Prometheus remote-write receiver","port":%d}`,
		time.Now().Format(time.RFC3339), *promPort)

	// TODO: Initialize connections to next FB and DLQ
	logger.Printf(`{"level":"info","timestamp":"%s","message":"Connecting to next FB","address":"%s"}`,
		time.Now().Format(time.RFC3339), *nextFB)
	logger.Printf(`{"level":"info","timestamp":"%s","message":"Connecting to DLQ service","address":"%s"}`,
		time.Now().Format(time.RFC3339), *dlqServiceAddr)

	// TODO: Initialize and start the actual receivers and processors

	// Wait for termination
	<-ctx.Done()
	logger.Printf(`{"level":"info","timestamp":"%s","message":"Shutting down"}`, time.Now().Format(time.RFC3339))

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		logger.Printf(`{"level":"error","timestamp":"%s","message":"Error shutting down metrics server","error":"%s"}`,
			time.Now().Format(time.RFC3339), err)
	}

	// TODO: Clean shutdown of other components

	logger.Printf(`{"level":"info","timestamp":"%s","message":"Shutdown complete"}`, time.Now().Format(time.RFC3339))
}

// initTracer initializes the OpenTelemetry tracer
func initTracer(ctx context.Context, exporterEndpoint string, samplingRatio float64) (func(), error) {
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(exporterEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("fb-rx"),
			semconv.ServiceVersionKey.String(Version),
			attribute.String("environment", "dev-lab"),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(samplingRatio)),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			log.Printf(`{"level":"error","timestamp":"%s","message":"Error shutting down tracer provider","error":"%s"}`,
				time.Now().Format(time.RFC3339), err)
		}
	}, nil
}
