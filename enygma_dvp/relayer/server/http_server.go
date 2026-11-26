package server

import (
	"math"
	"net/http"
	"time"
)

// NewHTTPServer sets connection deadlines for the relayer.
func NewHTTPServer(addr string, handler http.Handler, miningTimeout time.Duration) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      relayWriteTimeout(miningTimeout),
		IdleTimeout:       120 * time.Second,
	}
}

// relayWriteTimeout allows for the body read, simulation, mining and response.
func relayWriteTimeout(miningTimeout time.Duration) time.Duration {
	const writeHeadroom = simulateTimeout + 30*time.Second
	return min(miningTimeout, math.MaxInt64-writeHeadroom) + writeHeadroom
}
