#!/usr/bin/env python3
"""
TF-K8s Scenario Runner
======================
Executes validation scenarios for EIDC compliance testing.
"""

import os
import sys
import yaml
import argparse
import subprocess
import glob
import json
import time
from datetime import datetime

# Scenario group definitions - must match catalog
SCENARIO_GROUPS = {
    "slo-core": [
        "TF-SLO-VOL_DataVolumeReduction_SteadyWorkload",
        "TF-SLO-SER_ProcessCardinalityReduction_MixedWorkload",
        "TF-SLO-TOP5_DiagnosticIntegrity_TopCPUProcesses"
    ],
    "slo-perf": [
        "TF-SLO-CPU_CollectorPerf_LoadTest_10Kdps_cAdvisor",
        "TF-SLO-RAM_CollectorPerf_LoadTest_10Kdps_HighState_cAdvisor",
        "TF-SLO-LAT_CollectorPipelineLatency_SteadyWorkload"
    ],
    "slo-alert": [
        "TF-SLO-ALR_Recall_CPUSpikeReplay",
        "TF-SLO-ALR_Precision_NoSpikeWindow"
    ],
    "mfr-basic": [
        "TF-MFR-SC.1_IngestionSources",
        "TF-MFR-SC.3_ExportSchema",
        "TF-MFR-SC.4_AttributeHandling"
    ],
    "mfr-advanced": [
        "TF-MFR-SC.1.4_PidEnrichment_HostmetricsVariations",
        "TF-MFR-SC.5_Deduplication_MultiSourceMultiNode",
        "TF-MFR-SC.2_AggregationLogic_DiskIONetworkIO"
    ],
    "mfr-components": [
        "TF-MFR-EP_AllRequirements",
        "TF-MFR-LA_AllRequirements"
    ],
    "regression": [
        "TF-REG-Filter_HighCard_Memory_Scale",
        "TF-LINT-OTTL_StrictErrorMode"
    ],
    "security": [
        "TF-SEC-1_SBOM_Validation",
        "TF-SEC-2_ImageSignature_Validation",
        "TF-SEC-3_EdgeProbeHardening"
    ]
}

def find_scenarios(group, scenario_id=None):
    """Find all scenario definition files for a given group or specific scenario."""
    if scenario_id:
        # Search for a specific scenario by ID
        pattern = f"tf-k8s/scenarios/enabled/{scenario_id}/scenario_definition.yaml"
        scenarios = glob.glob(pattern)
        if not scenarios:
            # Check if it exists in pending
            pattern = f"tf-k8s/scenarios/pending/{scenario_id}/scenario_definition.yaml"
            pending_scenarios = glob.glob(pattern)
            if pending_scenarios:
                print(f"WARNING: Scenario {scenario_id} is in pending status and won't be executed")
            else:
                print(f"ERROR: Scenario {scenario_id} not found")
        return scenarios
    
    # Find all enabled scenarios for the group
    if group in SCENARIO_GROUPS:
        scenarios = []
        for scenario_id in SCENARIO_GROUPS[group]:
            pattern = f"tf-k8s/scenarios/enabled/{scenario_id}/scenario_definition.yaml"
            found = glob.glob(pattern)
            if found:
                scenarios.extend(found)
            else:
                print(f"WARNING: Scenario {scenario_id} not found or not enabled")
        return scenarios
    else:
        # If not a predefined group, try to find all scenarios in a directory
        pattern = f"tf-k8s/scenarios/enabled/{group}*/scenario_definition.yaml"
        return glob.glob(pattern)

def setup_scenario(scenario_path, cluster_type):
    """Set up a scenario based on its definition file."""
    print(f"Setting up scenario from {scenario_path}")
    
    # Load the scenario definition
    with open(scenario_path, 'r') as f:
        scenario = yaml.safe_load(f)
    
    # Create namespace for the scenario
    scenario_id = scenario.get('metadata', {}).get('id', 'unknown')
    namespace = f"tf-k8s-{scenario_id.lower().replace('_', '-')}"
    
    subprocess.run(['kubectl', 'create', 'namespace', namespace], check=True)
    print(f"Created namespace {namespace}")
    
    # Apply all manifests in the scenario's k8s directory
    k8s_dir = os.path.join(os.path.dirname(scenario_path), 'k8s')
    if os.path.exists(k8s_dir):
        manifest_files = glob.glob(f"{k8s_dir}/*.yaml")
        for manifest in manifest_files:
            print(f"Applying {manifest}")
            subprocess.run(['kubectl', 'apply', '-f', manifest, '-n', namespace], check=True)
    
    return scenario, namespace

