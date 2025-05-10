---
title: "Custom Resource Definition (CRD) Schema"
updated: "2025-05-10"
toc: true
---

# NRDOT+ Custom Resource Definition (CRD) Schema

This document describes the Custom Resource Definition (CRD) schema used by NRDOT+ for pipeline configuration.

## Overview

The `NRDOTPlusPipeline` CRD defines the configuration for a telemetry processing pipeline in the NRDOT+ system. It specifies the function blocks, their configurations, and how they are connected.

## Schema

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: nrdotpluspipelines.nrdot.newrelic.com
spec:
  group: nrdot.newrelic.com
  names:
    kind: NRDOTPlusPipeline
    listKind: NRDOTPlusPipelineList
    plural: nrdotpluspipelines
    singular: nrdotpluspipeline
    shortNames:
      - nrpl
  scope: Namespaced
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              required:
                - functionBlocks
              properties:
                functionBlocks:
                  type: array
                  items:
                    type: object
                    required:
                      - name
                      - type
                    properties:
                      name:
                        type: string
                        description: "Name of the function block"
                      type:
                        type: string
                        enum:
                          - fb-rx
                          - fb-en-host
                          - fb-en-k8s
                          - fb-cl
                          - fb-dp
                          - fb-fs
                          - fb-agg
                          - fb-gw-pre
                          - fb-gw
                          - fb-dlq
                        description: "Type of the function block"
                      config:
                        type: object
                        description: "Configuration for the function block"
                        properties:
                          general:
                            type: object
                            description: "General configuration for the function block"
                            properties:
                              logLevel:
                                type: string
                                enum:
                                  - debug
                                  - info
                                  - warn
                                  - error
                                description: "Log level for the function block"
                              maxBatchSize:
                                type: integer
                                minimum: 1
                                maximum: 10000
                                description: "Maximum batch size for processing"
                              circuitBreaker:
                                type: object
                                description: "Circuit breaker configuration"
                                properties:
                                  enabled:
                                    type: boolean
                                    description: "Whether circuit breaking is enabled"
                                  maxFailures:
                                    type: integer
                                    minimum: 1
                                    description: "Maximum number of failures before tripping"
                                  resetTimeoutSeconds:
                                    type: integer
                                    minimum: 1
                                    description: "Timeout in seconds before resetting the circuit breaker"
                              dlq:
                                type: object
                                description: "Dead letter queue configuration"
                                properties:
                                  enabled:
                                    type: boolean
                                    description: "Whether DLQ is enabled"
                                  endpoint:
                                    type: string
                                    description: "Endpoint for the DLQ"
                          specific:
                            type: object
                            description: "Function block specific configuration"
                      connections:
                        type: array
                        description: "Connections to other function blocks"
                        items:
                          type: object
                          required:
                            - target
                          properties:
                            target:
                              type: string
                              description: "Name of the target function block"
                            filter:
                              type: string
                              description: "Optional filter expression for the connection"
                      resources:
                        type: object
                        description: "Resource requirements for the function block"
                        properties:
                          limits:
                            type: object
                            description: "Resource limits"
                            properties:
                              cpu:
                                type: string
                                description: "CPU limit"
                              memory:
                                type: string
                                description: "Memory limit"
                          requests:
                            type: object
                            description: "Resource requests"
                            properties:
                              cpu:
                                type: string
                                description: "CPU request"
                              memory:
                                type: string
                                description: "Memory request"
                observability:
                  type: object
                  description: "Observability configuration for the pipeline"
                  properties:
                    metrics:
                      type: object
                      description: "Metrics configuration"
                      properties:
                        enabled:
                          type: boolean
                          description: "Whether metrics are enabled"
                        endpoint:
                          type: string
                          description: "Endpoint for metrics"
                    traces:
                      type: object
                      description: "Traces configuration"
                      properties:
                        enabled:
                          type: boolean
                          description: "Whether traces are enabled"
                        endpoint:
                          type: string
                          description: "Endpoint for traces"
                    logs:
                      type: object
                      description: "Logs configuration"
                      properties:
                        enabled:
                          type: boolean
                          description: "Whether structured logs are enabled"
                        endpoint:
                          type: string
                          description: "Endpoint for logs"
            status:
              type: object
              properties:
                phase:
                  type: string
                  description: "Current phase of the pipeline"
                  enum:
                    - Pending
                    - Deploying
                    - Running
                    - Failed
                message:
                  type: string
                  description: "Status message"
                functionBlockStatuses:
                  type: array
                  description: "Status of each function block"
                  items:
                    type: object
                    properties:
                      name:
                        type: string
                        description: "Name of the function block"
                      status:
                        type: string
                        description: "Status of the function block"
                      message:
                        type: string
                        description: "Status message"
                      lastUpdated:
                        type: string
                        format: date-time
                        description: "Last update time"
