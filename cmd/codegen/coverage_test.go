package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCoverageMatrixDrift(t *testing.T) {
	// The test runs in cmd/codegen so we need to navigate up to the root to read the files
	err := os.Chdir("../..")
	if err != nil {
		t.Fatalf("could not chdir to project root: %v", err)
	}

	generated := generateCoverageMatrix()

	committed, err := os.ReadFile(filepath.Join("docs", "reference", "construct_coverage.md"))
	if err != nil {
		t.Fatalf("could not read committed coverage matrix: %v", err)
	}

	if generated != string(committed) {
		t.Errorf("Committed docs/reference/construct_coverage.md differs from generator output. Run 'go run ./cmd/codegen' to update it.")
	}
}
