// Package circuittest holds the checks a gnark module's tests run on the
// circuits it deploys, so that no value in them is left for the prover to
// choose. A hint's outputs are whatever the prover's solver returns: gnark's
// own gadgets constrain the outputs of the hints they take, while a hint
// taken anywhere else (a Poseidon hash computed in a hint, say) leaves its
// output unconstrained unless the caller adds the constraints itself. The
// deployed circuits take hints only through gnark's gadgets.
package circuittest

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	cs "github.com/consensys/gnark/constraint/bn254"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/std/math/bits"
	"github.com/consensys/gnark/std/math/bitslice"
)

// Named is a deployed circuit, by the name its keys are saved under.
type Named struct {
	Name    string
	Circuit frontend.Circuit
}

// gnarkHints prefixes the name of every hint gnark defines: gnark records a
// hint by its function's full name, package path first.
const gnarkHints = "github.com/consensys/gnark/"

// CheckHints compiles each circuit and fails t on a hint its constraint
// system depends on that gnark does not define. It also fails when the
// circuits depend on no hint at all: each module's circuits decompose
// values into bits, which takes gnark's own hint, so reading none means
// the dependencies are not being read.
func CheckHints(t testing.TB, circuits []Named) {
	t.Helper()
	read := 0
	for _, c := range circuits {
		hints, err := hintDependencies(c.Circuit)
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		read += len(hints)
		for _, h := range hints {
			if !strings.HasPrefix(h, gnarkHints) {
				t.Errorf("%s depends on hint %s, which gnark does not define", c.Name, h)
			}
		}
	}
	if read == 0 {
		t.Fatal("no circuit records a hint dependency, not even gnark's own: the dependency lists are not being read")
	}
}

// hintDependencies compiles c to a BN254 R1CS and returns the names of the
// hints it depends on, as gnark records them, sorted.
func hintDependencies(c frontend.Circuit) ([]string, error) {
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, c)
	if err != nil {
		return nil, err
	}
	sys, ok := ccs.(*cs.R1CS)
	if !ok {
		return nil, fmt.Errorf("compiled to a %T, want a BN254 R1CS", ccs)
	}
	return slices.Sorted(maps.Values(sys.MHintsDependencies)), nil
}

// CheckSources parses every non-test .go file of each package of this repo
// that the package in the working directory builds (under go test, the
// test's own package) and fails t on each use of a gnark API that can leave
// a value unconstrained (unconstraining). A new gadget that needs one
// changes this check in the review that checks the gadget's constraints.
// It also fails when the packages it parsed leave out a circuit's own
// package or one in alsoScan, so a scan that reads too little fails rather
// than passes.
func CheckSources(t testing.TB, circuits []Named, alsoScan ...string) {
	t.Helper()
	if len(circuits) == 0 {
		t.Fatal("no circuits to check")
	}
	want := slices.Clone(alsoScan)
	for _, c := range circuits {
		typ := reflect.TypeOf(c.Circuit)
		if typ.Kind() == reflect.Pointer {
			typ = typ.Elem()
		}
		want = append(want, typ.PkgPath())
	}
	checkSources(t, ".", want)
}

