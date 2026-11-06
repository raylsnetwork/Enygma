// Enygma Retail Shield Demo — the web UI over the shield demo's scenario
// engine. It streams the engine's events to the page as Server-Sent Events
// and starts runs from the page's options. Given the services' log
// directory, it also streams their logs to the page's terminal.
//
// Start the whole stack with run.sh; this binary alone expects the chain, the
// gnark server and the relayer to be up already.
//
//	go run . -root ..                 # then open http://localhost:9092
//	go run . -root .. -logs $LOG_DIR  # with run.sh's service logs
//	go run . -root .. -dev            # serve the pages from disk: edit, refresh
//
// It also serves the Protocol primer, a static page, at /protocol.
package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"enygma_retail_payments/shield_demo/engine"
)

//go:embed index.html
var indexHTML string

// protocolHTML is the Protocol primer, served at /protocol: a static page
// that makes no requests, so it also opens from disk.
//
//go:embed protocol.html
var protocolHTML string

// broker fans engine events and service-log lines out to every connected
// SSE client, and keeps each service's last replayLines lines for new ones.
type broker struct {
	mu      sync.Mutex
	clients map[chan []byte]struct{}
	// replay holds each service's latest lines, oldest first.
	replay map[string][][]byte
}

// replayLines is how many of each service's lines a new client is sent.
const replayLines = 1000

func newBroker() *broker {
	return &broker{clients: map[chan []byte]struct{}{}, replay: map[string][][]byte{}}
}

// subscribe adds a client and returns the service lines to replay to it
// first; lines published after it come on the channel.
func (b *broker) subscribe() (chan []byte, [][]byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan []byte, 512)
	b.clients[ch] = struct{}{}
	var replay [][]byte
	for _, svc := range services {
		replay = append(replay, b.replay[svc]...)
	}
	return ch, replay
}

func (b *broker) unsubscribe(ch chan []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.clients, ch)
}

func (b *broker) publish(v any) {
	data, _ := json.Marshal(v)
	b.mu.Lock()
	defer b.mu.Unlock()
	b.send(data)
}

// publishLine publishes one service-log line and keeps it for replay.
func (b *broker) publishLine(e termEvent) {
	data, _ := json.Marshal(e)
	b.mu.Lock()
	defer b.mu.Unlock()
	r := append(b.replay[e.Service], data)
	if len(r) > replayLines {
		r = r[len(r)-replayLines:]
	}
	b.replay[e.Service] = r
	b.send(data)
}

// send hands data to every client that has room for it; b.mu is held.
func (b *broker) send(data []byte) {
	for ch := range b.clients {
		select {
		case ch <- data:
		default:
		}
	}
}

// Emit makes the broker the engine's Sink, stamping log lines with a time.
func (b *broker) Emit(e engine.Event) {
	b.publish(struct {
		engine.Event
		TS string `json:"ts"`
	}{e, stamp()})
}

// stamp is the time the page shows beside a line.
func stamp() string { return time.Now().Format("15:04:05.000") }

type server struct {
	cfg    engine.Config
	broker *broker
	// running is held by a run or a manual transfer; engine is the last run
	// that completed, which manual transfers continue.
	running sync.Mutex
	engine  *engine.Engine
	// live is the run in progress, whose gate /release opens, and
	// endWalk ends it if it is a walkthrough; both are nil between runs.
	mu      sync.Mutex
	live    *engine.Engine
	endWalk context.CancelFunc
	// pageFile and primerFile, if set, are read for the page and the
	// primer on every request instead of the embedded copies (-dev).
	pageFile, primerFile string
}

// newServer serves runs with cfg. With a logDir, it tails the services' logs
// in it into the event stream until ctx ends.
func newServer(ctx context.Context, cfg engine.Config, logDir string) *server {
	s := &server{cfg: cfg, broker: newBroker()}
	if logDir != "" {
		s.broker.tailServiceLogs(ctx, logDir)
	}
	return s
}

func (s *server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/protocol", s.handlePrimer)
	mux.HandleFunc("/events", s.handleEvents)
	mux.HandleFunc("/defaults", s.handleDefaults)
	mux.HandleFunc("/run", s.handleRun)
	mux.HandleFunc("/transfer", s.handleTransfer)
	mux.HandleFunc("/release", s.handleRelease)
	return mux
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	servePage(w, indexHTML, s.pageFile)
}

func (s *server) handlePrimer(w http.ResponseWriter, r *http.Request) {
	servePage(w, protocolHTML, s.primerFile)
}

// servePage writes the embedded page, or the file's contents when file is set.
func servePage(w http.ResponseWriter, embedded, file string) {
	page := embedded
	if file != "" {
		b, err := os.ReadFile(file)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		page = string(b)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, page)
}

