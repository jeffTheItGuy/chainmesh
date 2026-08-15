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
	"github.com/yourname/blockmesh/shared/model"
	"github.com/yourname/blockmesh/shared/storage/postgres"
	"github.com/yourname/blockmesh/shared/util"
)

func adminAuth(next http.HandlerFunc) http.HandlerFunc {
	secret := os.Getenv("ADMIN_SECRET")
	if secret == "" {
		return next
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Admin-Secret") != secret {
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

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
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/tenants", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			tenants, err := db.ListTenants(r.Context())
			if err != nil {
				http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(tenants)
		case http.MethodPost:
			var req struct {
				Name     string `json:"name"`
				QuotaRPM int    `json:"quota_rpm"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
				return
			}
			if req.Name == "" {
				http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
				return
			}
			if req.QuotaRPM <= 0 {
				req.QuotaRPM = 60
			}
			key := util.GenerateAPIKey()
			tenant, err := db.CreateTenant(r.Context(), req.Name, key, req.QuotaRPM)
			if err != nil {
				http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
				return
			}
			// Return the key once so it can be copied
			json.NewEncoder(w).Encode(map[string]any{
				"id":        tenant.ID,
				"name":      tenant.Name,
				"api_key":   key,
				"quota_rpm": tenant.QuotaRPM,
				"created_at": tenant.CreatedAt,
			})
		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	}))
	mux.HandleFunc("/usage", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		dayStr := r.URL.Query().Get("day")
		day := time.Now()
		if dayStr != "" {
			var err error
			day, err = time.Parse("2006-01-02", dayStr)
			if err != nil {
				http.Error(w, `{"error":"invalid day format, use YYYY-MM-DD"}`, http.StatusBadRequest)
				return
			}
		}

		var tenantID string
		if apiKey := r.URL.Query().Get("api_key"); apiKey != "" {
			t, err := db.GetTenantByAPIKey(r.Context(), apiKey)
			if err != nil {
				http.Error(w, `{"error":"invalid api_key"}`, http.StatusBadRequest)
				return
			}
			tenantID = t.ID
		} else if id := r.URL.Query().Get("tenant"); id != "" {
			tenantID = id
		} else {
			http.Error(w, `{"error":"tenant or api_key required"}`, http.StatusBadRequest)
			return
		}

		usage, err := db.GetDailyUsage(r.Context(), tenantID, day)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(usage)
	})
	mux.HandleFunc("/blocks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		blocks, err := db.ListBlocks(r.Context(), 50)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(blocks)
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
