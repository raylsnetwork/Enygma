package server_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"enygma_dvp/relayer/config"
	"enygma_dvp/relayer/server"
)

func TestHTTPServerTimeouts(t *testing.T) {
	t.Setenv("RELAYER_PRIVATE_KEY", "unused")
	t.Setenv("RELAYER_API_KEY", "test")
	t.Setenv("RELAYER_TX_TIMEOUT", "")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	srv := server.NewHTTPServer(":8091", http.NotFoundHandler(), cfg.TxTimeout)
	if srv.ReadHeaderTimeout <= 0 {
		t.Error("read header timeout must be positive")
	}
	if srv.ReadTimeout <= 0 {
		t.Error("read timeout must be positive")
	}
	if srv.IdleTimeout <= 0 {
		t.Error("idle timeout must be positive")
	}
	if srv.WriteTimeout <= 2*time.Minute+60*time.Second {
		t.Error("write timeout must cover default mining and simulation")
	}
	t.Setenv("RELAYER_TX_TIMEOUT", "5m")
	cfg, err = config.Load()
	if err != nil {
		t.Fatal(err)
	}
	srv = server.NewHTTPServer(":8091", http.NotFoundHandler(), cfg.TxTimeout)
	if srv.WriteTimeout <= 5*time.Minute+60*time.Second {
		t.Error("write timeout must cover configured mining and simulation")
	}
}

func TestHTTPServerTimeoutDurationBoundary(t *testing.T) {
	t.Setenv("RELAYER_PRIVATE_KEY", "unused")
	t.Setenv("RELAYER_API_KEY", "test")
	t.Run("largest supported mining timeout", func(t *testing.T) {
		t.Setenv("RELAYER_TX_TIMEOUT", "2562047h47m16.854775806s")
		cfg, err := config.Load()
		if err != nil {
			t.Fatal(err)
		}
		srv := server.NewHTTPServer(":8091", http.NotFoundHandler(), cfg.TxTimeout)
		if srv.WriteTimeout <= cfg.TxTimeout {
			t.Fatalf("write timeout %s must exceed mining timeout %s", srv.WriteTimeout, cfg.TxTimeout)
		}
	})
	t.Run("no room for a write deadline", func(t *testing.T) {
		t.Setenv("RELAYER_TX_TIMEOUT", "2562047h47m16.854775807s")
		if _, err := config.Load(); err == nil || !strings.Contains(err.Error(), "RELAYER_TX_TIMEOUT") {
			t.Fatalf("maximum duration returned %v, want a RELAYER_TX_TIMEOUT error", err)
		}
	})
}
