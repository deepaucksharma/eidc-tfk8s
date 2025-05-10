package fb

import (
	"context"
	"errors"
)

// Common errors
var (
	ErrConfigInvalid      = errors.New("invalid configuration")
	ErrProcessingFailed   = errors.New("processing failed")
	ErrForwardingFailed   = errors.New("forwarding failed")
	ErrCircuitBreakerOpen = errors.New("circuit breaker is open")
	ErrDLQSendFailed      = errors.New("failed to send to DLQ")
	ErrShutdownTimeout    = errors.New("shutdown timed out")
)

// ErrorCode represents an error code for standardized error handling
type ErrorCode string

// Common error codes
const (
	ErrorCodeUnknown              ErrorCode = "ERR_UNKNOWN"
	ErrorCodeInvalidInput         ErrorCode = "ERR_INVALID_INPUT"
	ErrorCodeInvalidConfig        ErrorCode = "ERR_INVALID_CONFIG"
	ErrorCodeProcessingFailed     ErrorCode = "ERR_PROCESSING_FAILED"
	ErrorCodeForwardingFailed     ErrorCode = "ERR_FORWARDING_FAILED"
	ErrorCodeCircuitBreakerOpen   ErrorCode = "ERR_CIRCUIT_BREAKER_OPEN"
	ErrorCodeDLQSendFailed        ErrorCode = "ERR_DLQ_SEND_FAILED"
	ErrorCodePoisonBatch          ErrorCode = "ERR_POISON_BATCH"
	ErrorCodePIILeak              ErrorCode = "ERR_PII_LEAK"
	ErrorCodeThrottled            ErrorCode = "ERR_THROTTLED"
	ErrorCodeServiceUnavailable   ErrorCode = "ERR_SERVICE_UNAVAILABLE"
	ErrorCodeTimeout              ErrorCode = "ERR_TIMEOUT"
)

// FunctionBlock defines the interface that all function blocks must implement
type FunctionBlock interface {
	// Name returns the name of the function block
	Name() string

	// Initialize initializes the function block
	Initialize(ctx context.Context) error

	// ProcessBatch processes a batch of data
	ProcessBatch(ctx context.Context, batch *MetricBatch) (*ProcessResult, error)

	// UpdateConfig updates the function block's configuration
	UpdateConfig(ctx context.Context, configBytes []byte, generation int64) error

	// Ready returns whether the function block is ready to process data
	Ready() bool

	// Shutdown shuts down the function block
	Shutdown(ctx context.Context) error
}

// MetricBatch represents a batch of metrics being processed
type MetricBatch struct {
	// Unique identifier for this batch
	BatchID string

	// The serialized metric batch data
	Data []byte

	// Format of the data (e.g., "otlp", "prometheus")
	Format string

	// Whether this is a replay from DLQ
	Replay bool

	// Configuration generation applied to this batch
	ConfigGeneration int64

	// Metadata for processing
	Metadata map[string]string

	// Internal labels for pipeline processing
	InternalLabels map[string]string
}

// ProcessResult represents the result of processing a batch
type ProcessResult struct {
	// Status of the operation
	Status Status

	// Error message, if any
	ErrorMessage string

	// Error code, if any
	ErrorCode ErrorCode

	// Batch ID echo
	BatchID string

	// Whether the batch was sent to DLQ
	SentToDLQ bool
}

// Status represents the status of a processing operation
type Status int

const (
	// StatusUnknown means the status is unknown
	StatusUnknown Status = iota

	// StatusSuccess means the operation was successful
	StatusSuccess

	// StatusError means the operation failed
	StatusError

	// StatusThrottled means the operation was throttled
	StatusThrottled
)

// String returns the string representation of the status
func (s Status) String() string {
	switch s {
	case StatusSuccess:
		return "SUCCESS"
	case StatusError:
		return "ERROR"
	case StatusThrottled:
		return "THROTTLED"
	default:
		return "UNKNOWN"
	}
}

// BaseFunctionBlock provides common functionality for all function blocks
type BaseFunctionBlock struct {
	name              string
	ready             bool
	configGeneration  int64
}

// NewBaseFunctionBlock creates a new BaseFunctionBlock with the given name
func NewBaseFunctionBlock(name string) BaseFunctionBlock {
	return BaseFunctionBlock{
		name:  name,
		ready: false,
	}
}

// Name returns the name of the function block
func (b *BaseFunctionBlock) Name() string {
	return b.name
}

// SetReady sets the ready state of the function block
func (b *BaseFunctionBlock) SetReady(ready bool) {
	b.ready = ready
}

// Ready returns whether the function block is ready to process data
func (b *BaseFunctionBlock) Ready() bool {
	return b.ready
}

// SetConfigGeneration sets the configuration generation
func (b *BaseFunctionBlock) SetConfigGeneration(generation int64) {
	b.configGeneration = generation
}

// GetConfigGeneration returns the current configuration generation
func (b *BaseFunctionBlock) GetConfigGeneration() int64 {
	return b.configGeneration
}

// NewErrorResult creates a new error processing result
func NewErrorResult(batchID string, errCode ErrorCode, err error, sentToDLQ bool) *ProcessResult {
	var errMsg string
	if err != nil {
		errMsg = err.Error()
	}

	return &ProcessResult{
		Status:       StatusError,
		ErrorMessage: errMsg,
		ErrorCode:    errCode,
		BatchID:      batchID,
		SentToDLQ:    sentToDLQ,
	}
}

// NewSuccessResult creates a new success processing result
func NewSuccessResult(batchID string) *ProcessResult {
	return &ProcessResult{
		Status:  StatusSuccess,
		BatchID: batchID,
	}
}

// NewThrottledResult creates a new throttled processing result
func NewThrottledResult(batchID string) *ProcessResult {
	return &ProcessResult{
		Status:  StatusThrottled,
		BatchID: batchID,
	}
}