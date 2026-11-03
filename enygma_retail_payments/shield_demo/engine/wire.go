package engine

import (
	"bytes"
	"io"
	"net/http"
	"sync"
	"time"
)

// recorder is an http.RoundTripper that keeps the last request it sent and
// the response it got back, bodies byte for byte, so a hop can show what
// went over the wire. Headers are left out: the relayer's carry its API key.
type recorder struct {
	mu   sync.Mutex
	last *HopDetail
}

// RoundTrip sends req with the default transport and keeps it and its
// response as r's last exchange.
func (r *recorder) RoundTrip(req *http.Request) (*http.Response, error) {
	var sent []byte
	if req.Body != nil {
		var err error
		if sent, err = io.ReadAll(req.Body); err != nil {
			return nil, err
		}
		req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(sent))
	}
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	got, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(got))
	r.mu.Lock()
	r.last = &HopDetail{Kind: DetailHTTP, Method: req.Method, URL: req.URL.String(), Status: resp.StatusCode,
		Request: string(sent), Response: string(got)}
	r.mu.Unlock()
	return resp, nil
}

// client is an HTTP client that records through r, timing out after timeout.
func (r *recorder) client(timeout time.Duration) *http.Client {
	return &http.Client{Transport: r, Timeout: timeout}
}

// exchange is the http hop detail of the last request r sent.
func (r *recorder) exchange() HopDetail {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.last == nil {
		return HopDetail{Kind: DetailHTTP}
	}
	return *r.last
}
