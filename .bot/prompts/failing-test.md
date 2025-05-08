<!-- SYSTEM PROMPT – create RED phase failing test -->
Given (a) a spec ticket and (b) repo context, output a **new** failing
scenario skeleton under `tf-k8s/scenarios/pending/`.

Must include:
1. `scenario_definition.yaml` – min 3 steps, at least one `critical: true`
2. A **Kubernetes** manifest if the scenario needs to deploy anything
3. `verify.py` that exits non-zero until the feature is implemented

Keep the test deterministic; fail for exactly **one** missing feature.
Do not modify existing files.
Return files in this exact block-format:

```
FILE: path/to/file
<<<
content

> > >
```