---
title: "Function Block Interface"
updated: "2025-05-10"
toc: true
---

# Function Block Interface

This document describes the interface that all Function Blocks must implement.

## Overview

Function Blocks (FBs) in NRDOT+ are modular components that implement specific processing capabilities in the telemetry pipeline. Each FB communicates with other FBs through standardized interfaces.

## Core Interfaces

### FunctionBlock Interface

```go
// FunctionBlock defines the base interface that all function blocks must implement
type FunctionBlock interface {
    // Start initializes and starts the function block
    Start(ctx context.Context) error
    
    // Stop gracefully stops the function block
    Stop(ctx context.Context) error
    
    // Status returns the current status of the function block
    Status() *StatusInfo
    
    // GetMetrics returns the current metrics of the function block
    GetMetrics() *MetricsInfo
    
    // ID returns the unique identifier for this function block
    ID() string
    
    // Type returns the type of this function block (e.g., "rx", "dp", "gw")
    Type() string
    
    // Version returns the version of this function block
    Version() string
}
```

### Data Processing Interface

```go
// DataProcessor defines the interface for processing telemetry data
type DataProcessor interface {
    // Process processes a batch of telemetry data and returns the processed data
    Process(ctx context.Context, data *TelemetryBatch) (*TelemetryBatch, error)
    
    // ProcessAsync processes a batch of telemetry data asynchronously
    ProcessAsync(ctx context.Context, data *TelemetryBatch, callback ProcessCallback) error
}
```

### Configuration Interface

```go
// Configurable defines the interface for configurable function blocks
type Configurable interface {
    // ApplyConfig applies a new configuration to the function block
    ApplyConfig(ctx context.Context, config *ConfigData) error
    
    // GetConfig returns the current configuration of the function block
    GetConfig() *ConfigData
    
    // ValidateConfig validates a configuration without applying it
    ValidateConfig(config *ConfigData) error
}
```

## gRPC Services

### ChainPushService

The ChainPushService is a gRPC service that function blocks implement to receive telemetry data from upstream function blocks.

```protobuf
syntax = "proto3";

package nrdot.fb;

service ChainPushService {
    // PushTelemetry pushes telemetry data to the function block
    rpc PushTelemetry(PushTelemetryRequest) returns (PushTelemetryResponse);
    
    // PushTelemetryStream pushes telemetry data as a stream
    rpc PushTelemetryStream(stream PushTelemetryRequest) returns (stream PushTelemetryResponse);
}

message PushTelemetryRequest {
    // Serialized telemetry batch
    bytes data = 1;
    
    // Metadata for the telemetry batch
    map<string, string> metadata = 2;
    
    // Trace context for the telemetry batch
    TraceContext traceContext = 3;
}

message PushTelemetryResponse {
    // Status code
    int32 status = 1;
    
    // Error message, if any
    string error = 2;
    
    // Number of items processed
    int32 processed = 3;
}

message TraceContext {
    // Trace ID
    string traceId = 1;
    
    // Span ID
    string spanId = 2;
    
    // Flags
    int32 flags = 3;
}
```

### ConfigControllerService

The ConfigControllerService is a gRPC service that the ConfigController implements to provide configuration to function blocks.

```protobuf
syntax = "proto3";

package nrdot.config;

service ConfigControllerService {
    // GetConfig returns the current configuration for a function block
    rpc GetConfig(GetConfigRequest) returns (GetConfigResponse);
    
    // WatchConfig watches for configuration changes
    rpc WatchConfig(WatchConfigRequest) returns (stream ConfigUpdate);
    
    // ReportStatus reports the status of a function block
    rpc ReportStatus(ReportStatusRequest) returns (ReportStatusResponse);
}

message GetConfigRequest {
    // Function block ID
    string fbId = 1;
    
    // Function block type
    string fbType = 2;
}

message GetConfigResponse {
    // Configuration data
    bytes configData = 1;
    
    // Configuration version
    string version = 2;
}

message WatchConfigRequest {
    // Function block ID
    string fbId = 1;
    
    // Function block type
    string fbType = 2;
    
    // Current config version
    string currentVersion = 3;
}

message ConfigUpdate {
    // New configuration data
    bytes configData = 1;
    
    // New configuration version
    string version = 2;
}

message ReportStatusRequest {
    // Function block ID
    string fbId = 1;
    
    // Function block type
    string fbType = 2;
    
    // Status information
    StatusInfo status = 3;
    
    // Configuration version
    string configVersion = 4;
}

message ReportStatusResponse {
    // Acknowledgement
    bool acknowledged = 1;
}

message StatusInfo {
    // Status code
    int32 statusCode = 1;
    
    // Status message
    string statusMessage = 2;
    
    // Is healthy
    bool healthy = 3;
    
    // Additional status details
    map<string, string> details = 4;
}
```

