package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"

	pb "github.com/newrelic/nrdot-internal-devlab/pkg/api/protobuf"
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
		namespace          = flag.String("namespace", "", "Kubernetes namespace (default: auto-detect)")
		leaseLockName      = flag.String("lease-lock-name", "config-controller", "Name of the lease lock")
		leaseLockNamespace = flag.String("lease-lock-namespace", "", "Namespace of the lease lock")
		id                 = flag.String("id", "", "Leader election ID (default: pod name)")
		leaseDuration      = flag.Duration("lease-duration", 15*time.Second, "Leader lease duration")
		renewDeadline      = flag.Duration("renew-deadline", 10*time.Second, "Leader renew deadline")
		retryPeriod        = flag.Duration("retry-period", 2*time.Second, "Leader election retry period")
	)
	flag.Parse()

	// Set up logging with JSON format
	logger := log.New(os.Stdout, "", 0)
	logger.Printf(`{"level":"info","timestamp":"%s","message":"Starting ConfigController","version":"%s","build_time":"%s","commit":"%s"}`,
		time.Now().Format(time.RFC3339), Version, BuildTime, CommitSHA)

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

	// Initialize Kubernetes client
	config, err := rest.InClusterConfig()
	if err != nil {
		logger.Printf(`{"level":"error","timestamp":"%s","message":"Failed to create in-cluster config","error":"%s"}`,
			time.Now().Format(time.RFC3339), err)
		os.Exit(1)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Printf(`{"level":"error","timestamp":"%s","message":"Failed to create Kubernetes client","error":"%s"}`,
			time.Now().Format(time.RFC3339), err)
		os.Exit(1)
	}

	// Auto-detect namespace if not provided
	if *namespace == "" {
		if data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
			*namespace = string(data)
		} else {
			logger.Printf(`{"level":"error","timestamp":"%s","message":"Failed to auto-detect namespace","error":"%s"}`,
				time.Now().Format(time.RFC3339), err)
			os.Exit(1)
		}
	}

	// Auto-detect pod name for ID if not provided
	if *id == "" {
		if hostname, err := os.Hostname(); err == nil {
			*id = hostname
		} else {
			logger.Printf(`{"level":"error","timestamp":"%s","message":"Failed to auto-detect hostname","error":"%s"}`,
				time.Now().Format(time.RFC3339), err)
			os.Exit(1)
		}
	}

	// Use the same namespace for lease lock if not specified
	if *leaseLockNamespace == "" {
		*leaseLockNamespace = *namespace
	}

	// Initialize gRPC server
	server := grpc.NewServer()
	// Create and register ConfigController as the ConfigService implementation
	configController := NewConfigController(logger, clientset, *namespace)
	pb.RegisterConfigServiceServer(server, configController)

	// Start gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *grpcPort))
	if err != nil {
		logger.Printf(`{"level":"error","timestamp":"%s","message":"Failed to listen on gRPC port","error":"%s"}`,
			time.Now().Format(time.RFC3339), err)
		os.Exit(1)
	}

	go func() {
		logger.Printf(`{"level":"info","timestamp":"%s","message":"Starting gRPC server","port":%d}`,
			time.Now().Format(time.RFC3339), *grpcPort)
		if err := server.Serve(lis); err != nil {
			logger.Printf(`{"level":"error","timestamp":"%s","message":"gRPC server failed","error":"%s"}`,
				time.Now().Format(time.RFC3339), err)
			cancel()
		}
	}()

	// Set up leader election
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      *leaseLockName,
			Namespace: *leaseLockNamespace,
		},
		Client: clientset.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: *id,
		},
	}

	// Run leader election
	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   *leaseDuration,
		RenewDeadline:   *renewDeadline,
		RetryPeriod:     *retryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				logger.Printf(`{"level":"info","timestamp":"%s","message":"Started leading","id":"%s"}`,
					time.Now().Format(time.RFC3339), *id)
				// Start the controller when we become leader
				runController(ctx, configController, clientset, *namespace, logger)
			},
			OnStoppedLeading: func() {
				logger.Printf(`{"level":"info","timestamp":"%s","message":"Stopped leading","id":"%s"}`,
					time.Now().Format(time.RFC3339), *id)
				// We should exit if we stop being leader
				cancel()
			},
			OnNewLeader: func(identity string) {
				if identity != *id {
					logger.Printf(`{"level":"info","timestamp":"%s","message":"New leader elected","leader":"%s"}`,
						time.Now().Format(time.RFC3339), identity)
				}
			},
		},
	})

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

	// Gracefully stop the gRPC server
	server.GracefulStop()

	logger.Printf(`{"level":"info","timestamp":"%s","message":"Shutdown complete"}`, time.Now().Format(time.RFC3339))
}

// runController runs the main controller loop that watches for pipeline resources and distributes configuration
func runController(ctx context.Context, configController *ConfigController, clientset *kubernetes.Clientset, namespace string, logger *log.Logger) {
	logger.Printf(`{"level":"info","timestamp":"%s","message":"Starting controller for namespace","namespace":"%s"}`,
		time.Now().Format(time.RFC3339), namespace)

	// Create CRD controller
	crdController, err := NewCRDController(logger, configController, clientset, namespace)
	if err != nil {
		logger.Printf(`{"level":"error","timestamp":"%s","message":"Failed to create CRD controller","error":"%s"}`,
			time.Now().Format(time.RFC3339), err)
		return
	}

	// Start CRD controller
	if err := crdController.Start(ctx); err != nil {
		logger.Printf(`{"level":"error","timestamp":"%s","message":"Failed to start CRD controller","error":"%s"}`,
			time.Now().Format(time.RFC3339), err)
		return
	}

	// Wait for context cancellation
	<-ctx.Done()
}