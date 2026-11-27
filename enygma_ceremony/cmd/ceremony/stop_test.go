package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"enygma_ceremony/coordinator"
)

// TestCoordinateStops: an interrupt stops the coordinator but does not close
// its phase. A queued participant who asks meanwhile is told to keep waiting,
// so that their program joins the coordinator started next, rather than give
// up on a phase it was told is closed; and on stopping, the coordinator says
// the phase carries on.
func TestCoordinateStops(t *testing.T) {
	dir := t.TempDir()
	if code, stdout, stderr := runCommand("phase1-new", "-size", "8", "-out", filepath.Join(dir, "state-0")); code != 0 {
		t.Fatalf("phase1-new: %d\n%s%s", code, stdout, stderr)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}

	signals := make(chan chan<- os.Signal, 1)
	notify := stopSignals
	stopSignals = func(c chan<- os.Signal) { signals <- c }
	t.Cleanup(func() { stopSignals = notify })

	var out lockedBuffer
	exited := make(chan int, 1)
	go func() {
		exited <- run([]string{"coordinate", "-dir", dir, "-insecure-http", "-listen", addr}, &out, &out)
	}()
	var interrupt chan<- os.Signal
	select {
	case interrupt = <-signals:
	case code := <-exited:
		t.Fatalf("coordinate exited %d before serving:\n%s", code, out.String())
	}
	url := "http://" + addr
	alice, bob := register(t, url, "alice"), register(t, url, "bob")
	if r := askTurn(t, url, alice); r.Status != coordinator.Go {
		t.Fatalf("alice: %+v, want the turn", r)
	}
	if r := askTurn(t, url, bob); r.Status != coordinator.Queued {
		t.Fatalf("bob: %+v, want queued", r)
	}

	interrupt <- os.Interrupt
	for deadline := time.Now().Add(10 * time.Second); !strings.Contains(out.String(), "stopping: no new turns"); {
		if time.Now().After(deadline) {
			t.Fatalf("coordinate never said it was stopping:\n%s", out.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	if r := askTurn(t, url, bob); r.Status != coordinator.Queued || r.Position != 1 {
		t.Fatalf("bob, while the coordinator stops: %+v, want queued at 1", r)
	}
	interrupt <- os.Interrupt // stop at once, rather than wait for alice's turn
	select {
	case code := <-exited:
		if code != 0 {
			t.Fatalf("coordinate: exit %d\n%s", code, out.String())
		}
	case <-time.After(10 * time.Second): // not readGrace's 30 s
		t.Fatalf("coordinate did not stop at once on a second interrupt:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "Stopped after 0 contributions. The phase is not closed") {
		t.Fatalf("coordinate printed\n%s\nwant it to say the phase carries on", out.String())
	}
}

// register registers name with the coordinator at url and returns its session token.
func register(t *testing.T, url, name string) string {
	t.Helper()
	var s coordinator.Session
	post(t, url, "/v1/register", "", `{"name":"`+name+`"}`, &s)
	return s.Token
}

// askTurn asks the coordinator at url for token's turn.
func askTurn(t *testing.T, url, token string) coordinator.Reply {
	t.Helper()
	var r coordinator.Reply
	post(t, url, "/v1/turn", token, "", &r)
	return r
}

// noKeepAlive closes each connection after its request, so that none is
// left for the stopping coordinator to wait on.
var noKeepAlive = &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}

// post posts body to url+path, with token as its bearer when given, and
// decodes the JSON reply into v. It tries again while nothing answers: the
// coordinator may not be listening yet.
func post(t *testing.T, url, path, token, body string, v any) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		req, err := http.NewRequest(http.MethodPost, url+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := noKeepAlive.Do(req)
		if err != nil {
			if time.Now().After(deadline) {
				t.Fatal(err)
			}
			time.Sleep(10 * time.Millisecond)
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			msg, _ := io.ReadAll(resp.Body)
			t.Fatalf("POST %s: %s %s", path, resp.Status, msg)
		}
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			t.Fatalf("POST %s: %v", path, err)
		}
		return
	}
}

// lockedBuffer is a bytes.Buffer that a command's goroutines and a test can share.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// TestCoordinateAnswersAfterItStops (audit L5, H2): stopped while a turn
// is under way, or just after a contribution, coordinate still serves the
// transcript for a moment, read-only, so the program that made the last
// contribution can read it back; stopped with neither, it stops at once.
// It tells the operator
// to check the contributors lined up are in the chain before announcing
// its head, and to seal with their receipts.
func TestCoordinateAnswersAfterItStops(t *testing.T) {
	for _, c := range []struct{ turnUnderWay, contributed bool }{{true, false}, {false, true}, {false, false}} {
		turnUnderWay := c.turnUnderWay
		dir := t.TempDir()
		if code, stdout, stderr := runCommand("phase1-new", "-size", "8", "-out", filepath.Join(dir, "state-0")); code != 0 {
			t.Fatalf("phase1-new: %d\n%s%s", code, stdout, stderr)
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		addr := ln.Addr().String()
		if err := ln.Close(); err != nil {
			t.Fatal(err)
		}
		signals := make(chan chan<- os.Signal, 1)
		notify, grace := stopSignals, readGrace
		stopSignals, readGrace = func(c chan<- os.Signal) { signals <- c }, 3*time.Second
		t.Cleanup(func() { stopSignals, readGrace = notify, grace })

		var out lockedBuffer
		exited := make(chan int, 1)
		go func() {
			exited <- run([]string{"coordinate", "-dir", dir, "-insecure-http", "-listen", addr, "-turn", "2s"}, &out, &out)
		}()
		var interrupt chan<- os.Signal
		select {
		case interrupt = <-signals:
		case code := <-exited:
			t.Fatalf("coordinate exited %d before serving:\n%s", code, out.String())
		}
		url := "http://" + addr
		if turnUnderWay {
			if r := askTurn(t, url, register(t, url, "alice")); r.Status != coordinator.Go {
				t.Fatalf("alice: %+v, want the turn", r)
			}
		}
		if c.contributed {
			var joined lockedBuffer
			if code := run([]string{"join", "-url", url, "-name", "bob"}, &joined, &joined); code != 0 {
				t.Fatalf("bob's join: %d\n%s", code, joined.String())
			}
		}
		interrupt <- os.Interrupt
		if !turnUnderWay && !c.contributed {
			select {
			case <-exited:
			case <-time.After(2 * time.Second):
				t.Fatalf("coordinate, stopped with no turn under way, did not stop at once:\n%s", out.String())
			}
			if strings.Contains(out.String(), "read-only, for") {
				t.Fatalf("coordinate served on with no turn under way:\n%s", out.String())
			}
			continue
		}
		for deadline := time.Now().Add(10 * time.Second); !strings.Contains(out.String(), "read-only, for"); {
			if time.Now().After(deadline) {
				t.Fatalf("coordinate never said it serves on, read-only:\n%s", out.String())
			}
			time.Sleep(10 * time.Millisecond)
		}
		resp, err := http.Get(url + "/v1/transcript")
		if err != nil {
			t.Fatalf("the transcript, while coordinate serves on: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("the transcript, while coordinate serves on: %d", resp.StatusCode)
		}
		if code := <-exited; code != 0 {
			t.Fatalf("coordinate: exit %d\n%s", code, out.String())
		}
		if !strings.Contains(out.String(), "-size 8 -mine <their published receipts") || !strings.Contains(out.String(), "-include <their receipts>") {
			t.Fatalf("coordinate printed\n%s\nwant the check of the contributors lined up, and -include", out.String())
		}
	}
}
