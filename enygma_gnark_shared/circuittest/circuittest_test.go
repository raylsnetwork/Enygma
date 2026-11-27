package circuittest

import (
	"fmt"
	"maps"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/std/math/bits"
)

// recorder is a testing.TB that records the failures a check reports
// instead of failing the test that runs it.
type recorder struct {
	testing.TB
	failures []string
}

func (r *recorder) Helper() {}

func (r *recorder) Errorf(format string, args ...any) {
	r.failures = append(r.failures, fmt.Sprintf(format, args...))
}

func (r *recorder) Fatal(args ...any) {
	r.failures = append(r.failures, fmt.Sprint(args...))
	runtime.Goexit()
}

func (r *recorder) Fatalf(format string, args ...any) {
	r.Errorf(format, args...)
	runtime.Goexit()
}

// record runs check on a recorder, in a goroutine of its own so that a
// Fatal stops only the check, and returns the failures it reported.
func record(check func(testing.TB)) []string {
	r := &recorder{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		check(r)
	}()
	<-done
	return r.failures
}

// matchFailures fails t unless got has one failure per entry of want, in
// order, each containing that entry.
func matchFailures(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("failures %q, want %d containing %q", got, len(want), want)
	}
	for i := range want {
		if !strings.Contains(got[i], want[i]) {
			t.Errorf("failure %d is %q, want it to contain %q", i, got[i], want[i])
		}
	}
}

// squared takes X² from a hint of this package's own and constrains it.
type squared struct{ X, Y frontend.Variable }

func (c *squared) Define(api frontend.API) error {
	sq, err := api.Compiler().NewHint(square, 1, c.X)
	if err != nil {
		return err
	}
	api.AssertIsEqual(sq[0], api.Mul(c.X, c.X))
	api.AssertIsEqual(sq[0], c.Y)
	return nil
}

// square is a hint: res[0] = inputs[0]² mod the field.
func square(mod *big.Int, inputs, res []*big.Int) error {
	res[0].Mul(inputs[0], inputs[0])
	res[0].Mod(res[0], mod)
	return nil
}

// decomposed splits X into bits with gnark's gadget, which takes gnark's own
// hint.
type decomposed struct{ X frontend.Variable }

func (c *decomposed) Define(api frontend.API) error {
	bits.ToBinary(api, c.X, bits.WithNbDigits(8))
	return nil
}

// plain takes no hint at all.
type plain struct{ X, Y frontend.Variable }

func (c *plain) Define(api frontend.API) error {
	api.AssertIsEqual(c.X, c.Y)
	return nil
}

// TestCheckHints passes circuits that take only gnark's hints, and fails on
// a hint of this package's own, even one its circuit constrains, and on
// circuits that record no hint dependency at all.
func TestCheckHints(t *testing.T) {
	for _, c := range []struct {
		name     string
		circuits []Named
		want     []string
	}{
		{"gnark's hint", []Named{{"Decomposed", &decomposed{}}}, nil},
		{"a hint of its own", []Named{{"Decomposed", &decomposed{}}, {"Squared", &squared{}}},
			[]string{"Squared depends on hint enygma_gnark_shared/circuittest.square, which gnark does not define"}},
		{"no hint read", []Named{{"Plain", &plain{}}}, []string{"no circuit records a hint dependency"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			matchFailures(t, record(func(r testing.TB) { CheckHints(r, c.circuits) }), c.want)
		})
	}
}

// TestBuilderMethodsAreGnarks: each of builderMethods is a method of the
// builder gnark compiles an R1CS with, so the scan looks for no name gnark
// has dropped or renamed.
func TestBuilderMethodsAreGnarks(t *testing.T) {
	b, err := r1cs.NewBuilder[constraint.U64](ecc.BN254.ScalarField(), frontend.CompileConfig{})
	if err != nil {
		t.Fatal(err)
	}
	typ := reflect.TypeOf(b)
	for _, m := range builderMethods {
		if _, ok := typ.MethodByName(m); !ok {
			t.Errorf("gnark's R1CS builder (%v) has no method %s", typ, m)
		}
	}
}

