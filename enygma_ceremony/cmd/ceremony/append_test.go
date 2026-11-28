package main

import (
	"crypto/sha256"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"enygma_ceremony/coordinator"
)

// TestAppendCommand (audit H3): a lined-up contributor's contribution,
// made by hand on the latest state, is added to the phase, whose chain then
// verifies under the head append prints; append refuses what is no
// contribution on top of the latest, and a directory a coordinator serves.
func TestAppendCommand(t *testing.T) {
	dir, work := t.TempDir(), t.TempDir()
	if code, stdout, stderr := runCommand("phase1-new", "-size", "8", "-out", filepath.Join(dir, "state-0")); code != 0 {
		t.Fatalf("phase1-new: %d\n%s%s", code, stdout, stderr)
	}
	next := filepath.Join(work, "next")
	if code, stdout, stderr := runCommand("contribute", "-in", filepath.Join(dir, "state-0"), "-out", next); code != 0 {
		t.Fatalf("contribute: %d\n%s%s", code, stdout, stderr)
	}
	head := fmt.Sprintf("%x", sha256.Sum256(readFile(t, next)))

	if code, _, _ := runCommand("append", "-dir", dir, "-in", next); code != 2 {
		t.Fatalf("append without -name: exit %d, want 2", code)
	}
	inDir := filepath.Join(dir, "incoming-next")
	if err := os.WriteFile(inDir, readFile(t, next), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := runCommand("append", "-dir", dir, "-in", inDir, "-name", "Alice"); code != 2 || !strings.Contains(stderr, "inside -dir") {
		t.Fatalf("append of a file in the phase's directory: exit %d, want 2\n%s", code, stderr)
	}
	c, err := coordinator.New(coordinator.Config{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if code, stdout, stderr := runCommand("append", "-dir", dir, "-in", next, "-name", "Alice"); code != 1 {
		t.Fatalf("append while a coordinator serves the directory: exit %d, want 1\n%s%s", code, stdout, stderr)
	}
	c.Release()

	code, stdout, stderr := runCommand("append", "-dir", dir, "-in", next, "-name", "Alice")
	if code != 0 || !strings.Contains(stdout, "head sha256 "+head) || !strings.Contains(stdout, "-include") {
		t.Fatalf("append: exit %d\n%s%s", code, stdout, stderr)
	}
	if code, stdout, stderr := runCommand("verify", "-dir", dir, "-size", "8", "-head", head, "-mine", head); code != 0 {
		t.Fatalf("verify after append: exit %d\n%s%s", code, stdout, stderr)
	}
	if code, stdout, stderr := runCommand("append", "-dir", dir, "-in", next, "-name", "Alice"); code != 1 {
		t.Fatalf("append of what is no longer on top of the latest: exit %d, want 1\n%s%s", code, stdout, stderr)
	}
}

// TestDenyFile (audit H3): coordinate refuses the networks -deny-file names,
// and SIGHUP re-reads it, without a restart; a file it cannot read keeps
// the networks denied before.
func TestDenyFile(t *testing.T) {
	dir := t.TempDir()
	if code, stdout, stderr := runCommand("phase1-new", "-size", "8", "-out", filepath.Join(dir, "state-0")); code != 0 {
		t.Fatalf("phase1-new: %d\n%s%s", code, stdout, stderr)
	}
	deny := filepath.Join(t.TempDir(), "deny")
	if code, out := runBriefly(t, "coordinate", "-dir", dir, "-insecure-http", "-listen", "127.0.0.1:0", "-deny-file", deny); code != 1 || !strings.Contains(out, "-deny-file") {
		t.Fatalf("coordinate with a deny file it cannot read: exit %d, want 1\n%s", code, out)
	}
	if err := os.WriteFile(deny, []byte("not a network\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readDenyFile(deny); err == nil {
		t.Fatal("a deny file of no network: read, want it refused")
	}
	if err := os.WriteFile(deny, []byte("192.0.2.0/24, 198.51.100.0/24\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readDenyFile(deny); err == nil || !strings.Contains(err.Error(), "one address or CIDR a line") {
		t.Fatalf("a deny file with two networks on a line: %v, want it refused", err)
	}
	if err := os.WriteFile(deny, []byte("# this machine\n127.0.0.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	stops, reloads := make(chan chan<- os.Signal, 1), make(chan chan<- os.Signal, 1)
	notifyStop, notifyReload := stopSignals, reloadSignals
	stopSignals = func(c chan<- os.Signal) { stops <- c }
	reloadSignals = func(c chan<- os.Signal) { reloads <- c }
	t.Cleanup(func() { stopSignals, reloadSignals = notifyStop, notifyReload })

	var out lockedBuffer
	exited := make(chan int, 1)
	go func() {
		exited <- run([]string{"coordinate", "-dir", dir, "-insecure-http", "-listen", addr, "-deny-file", deny}, &out, &out)
	}()
	var interrupt, reload chan<- os.Signal
	for interrupt == nil || reload == nil {
		select {
		case interrupt = <-stops:
		case reload = <-reloads:
		case code := <-exited:
			t.Fatalf("coordinate exited %d before serving:\n%s", code, out.String())
		}
	}
	url := "http://" + addr
	registerCode := func() int {
		t.Helper()
		for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(10 * time.Millisecond) {
			resp, err := noKeepAlive.Post(url+"/v1/register", "application/json", strings.NewReader(`{"name":"bob"}`))
			if err == nil {
				resp.Body.Close()
				return resp.StatusCode
			}
			if time.Now().After(deadline) { // the coordinator may not be listening yet
				t.Fatal(err)
			}
		}
	}
	if code := registerCode(); code != http.StatusForbidden {
		t.Fatalf("registering from a network denied at the start: %d, want 403", code)
	}
	reread := func(content string, want int) {
		t.Helper()
		if err := os.WriteFile(deny, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		reload <- syscall.SIGHUP
		for deadline := time.Now().Add(10 * time.Second); registerCode() != want; time.Sleep(10 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatalf("registering after SIGHUP with %q: want %d\n%s", content, want, out.String())
			}
		}
	}
	reread("203.0.113.0/24 # documentation\n", http.StatusOK)
	reread("127.0.0.0/8\n", http.StatusForbidden)
	if err := os.WriteFile(deny, []byte("not a network\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	reload <- syscall.SIGHUP
	for deadline := time.Now().Add(10 * time.Second); !strings.Contains(out.String(), "denying the same networks as before"); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("coordinate never said it kept the networks denied:\n%s", out.String())
		}
	}
	if code := registerCode(); code != http.StatusForbidden {
		t.Fatalf("registering after a deny file it cannot read: %d, want 403 still", code)
	}
	reread("# nothing\n", http.StatusOK)
	waitFor(t, &out, "WARNING: SIGHUP: -deny-file names no networks")
	interrupt <- os.Interrupt
	interrupt <- os.Interrupt
	select {
	case <-exited:
	case <-time.After(10 * time.Second):
		t.Fatalf("coordinate did not stop:\n%s", out.String())
	}
}

// runBriefly runs a command that should exit at once, failing the test if
// it is still running after 10 s (a coordinate serving, say).
func runBriefly(t *testing.T, args ...string) (int, string) {
	t.Helper()
	var out lockedBuffer
	exited := make(chan int, 1)
	go func() { exited <- run(args, &out, &out) }()
	select {
	case code := <-exited:
		return code, out.String()
	case <-time.After(10 * time.Second):
		t.Fatalf("%s still running after 10 s:\n%s", args[0], out.String())
		return 0, ""
	}
}

// TestIPv4NetworksWrittenAsIPv6: an IPv4 network written as IPv6 (a
// ::ffff: prefix) is the IPv4 network, as addresses are compared unmapped.
func TestIPv4NetworksWrittenAsIPv6(t *testing.T) {
	got, err := parseProxies("::ffff:192.0.2.0/120, ::ffff:198.51.100.7")
	if err != nil {
		t.Fatal(err)
	}
	if want := []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24"), netip.MustParsePrefix("198.51.100.7/32")}; !slices.Equal(got, want) {
		t.Fatalf("parsed %v, want %v", got, want)
	}
	if got, err := parseProxies("::ffff:192.0.2.0/24"); err == nil {
		t.Fatalf("an IPv4 network written as IPv6, wider than /96: %v, want it refused", got)
	}
	for _, every := range []string{"::ffff:0:0/96", "0.0.0.0/0", "::/0"} {
		if got, err := parseProxies(every); err == nil {
			t.Errorf("%s, every address: %v, want it refused", every, got)
		}
	}
}

// TestInside: append's check that -in is not in the phase's directory sees
// through a link to it.
func TestInside(t *testing.T) {
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(real, "incoming"), 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(real, "incoming", "next")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "next")
	if err := os.WriteFile(outside, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	fileLink := filepath.Join(t.TempDir(), "next")
	if err := os.Symlink(file, fileLink); err != nil {
		t.Fatal(err)
	}
	linkInDir := filepath.Join(real, "incoming", "elsewhere")
	if err := os.Symlink(outside, linkInDir); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path, dir string
		want      bool
	}{{file, link, true}, {filepath.Join(link, "incoming", "next"), real, true}, {fileLink, real, true}, {linkInDir, real, true}, {outside, link, false}} {
		if got, err := inside(tc.path, tc.dir); err != nil || got != tc.want {
			t.Errorf("inside(%s, %s) = %v, %v; want %v", tc.path, tc.dir, got, err, tc.want)
		}
	}
}

// TestSIGHUP: SIGHUP never stops coordinate: without -deny-file it says
// there is nothing to re-read, and with one it re-reads it while waiting
// for the last turn after an interrupt, ending that turn when its network
// is denied.
func TestSIGHUP(t *testing.T) {
	grace, notifyStop, notifyReload := readGrace, stopSignals, reloadSignals
	readGrace = 100 * time.Millisecond
	t.Cleanup(func() { readGrace, stopSignals, reloadSignals = grace, notifyStop, notifyReload })
	for _, withFile := range []bool{false, true} {
		dir := t.TempDir()
		if code, stdout, stderr := runCommand("phase1-new", "-size", "8", "-out", filepath.Join(dir, "state-0")); code != 0 {
			t.Fatalf("phase1-new: %d\n%s%s", code, stdout, stderr)
		}
		deny := filepath.Join(t.TempDir(), "deny")
		if err := os.WriteFile(deny, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		addr := ln.Addr().String()
		if err := ln.Close(); err != nil {
			t.Fatal(err)
		}
		stops, reloads := make(chan chan<- os.Signal, 1), make(chan chan<- os.Signal, 1)
		stopSignals = func(c chan<- os.Signal) { stops <- c }
		reloadSignals = func(c chan<- os.Signal) { reloads <- c }
		args := []string{"coordinate", "-dir", dir, "-insecure-http", "-listen", addr}
		if withFile {
			args = append(args, "-deny-file", deny)
		}
		var out lockedBuffer
		exited := make(chan int, 1)
		go func() { exited <- run(args, &out, &out) }()
		var interrupt, reload chan<- os.Signal
		for interrupt == nil || reload == nil {
			select {
			case interrupt = <-stops:
			case reload = <-reloads:
			case code := <-exited:
				t.Fatalf("coordinate exited %d before serving:\n%s", code, out.String())
			}
		}
		stopSignals, reloadSignals = notifyStop, notifyReload
		url := "http://" + addr
		alice := register(t, url, "alice")
		if r := askTurn(t, url, alice); r.Status != coordinator.Go {
			t.Fatalf("alice: %+v, want the turn", r)
		}
		if !withFile {
			reload <- syscall.SIGHUP
			waitFor(t, &out, "no -deny-file to re-read")
			register(t, url, "bob") // still serving
			interrupt <- os.Interrupt
			interrupt <- os.Interrupt
			<-exited
			continue
		}
		interrupt <- os.Interrupt // waits for alice's turn
		waitFor(t, &out, "stopping: no new turns")
		if err := os.WriteFile(deny, []byte("127.0.0.0/8\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		reload <- syscall.SIGHUP
		select {
		case <-exited:
		case <-time.After(10 * time.Second):
			t.Fatalf("coordinate still waited on a turn whose network was denied:\n%s", out.String())
		}
		if !strings.Contains(out.String(), "turn ended: its network is denied") {
			t.Fatalf("the log does not say the turn ended:\n%s", out.String())
		}
	}
}

// waitFor waits until out holds want.
func waitFor(t *testing.T, out *lockedBuffer, want string) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); !strings.Contains(out.String(), want); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("coordinate never said %q:\n%s", want, out.String())
		}
	}
}
