package tracing

import (
	"context"
	"fmt"
	"os"
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

// Tracer provides a simplified interface for creating traces
type Tracer struct {
	serviceName string
	tracer      trace.Tracer
}

// NewTracer creates a new tracer for the given service name
func NewTracer(serviceName string) *Tracer {
	return &Tracer{
		serviceName: serviceName,
		tracer:      otel.GetTracerProvider().Tracer(serviceName),
	}
}

// InitTracer initializes the OpenTelemetry tracer provider
func InitTracer(ctx context.Context, serviceName, serviceVersion, exporterEndpoint string, samplingRatio float64) (func(), error) {
	if exporterEndpoint == "" {
		exporterEndpoint = os.Getenv("OTLP_EXPORTER_ENDPOINT")
		if exporterEndpoint == "" {
			exporterEndpoint = "otel-collector:4317" // Default endpoint
		}
	}

	// Create OTLP exporter
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(exporterEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create resource with service information
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String(serviceVersion),
			attribute.String("environment", os.Getenv("ENVIRONMENT")),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(samplingRatio)),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Set global tracer provider and propagator
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Return shutdown function
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			fmt.Printf("Error shutting down tracer provider: %v\n", err)
		}
	}, nil
}

// StartSpan starts a new span with the given name and attributes
func (t *Tracer) StartSpan(ctx context.Context, name string, attributes map[string]string) (context.Context, trace.Span) {
	attrs := make([]attribute.KeyValue, 0, len(attributes))
	for k, v := range attributes {
		attrs = append(attrs, attribute.String(k, v))
	}
	return t.tracer.Start(ctx, fmt.Sprintf("%s.%s", t.serviceName, name), trace.WithAttributes(attrs...))
}

// ContextWithAttributes adds attributes to the current span
func (t *Tracer) ContextWithAttributes(ctx context.Context, attributes map[string]string) context.Context {
	span := trace.SpanFromContext(ctx)
	for k, v := range attributes {
		span.SetAttributes(attribute.String(k, v))
	}
	return ctx
}

// RecordError records an error in the current span
func (t *Tracer) RecordError(ctx context.Context, err error) {
	span := trace.SpanFromContext(ctx)
	span.RecordError(err)
}

// AddEvent adds an event to the current span
func (t *Tracer) AddEvent(ctx context.Context, name string, attributes map[string]string) {
	span := trace.SpanFromContext(ctx)
	attrs := make([]attribute.KeyValue, 0, len(attributes))
	for k, v := range attributes {
		attrs = append(attrs, attribute.String(k, v))
	}
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// SetStatus sets the status of the current span
func (t *Tracer) SetStatus(ctx context.Context, code trace.StatusCode, description string) {
	span := trace.SpanFromContext(ctx)
	span.SetStatus(code, description)
}

// GetTraceID returns the trace ID for the current span
func (t *Tracer) GetTraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return ""
	}
	return span.SpanContext().TraceID().String()
}

// GetSpanID returns the span ID for the current span
func (t *Tracer) GetSpanID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return ""
	}
	return span.SpanContext().SpanID().String()
}