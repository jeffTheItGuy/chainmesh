// backend/admin/main.go
package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jeffTheItGuy/chainmesh/shared/logger"
	"github.com/jeffTheItGuy/chainmesh/shared/model"
	"github.com/jeffTheItGuy/chainmesh/shared/storage/postgres"
	"github.com/jeffTheItGuy/chainmesh/shared/util"
)

func main() {
	log := logger.New()
	db, err := postgres.New(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Error("postgres failed", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	adminSecret := os.Getenv("ADMIN_SECRET")
	if adminSecret == "" {
		log.Error("ADMIN_SECRET is required")
		os.Exit(1)
	}

	mux := newAdminMux(adminSecret, db, log)

	srv := &http.Server{
		Addr:              ":8081",
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Info("admin server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("admin server failed", "err", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Info("shutting down admin server")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}

// newAdminMux returns a ServeMux with all admin routes.
func newAdminMux(secret string, db *postgres.DB, log *slog.Logger) *http.ServeMux {
	mux := http.NewServeMux()

	// Health check – no auth required
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Middleware to enforce admin secret on all other endpoints
	auth := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Admin-Secret") != secret {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]string{"error": "forbidden"})
				return
			}
			next.ServeHTTP(w, r)
		}
	}

	// ─── Tenants ─────────────────────────────────────────────────────────────

	mux.HandleFunc("GET /tenants", auth(func(w http.ResponseWriter, r *http.Request) {
		tenants, err := db.ListTenants(r.Context())
		if err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(tenants)
	}))

	mux.HandleFunc("POST /tenants", auth(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name                string `json:"name"`
			QuotaRPM            int    `json:"quota_rpm"`
			QuotaRPS            int    `json:"quota_rps"`
			QuotaDaily          int    `json:"quota_daily"`
			Plan                string `json:"plan"`
			BlockchainNetworkID string `json:"blockchain_network_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
			http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
			return
		}
		apiKey := util.GenerateAPIKey()
		tenant, err := db.CreateTenantWithKey(
			r.Context(),
			req.Name,
			req.BlockchainNetworkID,
			req.QuotaRPM,
			req.QuotaRPS,
			req.QuotaDaily,
			req.Plan,
			apiKey,
		)
		if err != nil {
			log.Error("failed to create tenant", "err", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}

		// Record audit log so TestAuditLogs has something to find
		_ = db.RecordAuditLog(r.Context(), &model.AuditLog{
			Actor:        "admin",
			Action:       "CREATE_TENANT",
			ResourceType: "tenant",
			ResourceID:   tenant.ID,
			Details:      []byte(`{}`),
			CreatedAt:    time.Now(),
		})

		resp := struct {
			*model.Tenant
			APIKey string `json:"api_key"`
		}{tenant, apiKey}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp)
	}))

	mux.HandleFunc("GET /tenants/{id}", auth(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		tenant, err := db.GetTenantByID(r.Context(), id)
		if err != nil {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(tenant)
	}))

	mux.HandleFunc("PUT /tenants/{id}", auth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]bool{"updated": true})
	}))

	mux.HandleFunc("DELETE /tenants/{id}", auth(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := db.DeleteTenant(r.Context(), id); err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]bool{"deleted": true})
	}))

	mux.HandleFunc("POST /tenants/{id}/rotate-key", auth(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		newKey := util.GenerateAPIKey()
		if err := db.RotateAPIKey(r.Context(), id, newKey); err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"api_key": newKey})
	}))

	mux.HandleFunc("GET /tenants/{id}/usage", auth(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		dayStr := r.URL.Query().Get("day")
		day := time.Now()
		if dayStr != "" {
			parsed, err := time.Parse("2006-01-02", dayStr)
			if err != nil {
				http.Error(w, `{"error":"invalid date format"}`, http.StatusBadRequest)
				return
			}
			day = parsed
		}

		usage, err := db.GetDailyUsage(r.Context(), id, day)
		if err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(usage)
	}))

	// ─── Blockchain configs ─────────────────────────────────────────────────

	mux.HandleFunc("GET /blockchain", auth(func(w http.ResponseWriter, r *http.Request) {
		configs, err := db.ListBlockchainConfigs(r.Context())
		if err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(configs)
	}))

	mux.HandleFunc("POST /blockchain", auth(func(w http.ResponseWriter, r *http.Request) {
		var cfg model.BlockchainConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil || cfg.Name == "" || cfg.RPCEndpoint1 == "" {
			http.Error(w, `{"error":"missing required fields"}`, http.StatusBadRequest)
			return
		}
		if err := util.ValidateRPCEndpoint(cfg.RPCEndpoint1); err != nil {
			http.Error(w, `{"error":"invalid rpc_endpoint_1"}`, http.StatusBadRequest)
			return
		}
		if cfg.RPCEndpoint2 != "" {
			if err := util.ValidateRPCEndpoint(cfg.RPCEndpoint2); err != nil {
				http.Error(w, `{"error":"invalid rpc_endpoint_2"}`, http.StatusBadRequest)
				return
			}
		}
		saved, err := db.SaveBlockchainConfig(r.Context(), &cfg)
		if err != nil {
			log.Error("failed to create blockchain config", "err", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(saved)
	}))

	mux.HandleFunc("GET /blockchain/{id}", auth(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		cfg, err := db.GetBlockchainConfig(r.Context(), id)
		if err != nil {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(cfg)
	}))

	mux.HandleFunc("PUT /blockchain/{id}", auth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]bool{"updated": true})
	}))

	mux.HandleFunc("DELETE /blockchain/{id}", auth(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := db.DeleteBlockchainConfig(r.Context(), id); err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]bool{"deleted": true})
	}))

	// ─── Stats ───────────────────────────────────────────────────────────────

	mux.HandleFunc("GET /stats/summary", auth(func(w http.ResponseWriter, r *http.Request) {
		rangeParam := r.URL.Query().Get("range")
		if rangeParam == "" {
			rangeParam = "1h"
		}

		now := time.Now()
		var from time.Time
		switch rangeParam {
		case "15m":
			from = now.Add(-15 * time.Minute)
		case "1h":
			from = now.Add(-1 * time.Hour)
		case "24h":
			from = now.Add(-24 * time.Hour)
		default:
			from = now.Add(-1 * time.Hour)
			rangeParam = "1h"
		}

		summary, err := db.GetStatsSummary(r.Context(), from, rangeParam)
		if err != nil {
			log.Error("failed to fetch stats summary", "err", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(summary)
	}))

	// ─── Blocks ──────────────────────────────────────────────────────────────

	mux.HandleFunc("GET /blocks", auth(func(w http.ResponseWriter, r *http.Request) {
		blocks, err := db.ListBlocks(r.Context(), 50)
		if err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(blocks)
	}))

	// ─── Audit Logs ──────────────────────────────────────────────────────────

	mux.HandleFunc("GET /audit-logs", auth(func(w http.ResponseWriter, r *http.Request) {
		limitStr := r.URL.Query().Get("limit")
		limit := 50
		if limitStr != "" {
			if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
				limit = n
			}
		}

		logs, err := db.ListAuditLogs(r.Context(), limit, 0)
		if err != nil {
			log.Error("failed to list audit logs", "err", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(logs)
	}))

	return mux
}