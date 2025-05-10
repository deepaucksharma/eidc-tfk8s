# Chaos Testing Runbook

This runbook provides guidelines for running chaos tests against the NRDOT+ Internal Dev-Lab.

## Overview

Chaos testing helps us validate the resilience of our system by introducing controlled failures and observing the system's ability to recover. The chaos tests are defined in YAML files and are executed using Litmus Chaos.

## Prerequisites

- A running Kubernetes cluster with NRDOT+ deployed
- Litmus Chaos installed on the cluster
- Access to monitoring tools to observe the system during tests

## Test Scenarios

Each test scenario is linked to a specific requirement in the [Test Matrix](../appendices/appendix-a-test-matrix.md).

### Network Partition Tests

| Scenario | Description | Linked Test ID |
|----------|-------------|----------------|
| Network Partition Between FB-RX and FB-EN-HOST | Introduces network partition between FB-RX and FB-EN-HOST | NFR-RSL-01 |
| Network Partition Between FB-GW and Endpoint | Introduces network partition between FB-GW and export endpoint | NFR-RSL-01 |

### Pod Failure Tests

| Scenario | Description | Linked Test ID |
|----------|-------------|----------------|
| FB-RX Pod Failure | Kills FB-RX pod | NFR-RSL-01 |
| FB-DP Pod Failure | Kills FB-DP pod | NFR-RSL-01 |
| FB-GW Pod Failure | Kills FB-GW pod | NFR-RSL-01 |

### Resource Exhaustion Tests

| Scenario | Description | Linked Test ID |
|----------|-------------|----------------|
| CPU Stress on FB-DP | Introduces CPU stress on FB-DP | NFR-RSL-03 |
| Memory Stress on FB-DP | Introduces memory stress on FB-DP | NFR-RSL-03 |
| Disk Stress on FB-DP | Introduces disk stress on FB-DP | NFR-RSL-03 |

### Configuration Change Tests

| Scenario | Description | Linked Test ID |
|----------|-------------|----------------|
| Rapid Configuration Changes | Rapidly changes configuration of function blocks | NFR-CFG-01 |

## Running Chaos Tests

### Using CI Pipeline

The chaos tests are automatically run as part of the CI pipeline on the `main` branch.

### Running Manually

1. Deploy NRDOT+ to a test environment:
   ```bash
   helm install nrdot-devlab ./deploy/helm --values ./deploy/helm/values-lab.yaml
   ```

2. Install Litmus Chaos:
   ```bash
   helm repo add litmuschaos https://litmuschaos.github.io/litmus-helm/
   helm install chaos litmuschaos/litmus --namespace=litmus --create-namespace
   ```

3. Create chaos experiment:
   ```bash
   kubectl apply -f deploy/test/tf-k8s/resilience/chaos_tests.yaml
   ```

4. Monitor the test results:
   ```bash
   kubectl get chaosresult -n nrdot
   ```

## Interpreting Results

After running a chaos test, check the following:

1. Whether the system continued to function during the chaos
2. Whether the system recovered after the chaos ended
3. Whether any data was lost during the chaos
4. Whether circuit breakers and DLQs worked as expected

Any failures should be documented and addressed before releasing.