// TestUnconstrainingUsesFindsEveryMarkedUse: in
// testdata/unconstraining.go, the scan finds the use on each line marked
// "// want <name>" and nothing else; and every name it looks for is marked
// there at least once.
func TestUnconstrainingUsesFindsEveryMarkedUse(t *testing.T) {
	const file = "testdata/unconstraining.go"
	src, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	want, marked := map[string]bool{}, map[string]bool{}
	for i, line := range strings.Split(string(src), "\n") {
		if _, name, ok := strings.Cut(line, "// want "); ok {
			name = strings.TrimSpace(name)
			want[fmt.Sprintf("line %d: %s", i+1, name)] = true
			marked[name] = true
		}
	}
	if len(want) == 0 {
		t.Fatalf("%s marks no use", file)
	}
	uses, err := unconstrainingUses([]string{file})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, u := range uses {
		got[fmt.Sprintf("line %d: %s", u.pos.Line, u.name)] = true
	}
	for _, w := range slices.Sorted(maps.Keys(want)) {
		if !got[w] {
			t.Errorf("%s, %s: not found", file, w)
		}
	}
	for _, g := range slices.Sorted(maps.Keys(got)) {
		if !want[g] {
			t.Errorf("%s, %s: found but not marked", file, g)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(unconstraining())) {
		if !marked[name] {
			t.Errorf("%s marks no use of %s", file, name)
		}
	}
}

// gadgetsSrc declares and calls NewHint, on lines 5 and 10.
const gadgetsSrc = `package gadgets

// Builder is the part of a constraint builder Square uses.
type Builder interface {
	NewHint(f any, n int, in ...any) ([]any, error)
}

// Square returns x*x from a hint, with no constraint on it.
func Square(b Builder, x any) any {
	out, _ := b.NewHint(nil, 1, x)
	return out[0]
}
`

// TestCheckSources lays out two modules as this repo's gnark modules are:
// circuits, which imports fmt and replaces gadgets with a local directory.
// gadgets uses NewHint in its source and in its test file. The scan lists
// both packages and not fmt, reads no test file, flags each use in gadgets'
// source, and fails on a package it was told to scan and did not.
func TestCheckSources(t *testing.T) {
	root := t.TempDir()
	for path, src := range map[string]string{
		"circuits/go.mod":         "module example.com/circuits\n\ngo 1.26.0\n\nrequire example.com/gadgets v0.0.0\n\nreplace example.com/gadgets => ../gadgets\n",
		"circuits/circuits.go":    "package circuits\n\nimport (\n\t\"fmt\"\n\n\t\"example.com/gadgets\"\n)\n\nvar _, _ = fmt.Sprint, gadgets.Square\n",
		"gadgets/go.mod":          "module example.com/gadgets\n\ngo 1.26.0\n",
		"gadgets/gadgets.go":      gadgetsSrc,
		"gadgets/gadgets_test.go": "package gadgets\n\nvar _ = Builder.NewHint\n",
	} {
		path = filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dir := filepath.Join(root, "circuits")

	pkgs, err := sources(dir)
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string][]string{}
	for p, files := range pkgs {
		for _, f := range files {
			listed[p] = append(listed[p], filepath.Base(f))
		}
	}
	wantListed := map[string][]string{"example.com/circuits": {"circuits.go"}, "example.com/gadgets": {"gadgets.go"}}
	if !reflect.DeepEqual(listed, wantListed) {
		t.Fatalf("sources listed %v, want %v", listed, wantListed)
	}

	t.Run("each use", func(t *testing.T) {
		got := record(func(r testing.TB) { checkSources(r, dir, []string{"example.com/circuits", "example.com/gadgets"}) })
		matchFailures(t, got, []string{"gadgets.go:5:2: NewHint can leave a value unconstrained", "gadgets.go:10:14: NewHint can leave a value unconstrained"})
	})
	t.Run("a package not scanned", func(t *testing.T) {
		got := record(func(r testing.TB) { checkSources(r, dir, []string{"example.com/absent"}) })
		matchFailures(t, got, []string{"package example.com/absent is not among those scanned", "gadgets.go:5:2: NewHint", "gadgets.go:10:14: NewHint"})
	})
}

// TestCheckSourcesNeedsCircuits: with no circuit, CheckSources fails before
// it scans, rather than run a scan nothing told it to cover.
func TestCheckSourcesNeedsCircuits(t *testing.T) {
	matchFailures(t, record(func(r testing.TB) { CheckSources(r, nil) }), []string{"no circuits to check"})
}

// TestListedSourcesRefusesWhatItCannotRead: a line of go list's output with
// no tab, and a package directory with no non-test .go file, fail rather
// than read as nothing to scan.
func TestListedSourcesRefusesWhatItCannotRead(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x_test.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ name, out, want string }{
		{"no tab", "example.com/x " + dir + "\n", "want an import path, a tab and a directory"},
		{"only a test file", "example.com/x\t" + dir + "\n", "package example.com/x: no non-test .go file"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if _, err := listedSources(c.out); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("listedSources(%q): err %v, want one containing %q", c.out, err, c.want)
			}
		})
	}
}
