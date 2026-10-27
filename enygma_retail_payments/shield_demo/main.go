// Enygma Retail Shield Demo — the web UI over the shield demo's scenario
// engine. It streams the engine's events to the page as Server-Sent Events
// and starts runs from the page's options.
//
// Start the whole stack with run.sh; this binary alone expects the chain, the
// gnark server and the relayer to be up already.
//
//	go run . -root ..   # then open http://localhost:9092
package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"enygma_retail_payments/shield_demo/engine"
)

//go:embed index.html
var indexHTML string

// broker fans engine events out to every connected SSE client.
type broker struct {
	mu      sync.Mutex
	clients map[chan []byte]struct{}
}

func (b *broker) subscribe() chan []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan []byte, 512)
	b.clients[ch] = struct{}{}
	return ch
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
	}{e, time.Now().Format("15:04:05.000")})
}

type server struct {
	cfg     engine.Config
	broker  *broker
	running sync.Mutex
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, indexHTML)
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

	ch := s.broker.subscribe()
	defer s.broker.unsubscribe(ch)
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
		http.Error(w, `{"error":"bad options"}`, http.StatusBadRequest)
		return
	}
	if !s.running.TryLock() {
		http.Error(w, `{"error":"a run is already in progress"}`, http.StatusConflict)
		return
	}
	go func() {
		defer s.running.Unlock()
		s.broker.publish(map[string]string{"kind": "reset"})
		e := engine.New(s.cfg, s.broker)
		defer e.Close()
		if err := e.Run(context.Background(), opts); err != nil {
			log.Printf("run failed: %v", err)
		}
	}()
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"status":"started"}`)
}

func main() {
	root := flag.String("root", "..", "enygma_retail_payments directory")
	addr := flag.String("addr", ":9092", "listen address")
	flag.Parse()

	cfg := engine.DefaultConfig(*root)
	cfg.StepPause = 1200 * time.Millisecond
	s := &server{cfg: cfg, broker: &broker{clients: map[chan []byte]struct{}{}}}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/events", s.handleEvents)
	mux.HandleFunc("/defaults", s.handleDefaults)
	mux.HandleFunc("/run", s.handleRun)

	log.Printf("Enygma Retail Shield Demo listening on http://localhost%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
