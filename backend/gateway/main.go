package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yourname/blockmesh/gateway/middleware"
	"github.com/yourname/blockmesh/gateway/proxy"
	"github.com/yourname/blockmesh/shared/logger"
	"github.com/yourname/blockmesh/shared/statsrollup"
	"github.com/yourname/blockmesh/shared/storage/postgres"
	"github.com/yourname/blockmesh/shared/storage/redis"
	"github.com/yourname/blockmesh/shared/telemetry"
)

type SafeEndpointHealth struct {
	URL              string    `json:"url"`
	Healthy          bool      `json:"healthy"`
	LatencyMs        int64     `json:"latency_ms"`
	LastCheck        time.Time `json:"last_check"`
	ConsecutiveFails int       `json:"consecutive_fails"`
	TotalRequests    int64     `json:"total_requests"`
	TotalFailures    int64     `json:"total_failures"`
}

type SafeNetworkHealth struct {
	NetworkID string               `json:"network_id"`
	Nodes     []SafeEndpointHealth `json:"nodes"`
}

func redactURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "redacted-endpoint"
	}
	// Keep the host (e.g., eth-mainnet.g.alchemy.com) but hide the path/API key
	return u.Host + "/***"
}

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

	telemetryRecorder := telemetry.New(db, log, 10000)
	telemetryRecorder.Start()
	defer telemetryRecorder.Stop()

	rollupRefresher := statsrollup.New(db, log, time.Minute)
	rollupRefresher.Start()
	defer rollupRefresher.Stop()

	manager := NewManager(db, log)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err = manager.Start(ctx)
	cancel()
	if err != nil {
		log.Error("failed to start blockchain manager", "err", err)
		os.Exit(1)
	}
	defer manager.Stop()

	p := proxy.New(manager, db, cache, log, telemetryRecorder)

	handler := middleware.RequestID(
		middleware.Auth(db)(
			middleware.RateLimit(cache)(p),
		),
	)

	mux := http.NewServeMux()
	mux.Handle("/v1/", handler)

	mux.HandleFunc("/health/nodes", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		
		health := manager.Health()
		safeHealth := make([]SafeNetworkHealth, len(health))
		
		for i, net := range health {
			safeNodes := make([]SafeEndpointHealth, len(net.Nodes))
			for j, node := range net.Nodes {
				safeNodes[j] = SafeEndpointHealth{
					URL:              redactURL(node.URL),
					Healthy:          node.Healthy,
					LatencyMs:        node.Latency.Milliseconds(), // Convert ns to ms
					LastCheck:        node.LastCheck,
					ConsecutiveFails: node.ConsecutiveFails,
					TotalRequests:    node.TotalRequests,
					TotalFailures:    node.TotalFailures,
				}
			}
			safeHealth[i] = SafeNetworkHealth{
				NetworkID: net.NetworkID,
				Nodes:     safeNodes,
			}
		}
		
		json.NewEncoder(w).Encode(safeHealth)
	})

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Info("gateway starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server failed", "err", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Info("shutting down gateway")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	srv.Shutdown(shutdownCtx)
}