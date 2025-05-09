package test

import (
	"testing"
)

// A simple test to verify the testing environment
func TestSimple(t *testing.T) {
	// This test should always pass
	expected := 2
	actual := 1 + 1
	if actual != expected {
		t.Errorf("Expected %d, but got %d", expected, actual)
	}
}
