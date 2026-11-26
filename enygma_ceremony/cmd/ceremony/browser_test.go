package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestBrowserClientContributes: the program built for the browser
// (cmd/ceremony-web) takes a participant through a phase as `ceremony join`
// does, reporting each step to the page: it joins, takes its turn,
// contributes and uploads, and the transcript holds its contribution. It runs
// under Node as the page's worker runs it (testdata/browser.js); without
// Node, the test is skipped.
func TestBrowserClientContributes(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("needs node, which runs the browser's client here")
	}
	goroot, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		t.Fatal(err)
	}
	wasm := filepath.Join(t.TempDir(), "ceremony.wasm")
	build := exec.Command("go", "build", "-o", wasm, "../ceremony-web")
	build.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building for the browser: %v\n%s", err, out)
	}
	dir, url := openPhase(t)

	loader := filepath.Join(strings.TrimSpace(string(goroot)), "lib", "wasm", "wasm_exec.js")
	out, err := exec.Command(node, "testdata/browser.js", loader, wasm, url, "browser").Output()
	var got struct {
		Result struct {
			Index  uint32 `json:"index"`
			SHA256 string `json:"sha256"`
		} `json:"result"`
		Stages []string `json:"stages"`
		Error  string   `json:"error"`
	}
	if jerr := json.Unmarshal(out, &got); err != nil || jerr != nil {
		t.Fatalf("the browser's client: %v %v\n%s", err, jerr, out)
	}
	if want := []string{"joined", "turn", "downloading", "contributing", "uploading", "done"}; !slices.Equal(got.Stages, want) {
		t.Errorf("reported %v, want %v", got.Stages, want)
	}
	state, err := os.ReadFile(filepath.Join(dir, "state-1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var entry struct{ Participant, SHA256 string }
	if err := json.Unmarshal(state, &entry); err != nil {
		t.Fatal(err)
	}
	if got.Result.Index != 1 || entry.SHA256 != got.Result.SHA256 || entry.Participant != "browser" {
		t.Fatalf("the browser's client reported %+v; the transcript holds %+v", got.Result, entry)
	}
}
