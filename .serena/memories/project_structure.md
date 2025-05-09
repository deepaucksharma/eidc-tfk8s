# NRDOT+ Internal Dev-Lab Project Structure

```
nrdot-internal-devlab/
├── cmd/                          # Command-line entry points
│   ├── configcontroller/         # ConfigController for CRD management
│   ├── dlq-replay/               # DLQ replay tool
│   └── fb/                       # Function Block entry points
│       ├── rx/                   # FB-RX (Receiver) entry point
│       ├── en-host/              # FB-EN-HOST entry point
│       ├── en-k8s/               # FB-EN-K8S entry point
│       ├── cl/                   # FB-CL entry point
│       ├── dp/                   # FB-DP entry point
│       ├── fs/                   # FB-FS entry point
│       ├── agg/                  # FB-AGG entry point
│       ├── gw-pre/               # FB-GW-PRE entry point
│       ├── gw/                   # FB-GW entry point
│       └── dlq/                  # FB-DLQ entry point
├── deploy/                       # Deployment artifacts
│   ├── helm/                     # Helm chart for deployment
│   │   ├── templates/            # Kubernetes manifest templates
│   │   ├── values.yaml           # Default values
│   │   └── values-lab.yaml       # Lab-specific values
│   ├── k8s/                      # Raw K8s manifests
│   │   └── crds/                 # Custom Resource Definitions
│   └── scripts/                  # Operational scripts
│       ├── backup-restore/       # Backup and restore scripts
│       └── seed-rotation/        # PII salt rotation scripts
├── docs/                         # Documentation
│   ├── architecture/             # Architecture diagrams and descriptions
│   ├── runbooks/                 # Operational runbooks
│   └── api/                      # API documentation
├── internal/                     # Internal packages
│   ├── common/                   # Shared utilities
│   │   ├── metrics/              # Prometheus metrics
│   │   ├── tracing/              # W3C tracing
│   │   ├── logging/              # JSON logging
│   │   ├── resilience/           # Circuit breaker implementation
│   │   └── schema/               # Data schemas and validation
│   └── config/                   # Configuration handling
├── pkg/                          # Public API packages
│   ├── api/                      # API definitions
│   │   ├── protobuf/             # Protocol buffer definitions
│   │   └── crds/                 # CRD Go types
│   ├── fb/                       # Function Blocks
│   │   ├── rx/                   # Receiver (OTLP/gRPC, OTLP/HTTP, Prom)
│   │   ├── en-host/              # Host Enrichment
│   │   ├── en-k8s/               # Kubernetes Enrichment
│   │   ├── cl/                   # Classification (PII handling)
│   │   ├── dp/                   # Deduplication (BadgerDB)
│   │   ├── fs/                   # Filter/Sample
│   │   ├── agg/                  # Aggregation
│   │   ├── gw-pre/               # Gateway Pre-processing (LevelDB)
│   │   ├── gw/                   # Gateway (Schema enforcement)
│   │   └── dlq/                  # Dead Letter Queue
│   └── test/                     # Integration and e2e tests
│       ├── loadgen/              # Load generation
│       ├── slo/                  # SLO validation tests
│       └── chaos/                # Resilience and chaos tests
├── go.mod                        # Go module definition
├── go.sum                        # Go module checksums
└── README.md                     # Project overview
```

## Directory Explanations

### `cmd/`
Contains the entry points for all executables in the project. Each Function Block has its own main package, as do the ConfigController and DLQ replay tool.

### `deploy/`
Contains all deployment-related files:
- Helm charts for deploying the entire stack
- Kubernetes manifest files, including CRDs
- Operational scripts for backup/restore and maintenance

### `docs/`
Project documentation, including architecture diagrams, runbooks, and API specs.

### `internal/`
Go packages that are not intended to be imported by other projects. Contains common utilities used across the codebase.

### `pkg/`
Public API packages that could potentially be imported by other projects. Contains:
- API definitions (protobuf, CRDs)
- Function Block implementations
- Test utilities

### Key Files
- `go.mod`: Defines the module and its dependencies
- `README.md`: Project overview and getting started guide
