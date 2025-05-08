#!/usr/bin/env python3
"""
Edge-Probe Command Line Hashing Verification Script
==================================================
Verifies that process command lines are properly hashed in Edge-Probe metrics.
"""

import sys
import re
import hashlib

def check_hashed_command_lines(metrics_file):
    """Check if all command lines are properly hashed in the metrics."""
    with open(metrics_file, 'r') as f:
        metrics_content = f.read()
    
    # Count metrics with command line hash (check both naming conventions)
    hash_pattern = r'process[._]command_line_hash="([a-f0-9]{64})"'
    hashed_commands = re.findall(hash_pattern, metrics_content)
    
    # Look for plaintext sensitive patterns that should be hashed
    sensitive_patterns = [
        r'username=admin',
        r'password=S3cr3tP@ssw0rd',
        r'api_key=12345abcdef'
    ]
    
    leaked_sensitive_info = []
    for pattern in sensitive_patterns:
        if re.search(pattern, metrics_content):
            leaked_sensitive_info.append(pattern)
    
    # Verify hash consistency
    expected_hash = hashlib.sha256(
        "Running test process with username=admin password=S3cr3tP@ssw0rd api_key=12345abcdef".encode()
    ).hexdigest()
    
    hash_matches = [h == expected_hash for h in hashed_commands]
    
    # Prepare results
    results = {
        "hashing_success_percentage": 100 if hashed_commands else 0,
        "original_command_found_count": len(leaked_sensitive_info),
        "hash_consistency_percentage": (sum(hash_matches) / len(hashed_commands) * 100) if hashed_commands else 0,
    }
    
    # Print results
    print("\n=== Command Line Hashing Verification Results ===")
    print(f"Hashed command lines found: {len(hashed_commands)}")
    print(f"Hashing success percentage: {results['hashing_success_percentage']}%")
    print(f"Original sensitive info instances found: {results['original_command_found_count']}")
    
    if leaked_sensitive_info:
        print("WARNING: The following sensitive patterns were found in plaintext:")
        for pattern in leaked_sensitive_info:
            print(f"  - {pattern}")
    
    print(f"Hash consistency percentage: {results['hash_consistency_percentage']}%")
    
    # Overall success determination
    if (results['hashing_success_percentage'] == 100 and
        results['original_command_found_count'] == 0 and
        results['hash_consistency_percentage'] == 100):
        print("\nOVERALL RESULT: PASS - Command line hashing is working correctly")
        return 0
    else:
        print("\nOVERALL RESULT: FAIL - Command line hashing is not working correctly")
        return 1

if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("Usage: verify_hashing.py <metrics_file>")
        sys.exit(1)
    
    metrics_file = sys.argv[1]
    exit_code = check_hashed_command_lines(metrics_file)
    sys.exit(exit_code)
