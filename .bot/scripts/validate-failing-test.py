#!/usr/bin/env python3
"""
Failing Test Validator
======================
Validates that a failing test scenario meets all requirements for the TDD process.
"""

import os
import sys
import yaml
import argparse
import re
from pathlib import Path
import subprocess


class TestValidator:
    """Validator for failing test scenarios."""

    def __init__(self, scenario_path, verbose=False):
        """Initialize the validator with the path to the scenario directory."""
        self.scenario_path = Path(scenario_path)
        self.verbose = verbose
        self.errors = []
        self.warnings = []
        
        # Required files
        self.required_files = [
            "scenario_definition.yaml",
            # At least one K8s resource file is required
            # At least one verification script is likely required
        ]
    
    def log(self, message):
        """Print a message if verbose mode is enabled."""
        if self.verbose:
            print(message)
    
    def check_directory_structure(self):
        """Check if the scenario directory has the expected structure."""
        self.log(f"Checking directory structure for {self.scenario_path}")
        
        # Check if the scenario directory exists
        if not self.scenario_path.exists():
            self.errors.append(f"Scenario directory does not exist: {self.scenario_path}")
            return False
        
        # Check if required files exist
        for file_name in self.required_files:
            file_path = self.scenario_path / file_name
            if not file_path.exists():
                self.errors.append(f"Required file missing: {file_path}")
                return False
        
        # Check if k8s directory exists and has at least one YAML file
        k8s_dir = self.scenario_path / "k8s"
        if not k8s_dir.exists():
            self.warnings.append(f"K8s directory missing: {k8s_dir}")
        else:
            yaml_files = list(k8s_dir.glob("*.yaml")) + list(k8s_dir.glob("*.yml"))
            if not yaml_files:
                self.warnings.append(f"No Kubernetes resource files found in {k8s_dir}")
        
        return len(self.errors) == 0
    
    def validate_scenario_definition(self):
        """Validate the scenario definition YAML file."""
        scenario_file = self.scenario_path / "scenario_definition.yaml"
        self.log(f"Validating scenario definition: {scenario_file}")
        
        try:
            with open(scenario_file, 'r') as f:
                scenario = yaml.safe_load(f)
            
            # Check for required sections
            required_sections = ["metadata", "requirements", "steps", "success_criteria"]
            for section in required_sections:
                if section not in scenario:
                    self.errors.append(f"Required section missing in scenario definition: {section}")
            
            # Check metadata
            if "metadata" in scenario:
                metadata = scenario["metadata"]
                required_metadata = ["id", "name", "description", "version", "tags", "eidc_references"]
                for field in required_metadata:
                    if field not in metadata:
                        self.errors.append(f"Required metadata field missing: {field}")
                
                # Check ID format
                if "id" in metadata:
                    id_pattern = r"^TF-[A-Z]+-[0-9]+_[A-Za-z0-9_]+$"
                    if not re.match(id_pattern, metadata["id"]):
                        self.errors.append(f"Invalid scenario ID format: {metadata['id']}")
            
            # Check requirements
            if "requirements" in scenario:
                requirements = scenario["requirements"]
                if "cluster_type" not in requirements:
                    self.errors.append("Cluster type not specified in requirements")
                elif requirements["cluster_type"] not in ["kind", "k3d-duo"]:
                    self.errors.append(f"Invalid cluster type: {requirements['cluster_type']}")
                
                if "components" not in requirements or not requirements["components"]:
                    self.errors.append("No components specified in requirements")
            
            # Check steps
            if "steps" in scenario:
                steps = scenario["steps"]
                if not steps or len(steps) < 3:
                    self.errors.append(f"Insufficient steps in scenario: {len(steps) if steps else 0}")
                
                # Check if at least one step deploys components
                deploy_steps = [s for s in steps if any("kubectl apply" in cmd for cmd in s.get("commands", []))]
                if not deploy_steps:
                    self.errors.append("No deployment step found")
                
                # Check if there's a verification step
                verify_steps = [s for s in steps if any("verify" in cmd for cmd in s.get("commands", []))]
                if not verify_steps:
                    self.errors.append("No verification step found")
            
            # Check success criteria
            if "success_criteria" in scenario:
                criteria = scenario["success_criteria"]
                if not criteria or len(criteria) < 1:
                    self.errors.append("No success criteria specified")
                
                for i, criterion in enumerate(criteria):
                    if "name" not in criterion:
                        self.errors.append(f"Name missing in success criterion {i+1}")
                    if "threshold" not in criterion:
                        self.errors.append(f"Threshold missing in success criterion {i+1}")
                    if "metric" not in criterion:
                        self.errors.append(f"Metric missing in success criterion {i+1}")
            
            return len(self.errors) == 0
            
        except yaml.YAMLError as e:
            self.errors.append(f"Error parsing scenario definition: {e}")
            return False
        except Exception as e:
            self.errors.append(f"Unexpected error validating scenario definition: {e}")
            return False
    
    def check_verification_script(self):
        """Check if there's a verification script and if it looks valid."""
        # Look for verification scripts
        verify_scripts = list(self.scenario_path.glob("verify*.py"))
        
        if not verify_scripts:
            self.warnings.append("No verification script found")
            return True
        
        # Check the first verification script
        verify_script = verify_scripts[0]
        self.log(f"Checking verification script: {verify_script}")
        
        try:
            # Check if the script is executable
            if not os.access(verify_script, os.X_OK):
                self.warnings.append(f"Verification script is not executable: {verify_script}")
            
            # Check if the script has the expected structure
            with open(verify_script, 'r') as f:
                content = f.read()
            
            # Check for important patterns
            patterns = [
                (r"if\s+__name__\s*==\s*['\"]__main__['\"]", "Main execution block"),
                (r"sys\.exit", "Exit with status code"),
                (r"parse_args|argparse", "Command-line argument parsing"),
                (r"class\s+\w+", "Class definition")
            ]
            
            for pattern, description in patterns:
                if not re.search(pattern, content):
                    self.warnings.append(f"Verification script missing: {description}")
            
            return True
            
        except Exception as e:
            self.warnings.append(f"Error checking verification script: {e}")
            return True
    
    def check_k8s_resources(self):
        """Check Kubernetes resource files for common issues."""
        k8s_dir = self.scenario_path / "k8s"
        if not k8s_dir.exists():
            return True
        
        yaml_files = list(k8s_dir.glob("*.yaml")) + list(k8s_dir.glob("*.yml"))
        if not yaml_files:
            return True
        
        self.log(f"Checking {len(yaml_files)} Kubernetes resource files")
        
        for yaml_file in yaml_files:
            try:
                with open(yaml_file, 'r') as f:
                    content = f.read()
                
                # Parse all YAML documents in the file
                docs = list(yaml.safe_load_all(content))
                
                # Check for empty documents
                if not docs:
                    self.warnings.append(f"Empty YAML file: {yaml_file}")
                    continue
                
                # Check for required fields in each document
                for i, doc in enumerate(docs):
                    if not doc:
                        self.warnings.append(f"Empty YAML document {i+1} in {yaml_file}")
                        continue
                    
                    if "kind" not in doc:
                        self.warnings.append(f"Missing 'kind' in YAML document {i+1} in {yaml_file}")
                    
                    if "metadata" not in doc:
                        self.warnings.append(f"Missing 'metadata' in YAML document {i+1} in {yaml_file}")
                    elif "name" not in doc["metadata"]:
                        self.warnings.append(f"Missing 'metadata.name' in YAML document {i+1} in {yaml_file}")
            
            except yaml.YAMLError as e:
                self.errors.append(f"Error parsing YAML file {yaml_file}: {e}")
            except Exception as e:
                self.warnings.append(f"Unexpected error checking YAML file {yaml_file}: {e}")
        
        return len(self.errors) == 0
    
    def check_syntax(self):
        """Run syntax checks on Python and YAML files."""
        self.log("Running syntax checks")
        
        # Check Python files
        py_files = list(self.scenario_path.glob("*.py"))
        for py_file in py_files:
            try:
                result = subprocess.run(
                    ["python", "-m", "py_compile", str(py_file)],
                    capture_output=True,
                    text=True
                )
                
                if result.returncode != 0:
                    self.errors.append(f"Python syntax error in {py_file}: {result.stderr}")
            
            except Exception as e:
                self.warnings.append(f"Error checking Python syntax for {py_file}: {e}")
        
        # Check YAML files with yamllint if available
        try:
            result = subprocess.run(
                ["which", "yamllint"],
                capture_output=True,
                text=True
            )
            
            if result.returncode == 0:
                yaml_files = list(self.scenario_path.glob("**/*.yaml")) + list(self.scenario_path.glob("**/*.yml"))
                for yaml_file in yaml_files:
                    try:
                        result = subprocess.run(
                            ["yamllint", "-d", "relaxed", str(yaml_file)],
                            capture_output=True,
                            text=True
                        )
                        
                        if result.returncode != 0:
                            self.warnings.append(f"YAML lint issues in {yaml_file}: {result.stdout}")
                    
                    except Exception as e:
                        self.warnings.append(f"Error checking YAML syntax for {yaml_file}: {e}")
        
        except:
            self.log("yamllint not available, skipping YAML syntax check")
        
        return len(self.errors) == 0
    
    def validate(self):
        """Run all validation checks and return the result."""
        self.log(f"Validating failing test: {self.scenario_path}")
        
        checks = [
            self.check_directory_structure,
            self.validate_scenario_definition,
            self.check_verification_script,
            self.check_k8s_resources,
            self.check_syntax
        ]
        
        all_passed = True
        for check in checks:
            if not check():
                all_passed = False
        
        return all_passed
    
    def print_results(self):
        """Print validation results."""
        if not self.errors and not self.warnings:
            print(f"✅ Test scenario {self.scenario_path} is valid")
            return True
        
        if self.errors:
            print(f"❌ Found {len(self.errors)} errors:")
            for i, error in enumerate(self.errors):
                print(f"  {i+1}. {error}")
        
        if self.warnings:
            print(f"⚠️ Found {len(self.warnings)} warnings:")
            for i, warning in enumerate(self.warnings):
                print(f"  {i+1}. {warning}")
        
        return len(self.errors) == 0


def parse_args():
    """Parse command-line arguments."""
    parser = argparse.ArgumentParser(description="Validate failing test scenarios")
    parser.add_argument("scenario_path", help="Path to the scenario directory")
    parser.add_argument("-v", "--verbose", action="store_true", help="Enable verbose output")
    return parser.parse_args()


if __name__ == "__main__":
    args = parse_args()
    validator = TestValidator(args.scenario_path, args.verbose)
    valid = validator.validate()
    validator.print_results()
    
    sys.exit(0 if valid else 1)