func (s *server) handleEvents(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch, replay := s.broker.subscribe()
	defer s.broker.unsubscribe(ch)
	for _, msg := range replay {
		fmt.Fprintf(w, "data: %s\n\n", msg)
	}
	fl.Flush()
	tick := time.NewTicker(20 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
			fmt.Fprint(w, ": ping\n\n")
			fl.Flush()
		case msg := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			fl.Flush()
		}
	}
}

// defaults is /defaults' body: the default run options for the page's
// form and, with a deployment, the Verifier its on-chain proof check calls.
type defaults struct {
	engine.Options
	Chain *engine.Verifier `json:"chain,omitempty"`
}

// handleDefaults returns the default run options and the deployment's
// Verifier, if build/receipts.json names one.
func (s *server) handleDefaults(w http.ResponseWriter, r *http.Request) {
	d := defaults{Options: engine.DefaultOptions()}
	if v, err := engine.DeployedVerifier(s.cfg); err == nil {
		d.Chain = &v
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(d)
}

func (s *server) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	opts := engine.DefaultOptions()
	if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
		writeError(w, http.StatusBadRequest, "bad options")
		return
	}
	if err := opts.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !s.running.TryLock() {
		// A walkthrough waits on its presenter, who may have left it: a
		// new run ends it, at its gate or in its current action (whose
		// transaction may still mine; the new run's actors are fresh).
		if !s.endWalkthrough() {
			http.Error(w, `{"error":"a run is already in progress"}`, http.StatusConflict)
			return
		}
		s.running.Lock()
	}
	ctx, cancel := context.WithCancel(context.Background())
	e := engine.New(s.cfg, s.broker)
	s.mu.Lock()
	s.live, s.endWalk = e, nil
	if opts.Walkthrough {
		s.endWalk = cancel
	}
	s.mu.Unlock()
	go func() {
		defer s.running.Unlock()
		defer func() {
			s.mu.Lock()
			s.live, s.endWalk = nil, nil
			s.mu.Unlock()
			cancel()
		}()
		if s.engine != nil {
			s.engine.Close()
			s.engine = nil
		}
		s.broker.publish(map[string]string{"kind": "reset"})
		if err := e.Run(ctx, opts); err != nil {
			log.Printf("run failed: %v", err)
			e.Close()
			return
		}
		s.engine = e
	}()
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"status":"started"}`)
}

// endWalkthrough ends the run in progress if it is a walkthrough, and
// reports whether it was.
func (s *server) endWalkthrough() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.endWalk == nil {
		return false
	}
	s.endWalk()
	return true
}

// handleRelease lets a waiting walkthrough perform its next action.
func (s *server) handleRelease(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	e := s.live
	s.mu.Unlock()
	if e == nil || !e.Gate().Release() {
		writeError(w, http.StatusConflict, "no walkthrough is waiting for Next")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"status":"released"}`)
}

// handleTransfer makes a manual transfer on the last completed run and
// answers once it has settled, or with why it was refused.
func (s *server) handleTransfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var t engine.Transfer
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeError(w, http.StatusBadRequest, "bad transfer")
		return
	}
	if !s.running.TryLock() {
		writeError(w, http.StatusConflict, "a run or transfer is already in progress")
		return
	}
	defer s.running.Unlock()
	if s.engine == nil {
		writeError(w, http.StatusConflict, "run the scenario first")
		return
	}
	if err := s.engine.Transfer(context.Background(), t.From, t.To, t.Amount); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, engine.ErrRefused) {
			status = http.StatusUnprocessableEntity
		}
		writeError(w, status, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"status":"settled"}`)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func main() {
	root := flag.String("root", "..", "enygma_retail_payments directory")
	addr := flag.String("addr", ":9092", "listen address")
	logDir := flag.String("logs", "", "directory of the services' logs (hardhat.log, gnark.log, relayer.log) to show in the terminal; none if empty")
	dev := flag.Bool("dev", false, "serve <root>/shield_demo/index.html and protocol.html from disk on every request instead of the embedded copies, so a page edit needs only a refresh")
	flag.Parse()

	cfg := engine.DefaultConfig(*root)
	cfg.StepPause = 1200 * time.Millisecond
	s := newServer(context.Background(), cfg, *logDir)
	if *dev {
		s.pageFile = filepath.Join(*root, "shield_demo", "index.html")
		s.primerFile = filepath.Join(*root, "shield_demo", "protocol.html")
	}

	log.Printf("Enygma Retail Shield Demo listening on http://localhost%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, s.routes()))
}