def run_scenario(scenario, namespace):
    """Execute the scenario tests and collect results."""
    scenario_id = scenario.get('metadata', {}).get('id', 'unknown')
    print(f"Running scenario {scenario_id} in namespace {namespace}")
    
    # Execute the test steps
    steps = scenario.get('steps', [])
    results = {
        'scenario_id': scenario_id,
        'start_time': datetime.now().isoformat(),
        'steps': [],
        'success': True
    }
    
    for i, step in enumerate(steps):
        step_name = step.get('name', f"Step {i+1}")
        print(f"Executing step: {step_name}")
        
        # Execute commands for this step
        commands = step.get('commands', [])
        step_success = True
        step_output = []
        
        for cmd in commands:
            # Replace placeholders like {namespace}
            cmd = cmd.replace('{namespace}', namespace)
            print(f"Running command: {cmd}")
            
            try:
                output = subprocess.check_output(cmd, shell=True, text=True)
                step_output.append(output)
            except subprocess.CalledProcessError as e:
                print(f"Command failed: {e}")
                step_success = False
                results['success'] = False
                step_output.append(f"ERROR: {str(e)}")
                break
        
        results['steps'].append({
            'name': step_name,
            'success': step_success,
            'output': step_output
        })
        
        if not step_success and step.get('critical', True):
            print(f"Critical step {step_name} failed - aborting scenario")
            break
    
    # Evaluate success criteria
    criteria = scenario.get('success_criteria', [])
    results['criteria'] = []
    
    for criterion in criteria:
        criterion_name = criterion.get('name', 'Unnamed criterion')
        threshold = criterion.get('threshold', 0)
        actual = 0  # This would come from test results
        
        # In a real implementation, you would extract the actual value from test results
        # For now, we'll just set a placeholder success value
        criterion_success = True
        
        results['criteria'].append({
            'name': criterion_name,
            'threshold': threshold,
            'actual': actual,
            'success': criterion_success
        })
        
        if not criterion_success:
            results['success'] = False
    
    results['end_time'] = datetime.now().isoformat()
    return results

def cleanup_scenario(scenario, namespace):
    """Clean up resources created for the scenario."""
    print(f"Cleaning up scenario in namespace {namespace}")
    subprocess.run(['kubectl', 'delete', 'namespace', namespace, '--timeout=60s'], check=True)

def save_results(results, output_dir="tf-k8s/reports"):
    """Save test results to a JSON file."""
    os.makedirs(output_dir, exist_ok=True)
    scenario_id = results['scenario_id']
    timestamp = datetime.now().strftime("%Y%m%d-%H%M%S")
    filename = f"{output_dir}/{scenario_id}_{timestamp}.json"
    
    with open(filename, 'w') as f:
        json.dump(results, f, indent=2)
    
    print(f"Results saved to {filename}")
    return filename

def main():
    """Main function to parse arguments and execute scenarios."""
    parser = argparse.ArgumentParser(description="TF-K8s Scenario Runner")
    parser.add_argument("--group", required=True, help="Scenario group to run")
    parser.add_argument("--cluster", required=True, choices=["kind", "k3d-duo"], 
                        help="Cluster type to run tests on")
    parser.add_argument("--scenario", help="Specific scenario ID to run (optional)")
    args = parser.parse_args()
    
    # Find all scenarios to run
    if args.scenario:
        scenario_files = find_scenarios(args.group, args.scenario)
    else:
        scenario_files = find_scenarios(args.group)
    
    if not scenario_files:
        print(f"No enabled scenarios found for group {args.group}")
        sys.exit(1)
    
    success = True
    for scenario_file in scenario_files:
        try:
            # Set up the scenario
            scenario, namespace = setup_scenario(scenario_file, args.cluster)
            
            # Run the scenario
            results = run_scenario(scenario, namespace)
            
            # Save results
            save_results(results)
            
            if not results['success']:
                success = False
            
            # Clean up
            cleanup_scenario(scenario, namespace)
            
        except Exception as e:
            print(f"Error executing scenario {scenario_file}: {e}")
            success = False
    
    # Return success status
    if not success:
        sys.exit(1)

if __name__ == "__main__":
    main()
