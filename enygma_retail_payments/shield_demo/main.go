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
//	go run . -root .. -dev            # serve index.html from disk: edit, refresh
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
	// pageFile, if set, is read for the page on every request instead of
	// the embedded copy (-dev).
	pageFile string
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
	mux.HandleFunc("/events", s.handleEvents)
	mux.HandleFunc("/defaults", s.handleDefaults)
	mux.HandleFunc("/run", s.handleRun)
	mux.HandleFunc("/transfer", s.handleTransfer)
	// PROTOTYPE (privacy story): serves prototype/events.jsonl, a recorded run the page replays.
	mux.Handle("/prototype/", http.StripPrefix("/prototype/", http.FileServer(http.Dir("prototype"))))
	return mux
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	page := indexHTML
	if s.pageFile != "" {
		b, err := os.ReadFile(s.pageFile)
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

// handleDefaults returns the default run options for the page's form.
func (s *server) handleDefaults(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(engine.DefaultOptions())
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
		http.Error(w, `{"error":"a run is already in progress"}`, http.StatusConflict)
		return
	}
	go func() {
		defer s.running.Unlock()
		if s.engine != nil {
			s.engine.Close()
			s.engine = nil
		}
		s.broker.publish(map[string]string{"kind": "reset"})
		e := engine.New(s.cfg, s.broker)
		if err := e.Run(context.Background(), opts); err != nil {
			log.Printf("run failed: %v", err)
			e.Close()
			return
		}
		s.engine = e
	}()
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"status":"started"}`)
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
	dev := flag.Bool("dev", false, "serve <root>/shield_demo/index.html from disk on every request instead of the embedded copy, so a page edit needs only a refresh")
	flag.Parse()

	cfg := engine.DefaultConfig(*root)
	cfg.StepPause = 1200 * time.Millisecond
	s := newServer(context.Background(), cfg, *logDir)
	if *dev {
		s.pageFile = filepath.Join(*root, "shield_demo", "index.html")
	}

	log.Printf("Enygma Retail Shield Demo listening on http://localhost%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, s.routes()))
}
