package circuitconfig

import (
	"testing"

	"github.com/consensys/gnark/frontend"
)

type intConfig struct {
	Depth int
	Sizes []int
}

type variableConfig struct {
	Depth int
	Range frontend.Variable
}

type goodCircuit struct {
	Config intConfig
	X      frontend.Variable
}

type badCircuit struct {
	Config variableConfig
	X      frontend.Variable
}

type noConfigCircuit struct {
	X frontend.Variable
}

// TestCheck accepts an int-only config and a circuit without one, and
// rejects a config holding a frontend.Variable.
func TestCheck(t *testing.T) {
	if err := Check(&goodCircuit{}); err != nil {
		t.Fatalf("int-only config rejected: %v", err)
	}
	if err := Check(noConfigCircuit{}); err != nil {
		t.Fatalf("circuit without a config rejected: %v", err)
	}
	if err := Check(&badCircuit{}); err == nil {
		t.Fatal("config with a frontend.Variable accepted")
	}
}
