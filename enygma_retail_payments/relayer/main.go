package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"enygma_relayer/config"
	"enygma_relayer/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	r, h, err := server.New(cfg)
	if err != nil {
		log.Fatalf("server init: %v", err)
	}
	// h.Close() must run on every exit path, including the log.Fatalf below
	// (which calls os.Exit and would otherwise skip a plain `defer`) — so it
	// releases the persistent idempotency store's bbolt file lock cleanly
	// instead of only on a hard exit. Called explicitly on each path rather
	// than deferred.

	addr := fmt.Sprintf(":%s", cfg.Port)
	srv := &http.Server{Addr: addr, Handler: r}

	serveErr := make(chan error, 1)
	go func() {
		log.Printf("Enygma relayer listening on %s", addr)
		serveErr <- srv.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serveErr:
		h.Close()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("run: %v", err)
		}
	case sig := <-stop:
		log.Printf("relayer: received %s, shutting down", sig)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("relayer: graceful shutdown failed: %v", err)
		}
		h.Close()
	}
}
