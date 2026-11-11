// Package circuitconfig checks that a circuit's compile-time config holds
// only plain Go values.
//
// gnark turns every frontend.Variable it finds in a circuit struct into a
// witness, including one inside a config struct, unless the field is tagged
// `gnark:"-"`. A bound kept there as a frontend.Variable is then chosen by the
// prover, not fixed by the verifying key. Each module's tests run Check on every
// circuit it compiles, so such a field cannot come back unnoticed.
package circuitconfig

import (
	"fmt"
	"reflect"
)

// Check returns an error naming the first field of circuit's Config field
// (circuit is a circuit struct or a pointer to one) that is not a plain Go value:
// an interface (which frontend.Variable is), pointer, map, func or channel,
// at any depth. A circuit with no Config field passes.
func Check(circuit any) error {
	t := reflect.TypeOf(circuit)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return fmt.Errorf("%s is not a circuit struct", t)
	}
	f, ok := t.FieldByName("Config")
	if !ok {
		return nil
	}
	return plain(t.Name()+".Config", f.Type)
}

// plain reports the first part of type t, reached by path, that is not a
// plain Go value.
func plain(path string, t reflect.Type) error {
	switch t.Kind() {
	case reflect.Bool, reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return nil
	case reflect.Array, reflect.Slice:
		return plain(path+"[]", t.Elem())
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if err := plain(path+"."+f.Name, f.Type); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("%s is a %s, not a compile-time Go value: gnark would compile a frontend.Variable there as a prover-chosen witness", path, t)
	}
}
