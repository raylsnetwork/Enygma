// Package fuzzedit applies an edit script to bytes, for fuzz targets whose
// property needs the input they change: a genuine contribution made in the
// fuzzing process, which only that process knows (go test -fuzz runs each
// target in several processes, each making its own). The fuzzer mutates the
// script; the script means the same edits in every process.
package fuzzedit

// Apply returns data edited by script, read four bytes at a time as
// (op, offset high, offset low, value), the offset taken modulo the data's
// length as it then stands:
//
//	op%4 == 0: set the byte at offset to value
//	op%4 == 1: flip the bits of value|1 in the byte at offset (always a change)
//	op%4 == 2: delete value%32+1 bytes from offset
//	op%4 == 3: insert value before offset
//
// A trailing part of fewer than four bytes is ignored. data is not changed.
func Apply(data, script []byte) []byte {
	out := append([]byte(nil), data...)
	for ; len(script) >= 4; script = script[4:] {
		op, value := script[0]%4, script[3]
		if len(out) == 0 {
			if op == 3 {
				out = append(out, value)
			}
			continue
		}
		at := (int(script[1])<<8 | int(script[2])) % len(out)
		switch op {
		case 0:
			out[at] = value
		case 1:
			out[at] ^= value | 1
		case 2:
			out = append(out[:at], out[min(at+int(value%32)+1, len(out)):]...)
		case 3:
			out = append(out[:at], append([]byte{value}, out[at:]...)...)
		}
	}
	return out
}

// Op is one edit of a script, as Apply reads it.
func Op(op byte, offset int, value byte) []byte {
	return []byte{op, byte(offset >> 8), byte(offset), value}
}
