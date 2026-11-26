package server

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type deadlineRecorder struct {
	http.ResponseWriter
	record func(time.Time)
}

func (w deadlineRecorder) SetWriteDeadline(deadline time.Time) error {
	w.record(deadline)
	return http.NewResponseController(w.ResponseWriter).SetWriteDeadline(deadline)
}

func TestQueuedTransactionKeepsResponseDeadline(t *testing.T) {
	node := (&mockNode{}).serve(t)
	t.Cleanup(node.Close)
	h := newTestHandler(t, node.URL, 8_000_000)
	h.txMu.Lock()
	release := sync.OnceFunc(h.txMu.Unlock)

	const initialDeadline = 400 * time.Millisecond
	arrived := make(chan struct{})
	var deadlineMu sync.Mutex
	var deadlines []time.Time
	record := func(deadline time.Time) {
		deadlineMu.Lock()
		defer deadlineMu.Unlock()
		deadlines = append(deadlines, deadline)
	}
	router := gin.New()
	router.POST("/", func(c *gin.Context) {
		close(arrived)
		if _, err := h.transact(c.Writer, "ping"); err != nil {
			c.String(http.StatusInternalServerError, "%s", err)
			return
		}
		c.Status(http.StatusNoContent)
	})
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		router.ServeHTTP(deadlineRecorder{ResponseWriter: w, record: record}, r)
	}))
	srv.Config.WriteTimeout = initialDeadline
	srv.Start()
	defer srv.Close()
	defer release()

	type result struct {
		resp *http.Response
		err  error
	}
	outcome := make(chan result, 1)
	client := &http.Client{Timeout: 5 * time.Second}
	go func() {
		resp, err := client.Post(srv.URL, "application/json", nil)
		outcome <- result{resp, err}
	}()

	select {
	case <-arrived:
	case <-time.After(3 * time.Second):
		t.Fatal("request did not reach the server")
	}
	// Keep the submission lock contended past the connection's write deadline.
	time.Sleep(initialDeadline + 150*time.Millisecond)
	release()
	got := <-outcome
	if got.err != nil {
		t.Fatalf("queued transaction lost its HTTP response: %v", got.err)
	}
	defer got.resp.Body.Close()
	if got.resp.StatusCode != http.StatusNoContent {
		t.Fatalf("queued transaction returned %d, want 204", got.resp.StatusCode)
	}
	deadlineMu.Lock()
	defer deadlineMu.Unlock()
	if len(deadlines) != 2 || !deadlines[0].IsZero() || !deadlines[1].After(time.Now()) {
		t.Fatal("queued request must pause its deadline and restore it before the response")
	}
}