## Data Structures

### TelemetryBatch

```go
// TelemetryBatch represents a batch of telemetry data
type TelemetryBatch struct {
    // Metrics contains metric data
    Metrics []*Metric `json:"metrics,omitempty"`
    
    // Traces contains trace data
    Traces []*Trace `json:"traces,omitempty"`
    
    // Logs contains log data
    Logs []*Log `json:"logs,omitempty"`
    
    // Metadata contains metadata for the batch
    Metadata map[string]string `json:"metadata,omitempty"`
}
```

### ConfigData

```go
// ConfigData represents configuration data for a function block
type ConfigData struct {
    // General configuration
    General *GeneralConfig `json:"general,omitempty"`
    
    // Function block specific configuration
    Specific map[string]interface{} `json:"specific,omitempty"`
    
    // Version of the configuration
    Version string `json:"version,omitempty"`
}

// GeneralConfig represents general configuration common to all function blocks
type GeneralConfig struct {
    // LogLevel sets the log level
    LogLevel string `json:"logLevel,omitempty"`
    
    // MaxBatchSize sets the maximum batch size for processing
    MaxBatchSize int `json:"maxBatchSize,omitempty"`
    
    // CircuitBreaker configuration
    CircuitBreaker *CircuitBreakerConfig `json:"circuitBreaker,omitempty"`
    
    // DLQ configuration
    DLQ *DLQConfig `json:"dlq,omitempty"`
}
```

## Usage Example

Here is a basic example of implementing a Function Block:

```go
package myfb

import (
    "context"
    
    "github.com/newrelic/nrdot-internal-devlab/pkg/fb"
)

// MyFB is a custom function block
type MyFB struct {
    id      string
    fbType  string
    version string
    config  *fb.ConfigData
    // ... other fields
}

// NewMyFB creates a new instance of MyFB
func NewMyFB(id string) *MyFB {
    return &MyFB{
        id:      id,
        fbType:  "my-fb",
        version: "1.0.0",
        config:  fb.DefaultConfig(),
        // ... initialize other fields
    }
}

// Start implements the FunctionBlock interface
func (m *MyFB) Start(ctx context.Context) error {
    // Implementation
    return nil
}

// Stop implements the FunctionBlock interface
func (m *MyFB) Stop(ctx context.Context) error {
    // Implementation
    return nil
}

// Status implements the FunctionBlock interface
func (m *MyFB) Status() *fb.StatusInfo {
    // Implementation
    return &fb.StatusInfo{
        StatusCode:    0,
        StatusMessage: "OK",
        Healthy:       true,
    }
}

// GetMetrics implements the FunctionBlock interface
func (m *MyFB) GetMetrics() *fb.MetricsInfo {
    // Implementation
    return &fb.MetricsInfo{
        // ... metrics data
    }
}

// ID implements the FunctionBlock interface
func (m *MyFB) ID() string {
    return m.id
}

// Type implements the FunctionBlock interface
func (m *MyFB) Type() string {
    return m.fbType
}

// Version implements the FunctionBlock interface
func (m *MyFB) Version() string {
    return m.version
}

// Process implements the DataProcessor interface
func (m *MyFB) Process(ctx context.Context, data *fb.TelemetryBatch) (*fb.TelemetryBatch, error) {
    // Implementation
    return data, nil
}

// ApplyConfig implements the Configurable interface
func (m *MyFB) ApplyConfig(ctx context.Context, config *fb.ConfigData) error {
    // Implementation
    m.config = config
    return nil
}

// GetConfig implements the Configurable interface
func (m *MyFB) GetConfig() *fb.ConfigData {
    return m.config
}

// ValidateConfig implements the Configurable interface
func (m *MyFB) ValidateConfig(config *fb.ConfigData) error {
    // Implementation
    return nil
}
```
