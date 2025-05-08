#!/usr/bin/env python3
"""
Documentation Drift Detection Script
===================================
Regenerates documentation from schemas and checks for drift.
"""

import os
import sys
import yaml
import json
import subprocess
import glob
import difflib

def generate_docs():
    """Generate markdown docs from schema files."""
    schema_files = glob.glob("eidc/**/schemas/*.yaml", recursive=True)
    
    for schema_file in schema_files:
        print(f"Processing schema: {schema_file}")
        
        # Determine the output path
        schema_name = os.path.basename(schema_file).replace('.yaml', '')
        schema_dir = os.path.dirname(schema_file)
        output_dir = os.path.join("docs", "generated", os.path.relpath(schema_dir, "eidc"))
        os.makedirs(output_dir, exist_ok=True)
        
        output_file = os.path.join(output_dir, f"{schema_name}.md")
        
        # Generate markdown documentation
        try:
            with open(schema_file, 'r') as f:
                schema = yaml.safe_load(f)
            
            # Create markdown representation
            markdown = [f"# {schema.get('title', schema_name)}", ""]
            
            if 'description' in schema:
                markdown.extend([schema['description'], ""])
            
            # Add table of contents
            markdown.extend(["## Table of Contents", ""])
            
            # Add properties section
            if 'properties' in schema:
                markdown.extend(["## Properties", ""])
                
                for prop_name, prop_def in schema['properties'].items():
                    markdown.extend([f"### {prop_name}", ""])
                    
                    if 'description' in prop_def:
                        markdown.extend([prop_def['description'], ""])
                    
                    markdown.extend([f"**Type**: {prop_def.get('type', 'any')}", ""])
                    
                    if 'required' in schema and prop_name in schema['required']:
                        markdown.extend(["**Required**: Yes", ""])
                    else:
                        markdown.extend(["**Required**: No", ""])
                    
                    if 'enum' in prop_def:
                        markdown.extend([
                            "**Allowed Values**:", 
                            "```", 
                            yaml.dump(prop_def['enum']), 
                            "```", 
                            ""
                        ])
            
            # Write the markdown file
            with open(output_file, 'w') as f:
                f.write("\n".join(markdown))
            
            print(f"Generated documentation: {output_file}")
            
        except Exception as e:
            print(f"Error generating documentation for {schema_file}: {e}")
            return False
    
    return True

def check_docs_drift():
    """Check if there's drift between generated and committed docs."""
    # First, generate docs
    if not generate_docs():
        print("Documentation generation failed")
        return False
    
    # Check for git changes
    try:
        result = subprocess.run(['git', 'diff', '--name-only'], 
                               capture_output=True, text=True, check=True)
        
        changed_files = result.stdout.strip().split('\n')
        changed_md_files = [f for f in changed_files if f.endswith('.md') and 'eidc/' in f]
        
        if changed_md_files:
            print("Documentation drift detected in the following files:")
            for file in changed_md_files:
                print(f"  - {file}")
            
            # Show detailed diff for each file
            for file in changed_md_files:
                print(f"\nDiff for {file}:")
                subprocess.run(['git', 'diff', file], check=True)
            
            return False
        else:
            print("No documentation drift detected")
            return True
        
    except subprocess.CalledProcessError as e:
        print(f"Error checking for drift: {e}")
        return False

if __name__ == "__main__":
    success = check_docs_drift()
    sys.exit(0 if success else 1)
