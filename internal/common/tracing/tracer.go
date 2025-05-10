package tracing

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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
	traceID     string
}

// NewTracer creates a new tracer for the given service
func NewTracer(serviceName string) *Tracer {
	return &Tracer{
		serviceName: serviceName,
	}
}

// InitTracer initializes the OpenTelemetry tracer with OTLP exporter
func (t *Tracer) InitTracer(ctx context.Context, endpoint string, samplingRatio float64) error {
	// Create exporter
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint(endpoint),
	)
	if err != nil {
		return fmt.Errorf("failed to create exporter: %w", err)
	}

	// Get hostname for resource
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// Create resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(t.serviceName),
			attribute.String("host.name", hostname),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(samplingRatio)),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Set as global tracer provider
	otel.SetTracerProvider(tp)

	// Set propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return nil
}

// StartSpan starts a new span
func (t *Tracer) StartSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	tracer := otel.Tracer(t.serviceName)
	return tracer.Start(ctx, name)
}

// AddEvent adds an event to the current span
func (t *Tracer) AddEvent(ctx context.Context, name string, attrs map[string]string) {
	span := trace.SpanFromContext(ctx)
	attributes := make([]attribute.KeyValue, 0, len(attrs))
	for k, v := range attrs {
		attributes = append(attributes, attribute.String(k, v))
	}
	span.AddEvent(name, trace.WithAttributes(attributes...))
}

// AddAttributes adds attributes to the current span
func (t *Tracer) AddAttributes(ctx context.Context, attrs map[string]string) {
	span := trace.SpanFromContext(ctx)
	for k, v := range attrs {
		span.SetAttributes(attribute.String(k, v))
	}
}

// End ends the current span
func (t *Tracer) End(ctx context.Context) {
	span := trace.SpanFromContext(ctx)
	span.End()
}

// RecordError records an error in the current span
func (t *Tracer) RecordError(ctx context.Context, err error) {
	span := trace.SpanFromContext(ctx)
	span.RecordError(err)
}

// SetStatus sets the status of the current span
func (t *Tracer) SetStatus(ctx context.Context, code codes.Code, description string) {
	span := trace.SpanFromContext(ctx)
	span.SetStatus(code, description)
}

// AddDuration adds a duration attribute to the current span
func (t *Tracer) AddDuration(ctx context.Context, key string, duration time.Duration) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attribute.Int64(key, duration.Milliseconds()))
}

// TraceID gets the trace ID from the current span
func (t *Tracer) TraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	return span.SpanContext().TraceID().String()
}

// SpanFromContext gets the span from the current context
func (t *Tracer) SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// ContextWithSpan creates a new context with the given span
func (t *Tracer) ContextWithSpan(ctx context.Context, span trace.Span) context.Context {
	return trace.ContextWithSpan(ctx, span)
}
