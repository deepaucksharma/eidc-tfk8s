#!/usr/bin/env python3
"""
EIDC Schema Validation Script
=============================
Validates that all schema files are valid according to their schema definition.
"""

import os
import sys
import yaml
import json
import jsonschema
import glob

def validate_yaml_schema(schema_file):
    """Validate that a YAML schema file is valid."""
    try:
        with open(schema_file, 'r') as f:
            schema = yaml.safe_load(f)
        
        # If this is a JSON Schema, validate it against the JSON Schema meta-schema
        if schema.get('$schema') and 'json-schema.org' in schema.get('$schema'):
            # Convert the schema to JSON for validation
            schema_json = json.loads(json.dumps(schema))
            
            # The meta-schema to validate against
            meta_schema_url = schema.get('$schema')
            print(f"Validating {schema_file} against {meta_schema_url}")
            
            # For simplicity, we'll use the Draft7Validator
            jsonschema.Draft7Validator.check_schema(schema_json)
            
            print(f"✓ Schema {schema_file} is valid")
            return True
        else:
            print(f"? Schema {schema_file} doesn't reference a JSON Schema - skipping validation")
            return True
            
    except yaml.YAMLError as e:
        print(f"✗ Error parsing YAML in {schema_file}: {e}")
        return False
    except jsonschema.exceptions.SchemaError as e:
        print(f"✗ Schema {schema_file} is invalid: {e}")
        return False
    except Exception as e:
        print(f"✗ Unexpected error validating {schema_file}: {e}")
        return False

def validate_all_schemas():
    """Find and validate all schema files in the repository."""
    schema_files = glob.glob("eidc/**/schemas/*.yaml", recursive=True)
    
    if not schema_files:
        print("No schema files found")
        return True
    
    all_valid = True
    for schema_file in schema_files:
        if not validate_yaml_schema(schema_file):
            all_valid = False
    
    return all_valid

if __name__ == "__main__":
    success = validate_all_schemas()
    sys.exit(0 if success else 1)