```

## Example

Here is an example of a `NRDOTPlusPipeline` custom resource:

```yaml
apiVersion: nrdot.newrelic.com/v1
kind: NRDOTPlusPipeline
metadata:
  name: example-pipeline
  namespace: nrdot
spec:
  functionBlocks:
    - name: receiver
      type: fb-rx
      config:
        specific:
          ports:
            otlpGrpc: 4317
            otlpHttp: 4318
            prometheusRemoteWrite: 9411
      connections:
        - target: host-enricher
    
    - name: host-enricher
      type: fb-en-host
      config:
        general:
          logLevel: info
          maxBatchSize: 1000
      connections:
        - target: k8s-enricher
    
    - name: k8s-enricher
      type: fb-en-k8s
      config:
        specific:
          k8sApiTimeout: 5s
      connections:
        - target: classifier
    
    - name: classifier
      type: fb-cl
      config:
        specific:
          piiFields:
            - email
            - username
            - ipAddress
      connections:
        - target: deduplicator
    
    - name: deduplicator
      type: fb-dp
      config:
        specific:
          storageType: badgerdb
          ttlMinutes: 60
      connections:
        - target: filter-sampler
    
    - name: filter-sampler
      type: fb-fs
      config:
        specific:
          filters:
            - pattern: "status=error"
              sampleRate: 1.0
            - pattern: "environment=dev"
              sampleRate: 0.1
      connections:
        - target: aggregator
    
    - name: aggregator
      type: fb-agg
      config:
        specific:
          windowSeconds: 60
          aggregations:
            - metric: "http_requests_total"
              type: "sum"
      connections:
        - target: pre-gateway
    
    - name: pre-gateway
      type: fb-gw-pre
      connections:
        - target: gateway
    
    - name: gateway
      type: fb-gw
      config:
        specific:
          schemaValidation: true
          exportEndpoints:
            - name: "prometheus"
              type: "prometheus"
              url: "http://prometheus:9090/api/v1/write"
      connections:
        - target: dlq
    
    - name: dlq
      type: fb-dlq
      config:
        specific:
          storage: "memory"
          retentionHours: 24
  
  observability:
    metrics:
      enabled: true
      endpoint: "http://prometheus:9090"
    traces:
      enabled: true
      endpoint: "http://jaeger:16686"
    logs:
      enabled: true
      endpoint: "http://loki:3100"
```

## Usage

To apply a pipeline configuration:

```bash
kubectl apply -f pipeline.yaml
```

To get the status of a pipeline:

```bash
kubectl get nrdotpluspipeline example-pipeline -n nrdot -o yaml
```

## Validation

The CRD includes validation to ensure that pipeline configurations are valid. Invalid configurations will be rejected by the Kubernetes API server.

## Status Updates

The ConfigController updates the status of the pipeline as it is deployed and as function blocks report their status. The status includes:

- The current phase of the pipeline
- A status message
- The status of each function block
- The last update time for each function block
