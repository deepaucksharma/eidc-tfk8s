# Annex B – Illustrative OTel Collector Config Snippets  
*(EIDC v1.2, Smart-Collector Stage-5 Pipeline)*

> **NON-normative**: these fragments demonstrate **how** to realise each logical
> stage of the Stage-5 pipeline using standard OTel processors on
> `otelcol-contrib v0.136.1-nr1`.  
>   
> The *normative* definition of behaviour is §6 of
> `EIDC-NRDOT+-FinalBlueprint.md`.

## B-1 · Receivers & High-level Service Skeleton

```yaml
receivers:
  hostmetrics:
    collection_interval: 10s
    scrapers: { cpu: {}, memory: {}, processes: {}, process: {} }

  prometheus/edge:
    config:
      scrape_configs:
        - job_name: "nr-edge-probe"
          scrape_interval: 15s
          static_configs: [ { targets: [ "localhost:9999" ] } ]

  otlp:
    protocols:
      grpc: {}
      http: {}

extensions:
  health_check: {}
  pprof: { endpoint: :1777 }
  zpages: { endpoint: :55679 }
```

---

## B-2 · Stage S1 – PID normalisation & `/proc` enrichment

```yaml
processors:
  transform/pid_enrich_normalize:
    error_mode: ignore
    trace_statements: []   # kept empty; metrics only
    metric_statements:
      - context: datapoint
        statements:
          # 1 · convert LA millisecond start time ➜ nanoseconds
          - set(attributes["process.start_time_ns"]) =
              MultiplyInt(attributes["process.runtime.jvm.start_time_ms"], 1_000_000)
              where IsSet(attributes["process.runtime.jvm.start_time_ms"])
          # 2 · fallback: read /proc when hostmetrics lacked start_time_ns
          - set(attributes["process.start_time_ns"]) =
              PidStartTimeNs(attributes["process.pid"])
              where name matches "^process\\." and
                    (attributes["otel.metrics.source"] == "hostmetrics") and
                    Not(IsSet(attributes["process.start_time_ns"]))
          # 3 · still missing? build boot-id fallback key
          - set(attributes["process.custom.boot_id_ref"]) =
              HostBootID()
              where Not(IsSet(attributes["process.start_time_ns"]))
          # 4 · derive basename of executable
          - set(attributes["process.executable.name"]) =
              Basename(attributes["process.executable.path"])
              where IsSet(attributes["process.executable.path"])
```

> **Helper functions used** (`PidStartTimeNs`, `HostBootID`) are compiled-in
> Go helpers exposed to OTTL in New-Relic fork `0.136.1-nr1`.

---

## B-3 · Stage S2 – Source-dedup filter

```yaml
processors:
  filter/deduplicate_sources:
    error_mode: ignore
    metrics:
      metric:
        # Drop lower-priority hostmetrics datapoints when LA already covers pid
        - |
          IsMatch(name, "^process\\.") and
          attributes["otel.metrics.source"] == "hostmetrics" and
          SourceCacheContains(
            "la",                              # cache-bucket
            attributes["host.name"],
            attributes["process.pid"],
            Coalesce(attributes["process.start_time_ns"],
                     attributes["process.custom.boot_id_ref"])
          )
        # Cache insert rules (executed via 'Action' directive in NR fork)
        - action: cacheput
          key_fields:
            - attributes["host.name"]
            - attributes["process.pid"]
            - Coalesce(attributes["process.start_time_ns"],
                       attributes["process.custom.boot_id_ref"])
          value: "la"
          where attributes["otel.metrics.source"] == "language_agent"
```

---

## B-4 · Stage S3 – Classification & PII hygiene

```yaml
processors:
  transform/classify_sanitize:
    error_mode: ignore
    metric_statements:
      - context: resource
        statements:
          - |
            set(attributes["process.custom.classification"], "system_daemon")
            where attributes["process.owner"] == "root" and
                  IsMatch(attributes["process.executable.name"], "^(sshd|systemd|dockerd)$")

      - context: datapoint
        statements:
          # Hash + drop raw CLI
          - set(attributes["process.custom.command_line_hash"]) =
              SHA256(attributes["process.command_line"])
              where IsSet(attributes["process.command_line"])
          - delete_key(attributes, "process.command_line")
          - delete_key(attributes, "process.command_args")
```

---

## B-5 · Stage S4 – Intelligent filtering / 1-in-N sampling

```yaml
processors:
  filter/intelligent_filter_sample:
    error_mode: ignore
    metrics:
      metric:
        # ultra-low CPU drop for "other_ephemeral"
        - |
          name == "process.cpu.utilization" and
          attributes["process.custom.classification"] == "other_ephemeral" and
          value_double < 0.0005

        # 1-in-5 deterministic sample on other_persistent
        - |
          name matches "^process\\." and
          attributes["process.custom.classification"] == "other_persistent" and
          Mod(Hash(attributes["process.pid"]), 5) != 0
```

---

## B-6 · Stage S5 – Delta-conversion then "other" aggregation

```yaml
processors:
  cumulativetodelta:
    include:
      match_type: regexp
      metric_names:
        - ".*cpu\\.time"
        - ".*disk\\.io"
        - ".*network\\.io"

  routing/aggregate_others:
    table:
      - statement: |
          attributes["process.custom.classification"] in ("other_persistent","other_ephemeral")
        pipelines: [ metrics/others ]

        # default route
      - pipelines: [ metrics/default ]

  metricstransform/aggregate_other_metrics:
    transforms:
      - include: "process.cpu.utilization"
        new_name: "process.aggregated.other.cpu.utilization"
        operations:
          - action: aggregate_labels
            label_set:
              - resource.attributes.host.name
              - attributes.process.custom.classification
            aggregation_type: mean

      - include: "process.memory.usage"
        new_name: "process.aggregated.other.memory.usage"
        operations:
          - action: aggregate_labels
            label_set:
              - resource.attributes.host.name
              - attributes.process.custom.classification
            aggregation_type: sum
```

---

## B-7 · Stage S6 – Final schema-enforce & export

```yaml
processors:
  filter/final_schema:
    error_mode: ignore
    metrics:
      metric:
        - |
          not SchemaMatch(name)   # drop anything outside YAML schema

exporters:
  otlphttp:
    endpoint: https://otlp.nr-data.net:4318
    headers: { api-key: "${NEW_RELIC_LICENSE_KEY}" }
    compression: gzip
    sending_queue: { enabled: true, num_consumers: 4, queue_size: 2000 }
```

*(`SchemaMatch()` helper lives in the NR fork and references
`SmartCollector_Output_Metrics_v1.2.yaml` at startup.)*

---

## B-8 · End-to-end Service Pipelines (summary)

```yaml
service:
  pipelines:
    metrics/host_process_optimized:
      receivers: [ hostmetrics, prometheus/edge ]
      processors:
        - memory_limiter
        - batch
        - resourcedetection
        - transform/pid_enrich_normalize
        - filter/deduplicate_sources
        - transform/classify_sanitize
        - filter/intelligent_filter_sample
        - cumulativetodelta
        - routing/aggregate_others
        - filter/final_schema
        - batch
      exporters: [ otlphttp ]
```

---

*(end of file)*