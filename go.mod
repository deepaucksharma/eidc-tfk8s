module github.com/newrelic/nrdot-internal-devlab

go 1.21

require (
	github.com/dgraph-io/badger/v3 v3.2103.5
	github.com/prometheus/client_golang v1.18.0
	github.com/prometheus/client_model v0.5.0
	github.com/prometheus/common v0.45.0
	github.com/syndtr/goleveldb v1.0.0
	go.opentelemetry.io/collector v0.97.0
	go.opentelemetry.io/collector/exporter/otlpexporter v0.97.0
	go.opentelemetry.io/collector/receiver/otlpreceiver v0.97.0
	go.opentelemetry.io/collector/receiver/prometheusreceiver v0.97.0
	go.opentelemetry.io/otel v1.24.0
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v0.46.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.24.0
	go.opentelemetry.io/otel/sdk v1.24.0
	go.opentelemetry.io/otel/sdk/metric v1.24.0
	google.golang.org/grpc v1.62.0
	k8s.io/apimachinery v0.29.2
	k8s.io/client-go v0.29.2
	sigs.k8s.io/controller-runtime v0.17.2
)
