# Annex E – Migration Guide (v0.125 → v0.136.1-nr1 → EIDC v1.2)

This guide helps operators upgrade an **existing vanilla OTel Collector
0.125 deployment** (or NRDOT+ v1.0) to the fully-optimised **NRDOT+ v1.2**
collector.

---

## E-1 · Version Bumps

| Component          | From           | ➜ | To (NRDOT+ v1.2)        |
|--------------------|---------------|---|-------------------------|
| OTel Collector     | `0.125`       |   | `0.136.1-nr1` *(NR fork)*|
| hostmetrics scrapers| legacy fields|   | new typed metrics (v0.134+)|
| Edge-Probe         | n/a           |   | **v1.1** optional       |

---

## E-2 · Key Config Diffs

### 1 · Receiver stanza

```diff
-receivers:
-  hostmetrics:
-    collection_interval: 10s
+receivers:
+  hostmetrics:
+    collection_interval: 10s
+    scrapers:
+      cpu: {}
+      memory: {}
+      processes: {}
+      process: {}
```

### 2 · New processors

* `cumulativetodelta` now REQUIRED
* `transform` / `filter` blocks replace older `attributes` hacks
* `routing` block splits "other" aggregation path

### 3 · Deprecated → Removed

| Legacy                                                | Replacement                                  |
| ----------------------------------------------------- | -------------------------------------------- |
| `metricstransform.op: sum` on **cumulative** counters | run `cumulativetodelta` *before* aggregation |
| `attributes.actions: add_hash`                        | use `transform` with `SHA256()`              |

---

## E-3 · Step-by-step Upgrade

1. **Dry-run** NRDOT+ v1.2 with `otlphttp` exporter pointed to a *staging*
   New Relic account.
2. Verify baseline → v1.2 SLOs with `TF-K8s --scenario slo-core`.
3. Enable the new collector alongside existing; mirror traffic with
   `pipeline_copy` pattern (optional).
4. Flip production ingest DNS to NRDOT+ endpoint.
5. Remove legacy collector.

---

## E-4 · Automated Lint

The repo ships `scripts/lint-config.py` to:

* flag unsupported processor names
* ensure required OTTL helpers are loaded
* check `error_mode` settings

Run:

```bash
python scripts/lint-config.py your-config.yaml
```

---

## E-5 · Rollback

If functional SLOs regress, execute:

```bash
helm rollback nrdotplus-collector <previous_revision>
```

then file `CCB-HOTFIX-EIDC` with observed metrics + logs.

---

*(end of file)*