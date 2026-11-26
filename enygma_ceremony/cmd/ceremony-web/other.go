//go:build !(js && wasm)

package main

import (
	"fmt"
	"os"
)

// main says what this command is for: it runs in a browser, built with
// GOOS=js GOARCH=wasm. On a computer, run cmd/ceremony.
func main() {
	fmt.Fprintln(os.Stderr, "ceremony-web runs in the browser (GOOS=js GOARCH=wasm, make ceremony-dist); on a computer, run cmd/ceremony")
	os.Exit(2)
}
