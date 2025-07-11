package database

import (
	"testing"
)

func TestSimple(t *testing.T) {
	if 1+1 != 2 {
		t.Error("basic math failed")
	}
	t.Log("simple test passed")
}
