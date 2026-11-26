package server

import (
	"net/http"
	"time"
)

// NewHTTPServer sets connection deadlines for the relayer.
func NewHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		// Allow for the body read, simulation, mining and response.
		WriteTimeout: txTimeout + simulateTimeout + 30*time.Second,
		IdleTimeout:  120 * time.Second,
	}
}
