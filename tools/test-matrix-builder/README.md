# Test Matrix Builder

This tool reads the test matrix from `docs/appendices/appendix-a-test-matrix.md` and the E2E test program from `docs/appendices/appendix-i-e2e-program.md` and injects them into the main PRD document.

## Usage

```bash
# Update the PRD with the latest test matrix and E2E program
python test_matrix_builder.py

# Verify that all P0/P1 requirements have passing tests (for CI)
python test_matrix_builder.py --verify
```

## Integration with CI

The test matrix builder is used in CI to:

1. Verify that all P0/P1 requirements have passing tests
2. Update the PRD with the latest test matrix and E2E program

## Local Development

During local development, you should:

1. Update the test matrix in `docs/appendices/appendix-a-test-matrix.md` when implementing or modifying a requirement
2. Run `make docs-sync` or `python tools/test-matrix-builder/test_matrix_builder.py` before pushing to update the PRD