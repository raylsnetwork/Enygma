// Package statement describes a circuit's statement layout: the order of its
// public signals, as named fields with offsets and sizes. A layout is derived
// from the circuit definition itself (gnark's own schema walk, the same walk
// that orders the public witness), so the circuit stays the single source of
// truth; circuit packages only annotate each public Go field with its domain
// name.
//
// Each project's generator (enygma_payments' gnark-server/cmd/genlayout,
// enygma_retail_payments' gnark_circuits/cmd/genlayout) renders its layouts
// into a dependency-free Go package and a Solidity constants library through
// a Target (see render.go).
package statement

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/schema"
)

// Field is one named public field of a statement.
type Field struct {
	Name   string // domain name, e.g. "Nullifier"; one spelling in Go and Solidity
	Offset int    // index of the field's first signal
	Size   int    // number of signals
	// Shape is the field's dimensions in the circuit struct, outermost first:
	// nil for a scalar, [6] for a slice, [6 6] for the k×k fingerprint matrix,
	// [6 2] for a slice of (X, Y) points. Elements are flattened row-major,
	// so element [i][j] of a [n m] field is signal Offset + i*m + j (for
	// points: X at Offset + 2i, Y at Offset + 2i + 1).
	Shape []int
}

// Layout is a circuit's statement layout.
type Layout struct {
	Circuit string // Go and Solidity identifier prefix, e.g. "Transfer"
	Fields  []Field
	Len     int // total number of public signals
}

// Field returns the field with the given domain name.
func (l Layout) Field(name string) (Field, bool) {
	for _, f := range l.Fields {
		if f.Name == name {
			return f, true
		}
	}
	return Field{}, false
}

// Derive walks circuit's public leaves in public-witness order and groups
// them by top-level struct field. names maps each public Go field name to its
// domain name; a public field without a name, or a name for a field that is
// not public, is an error, so the annotation cannot drift from the circuit.
// Derive also checks that every field's leaves are visited in row-major
// order, which is what Field.Shape promises.
func Derive(circuitName string, circuit frontend.Circuit, names map[string]string) (Layout, error) {
	l := Layout{Circuit: circuitName}
	var indices [][]int // per leaf of the current field: its index path
	var goName string
	used := map[string]bool{}

	closeField := func() error {
		if goName == "" {
			return nil
		}
		f := &l.Fields[len(l.Fields)-1]
		shape, err := rowMajorShape(goName, indices)
		if err != nil {
			return err
		}
		f.Shape = shape
		return nil
	}

	tVariable := reflect.TypeOf((*frontend.Variable)(nil)).Elem()
	_, err := schema.Walk(ecc.BN254.ScalarField(), circuit, tVariable, func(leaf schema.LeafInfo, _ reflect.Value) error {
		if leaf.Visibility != schema.Public {
			return nil
		}
		parts := strings.Split(leaf.FullName(), "_")
		idx := make([]int, len(parts)-1)
		for i, p := range parts[1:] {
			n, err := strconv.Atoi(p)
			if err != nil {
				return fmt.Errorf("public leaf %s: unsupported nesting (only arrays of variables)", leaf.FullName())
			}
			idx[i] = n
		}
		if parts[0] != goName {
			if err := closeField(); err != nil {
				return err
			}
			goName = parts[0]
			if used[goName] {
				return fmt.Errorf("public field %s is not contiguous", goName)
			}
			name, ok := names[goName]
			if !ok {
				return fmt.Errorf("public field %s has no domain name", goName)
			}
			used[goName] = true
			l.Fields = append(l.Fields, Field{Name: name, Offset: l.Len})
			indices = indices[:0]
		}
		l.Fields[len(l.Fields)-1].Size++
		indices = append(indices, idx)
		l.Len++
		return nil
	})
	if err != nil {
		return Layout{}, err
	}
	if err := closeField(); err != nil {
		return Layout{}, err
	}
	for goName := range names {
		if !used[goName] {
			return Layout{}, fmt.Errorf("domain name given for %s, which is not a public field", goName)
		}
	}
	return l, nil
}

// MustDerive is Derive for package-level layouts; it panics on error.
func MustDerive(circuitName string, circuit frontend.Circuit, names map[string]string) Layout {
	l, err := Derive(circuitName, circuit, names)
	if err != nil {
		panic(err)
	}
	return l
}

// rowMajorShape infers a field's dimensions from its leaves' index paths and
// checks the leaves were visited in row-major order.
func rowMajorShape(goName string, indices [][]int) ([]int, error) {
	if len(indices[0]) == 0 {
		return nil, nil
	}
	shape := make([]int, len(indices[0]))
	for _, idx := range indices {
		for d, n := range idx {
			shape[d] = max(shape[d], n+1)
		}
	}
	for pos, idx := range indices {
		flat := 0
		for d, n := range idx {
			flat = flat*shape[d] + n
		}
		if flat != pos {
			return nil, fmt.Errorf("public field %s is not flattened row-major (element %v at position %d)", goName, idx, pos)
		}
	}
	if size := product(shape); size != len(indices) {
		return nil, fmt.Errorf("public field %s is ragged: shape %v but %d elements", goName, shape, len(indices))
	}
	return shape, nil
}

func product(shape []int) int {
	p := 1
	for _, n := range shape {
		p *= n
	}
	return p
}
