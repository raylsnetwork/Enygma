package server_test

import (
	"net/http"
	"testing"
	"time"

	"enygma_retail_payments/relayer/server"
)

func TestHTTPServerTimeouts(t *testing.T) {
	srv := server.NewHTTPServer(":8090", http.NotFoundHandler())
	if srv.ReadHeaderTimeout <= 0 {
		t.Error("read header timeout must be positive")
	}
	if srv.ReadTimeout <= 0 {
		t.Error("read timeout must be positive")
	}
	if srv.IdleTimeout <= 0 {
		t.Error("idle timeout must be positive")
	}
	if srv.WriteTimeout <= 45*time.Second+15*time.Second {
		t.Error("write timeout must cover mining and simulation")
	}
}