// checkSources fails t on each use of a name in unconstraining in the
// sources of the packages of this repo that the package in dir builds, and
// on a package in want that is not among them.
func checkSources(t testing.TB, dir string, want []string) {
	t.Helper()
	pkgs, err := sources(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range want {
		if _, ok := pkgs[p]; !ok {
			t.Errorf("package %s is not among those scanned: %v", p, slices.Sorted(maps.Keys(pkgs)))
		}
	}
	for _, p := range slices.Sorted(maps.Keys(pkgs)) {
		uses, err := unconstrainingUses(pkgs[p])
		if err != nil {
			t.Fatal(err)
		}
		for _, u := range uses {
			t.Errorf("%s: %s can leave a value unconstrained; the circuits here use gnark's constrained gadgets only", u.pos, u.name)
		}
	}
}

// listInRepo is go list's template for a package of this repo, which
// scripts/keys_src.sh selects the same way: a package of the main module, or
// of a module it replaces with a local directory (enygma_gnark_shared). It
// prints the import path and directory of such a package, a tab between
// them, and an empty line for any other.
const listInRepo = `{{with .Module}}{{if or .Main (and .Replace (not .Replace.Version))}}{{$.ImportPath}}{{"\t"}}{{$.Dir}}{{end}}{{end}}`

// sources runs go list -deps in dir and returns the non-test .go files of
// each package of this repo it lists, by import path (listedSources).
func sources(dir string) (map[string][]string, error) {
	cmd := exec.Command("go", "list", "-deps", "-f", listInRepo, ".")
	cmd.Dir = dir
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list -deps in %s: %w: %s", dir, err, stderr.String())
	}
	return listedSources(string(out))
}

// listedSources reads what go list printed with listInRepo and returns the
// non-test .go files in each package's directory, by import path. It fails
// on a line of another shape, and on a package with no such file.
func listedSources(out string) (map[string][]string, error) {
	pkgs := map[string][]string{}
	for line := range strings.Lines(out) {
		line = strings.TrimSuffix(line, "\n")
		if line == "" {
			continue
		}
		path, dir, ok := strings.Cut(line, "\t")
		if !ok || path == "" || dir == "" {
			return nil, fmt.Errorf("go list printed %q, want an import path, a tab and a directory", line)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		var files []string
		for _, e := range entries {
			if n := e.Name(); !e.IsDir() && strings.HasSuffix(n, ".go") && !strings.HasSuffix(n, "_test.go") {
				files = append(files, filepath.Join(dir, n))
			}
		}
		if len(files) == 0 {
			return nil, fmt.Errorf("package %s: no non-test .go file in %s", path, dir)
		}
		pkgs[path] = files
	}
	return pkgs, nil
}

// builderMethods are the methods of gnark's R1CS builder (the api a
// circuit's Define gets, and api.Compiler()) that make a variable the solver
// fills with no constraint on it (NewHint, NewHintForId), mark a variable
// boolean without constraining it (MarkBoolean), or add solver instructions
// of the caller's own (AddBlueprint, AddInstruction).
var builderMethods = []string{"NewHint", "NewHintForId", "MarkBoolean", "AddBlueprint", "AddInstruction"}

// gadgetOptions are gnark's std gadget options that leave out a constraint
// the gadget otherwise adds: that each digit is a digit, or that a bit
// decomposition is below the field modulus.
var gadgetOptions = []any{bits.WithUnconstrainedOutputs, bits.WithUnconstrainedInputs, bits.OmitModulusCheck, bitslice.WithUnconstrainedOutputs}

// unconstraining returns the names checkSources looks for: builderMethods,
// and the function name of each of gadgetOptions. Those are gnark's own
// functions, so a gnark upgrade that renames one fails to build here
// instead of leaving the scan looking for a name nothing uses.
func unconstraining() map[string]bool {
	names := map[string]bool{}
	for _, m := range builderMethods {
		names[m] = true
	}
	for _, o := range gadgetOptions {
		full := runtime.FuncForPC(reflect.ValueOf(o).Pointer()).Name()
		names[full[strings.LastIndex(full, ".")+1:]] = true
	}
	return names
}

// use is an identifier named in unconstraining, where it appears.
type use struct {
	pos  token.Position
	name string
}

// unconstrainingUses parses each file and returns every identifier in it
// named in unconstraining: a call, a method value, a method of an interface
// type and a declaration all count.
func unconstrainingUses(files []string) ([]use, error) {
	names := unconstraining()
	fset := token.NewFileSet()
	var uses []use
	for _, file := range files {
		f, err := parser.ParseFile(fset, file, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && names[id.Name] {
				uses = append(uses, use{fset.Position(id.Pos()), id.Name})
			}
			return true
		})
	}
	return uses, nil
}
