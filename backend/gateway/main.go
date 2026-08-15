package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yourname/blockmesh/gateway/middleware"
	"github.com/yourname/blockmesh/gateway/proxy"
	"github.com/yourname/blockmesh/shared/blockchain"
	"github.com/yourname/blockmesh/shared/logger"
	"github.com/yourname/blockmesh/shared/storage/postgres"
	"github.com/yourname/blockmesh/shared/storage/redis"
)

func main() {
	log := logger.New()

	db, err := postgres.New(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Error("postgres failed", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	cache := redis.New(os.Getenv("REDIS_ADDR"))
	defer cache.Close()

	endpoints := []string{os.Getenv("RPC_ENDPOINT_1")}
	if ep2 := os.Getenv("RPC_ENDPOINT_2"); ep2 != "" {
		endpoints = append(endpoints, ep2)
	}

	bc := blockchain.New(endpoints)
	p := proxy.New(bc, db, cache, log)

	mux := http.NewServeMux()
	handler := middleware.Auth(db)(middleware.RateLimit(cache)(p))
	mux.Handle("/v1/", handler)

	srv := &http.Server{Addr: ":8080", Handler: mux}
	go func() {
		log.Info("gateway starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server failed", "err", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
