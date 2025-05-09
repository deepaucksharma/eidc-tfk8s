package tracing

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

// Tracer provides standardized tracing for function blocks
type Tracer struct {
	fbName     string
	tracer     trace.Tracer
	propagator propagation.TextMapPropagator
}

// InitTracer initializes the global OpenTelemetry tracer
func InitTracer(ctx context.Context, serviceName, version, environment, exporterEndpoint string, samplingRatio float64) (func(), error) {
	// Create OTLP exporter
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(exporterEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	// Create resource with service information
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String(version),
			attribute.String("environment", environment),
		),
	)
	if err != nil {
		return nil, err
	}

	// Create trace provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(samplingRatio)),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Set global trace provider
	otel.SetTracerProvider(tp)

	// Set global propagator to W3C TraceContext
	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	otel.SetTextMapPropagator(propagator)

	// Return shutdown function
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			// Use stderr since logger might be unavailable during shutdown
			// Format as JSON to maintain consistency
			// TODO: Use logger if available
		}
	}, nil
}

// NewTracer creates a new tracer for the specified function block
func NewTracer(fbName string) *Tracer {
	return &Tracer{
		fbName:     fbName,
		tracer:     otel.Tracer("github.com/newrelic/nrdot-internal-devlab"),
		propagator: otel.GetTextMapPropagator(),
	}
}

// StartSpan starts a new span with the specified name and attributes
func (t *Tracer) StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	// Start with the FB name as prefix
	fullName := t.fbName + "-" + name

	// Add default attributes for all spans
	defaultAttrs := []attribute.KeyValue{
		attribute.String("fb_name", t.fbName),
	}

	// Combine default attributes with provided attributes
	allAttrs := append(defaultAttrs, attrs...)

	// Start the span
	return t.tracer.Start(ctx, fullName, trace.WithAttributes(allAttrs...))
}

// ExtractSpanContext extracts the span context from the provided carrier
func (t *Tracer) ExtractSpanContext(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	return t.propagator.Extract(ctx, carrier)
}

// InjectSpanContext injects the span context into the provided carrier
func (t *Tracer) InjectSpanContext(ctx context.Context, carrier propagation.TextMapCarrier) {
	t.propagator.Inject(ctx, carrier)
}

// RecordBatchProcessingSpan creates and records a span for batch processing
func (t *Tracer) RecordBatchProcessingSpan(ctx context.Context, batchID string, processingFunc func(context.Context) error) error {
	ctx, span := t.StartSpan(ctx, "process-batch",
		attribute.String("batch_id", batchID),
	)
	defer span.End()

	// Execute the processing function
	err := processingFunc(ctx)

	// Record error if any
	if err != nil {
		span.RecordError(err)
	}

	return err
}

// RecordBatchForwardingSpan creates and records a span for batch forwarding
func (t *Tracer) RecordBatchForwardingSpan(ctx context.Context, batchID string, nextFB string, forwardingFunc func(context.Context) error) error {
	ctx, span := t.StartSpan(ctx, "forward-batch",
		attribute.String("batch_id", batchID),
		attribute.String("next_fb", nextFB),
	)
	defer span.End()

	// Execute the forwarding function
	err := forwardingFunc(ctx)

	// Record error if any
	if err != nil {
		span.RecordError(err)
	}

	return err
}

// RecordConfigUpdateSpan creates and records a span for configuration updates
func (t *Tracer) RecordConfigUpdateSpan(ctx context.Context, generation int64, updateFunc func(context.Context) error) error {
	ctx, span := t.StartSpan(ctx, "update-config",
		attribute.Int64("config_generation", generation),
	)
	defer span.End()

	// Execute the update function
	err := updateFunc(ctx)

	// Record error if any
	if err != nil {
		span.RecordError(err)
	}

	return err
}
