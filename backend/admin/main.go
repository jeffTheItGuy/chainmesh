package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yourname/blockmesh/shared/logger"
	"github.com/yourname/blockmesh/shared/storage/postgres"
)

func main() {
	log := logger.New()

	db, err := postgres.New(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Error("postgres failed", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/tenants", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]string{})
	})
	mux.HandleFunc("/usage", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{})
	})

	srv := &http.Server{Addr: ":8081", Handler: mux}

	go func() {
		log.Info("admin starting", "addr", srv.Addr)
		srv.ListenAndServe()
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
