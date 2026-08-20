package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/yourname/blockmesh/shared/blockchain"
	"github.com/yourname/blockmesh/shared/logger"
	"github.com/yourname/blockmesh/shared/model"
	"github.com/yourname/blockmesh/shared/storage/postgres"
	"github.com/yourname/blockmesh/shared/util"
)

const maxAdminBodyBytes = 1 << 20 // 1 MB

func adminAuth(secret string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		provided := r.Header.Get("X-Admin-Secret")
		if !util.ConstantTimeEqual(provided, secret) {
			writeErr(w, http.StatusForbidden, "forbidden")
			return
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func main() {
	log := logger.New()
	adminSecret := os.Getenv("ADMIN_SECRET")
	if adminSecret == "" {
		log.Error("ADMIN_SECRET is not set - refusing to start with an unauthenticated admin API")
		os.Exit(1)
	}

	db, err := postgres.New(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Error("postgres failed", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/stats/summary", adminAuth(adminSecret, func(w http.ResponseWriter, r *http.Request) {
		rangeName := r.URL.Query().Get("range")
		if rangeName == "" {
			rangeName = "1h"
		}
		now := time.Now()
		var from time.Time
		switch rangeName {
		case "15m":
			from = now.Add(-15 * time.Minute)
		case "1h":
			from = now.Add(-1 * time.Hour)
		case "24h":
			from = now.Add(-24 * time.Hour)
		default:
			writeErr(w, http.StatusBadRequest, "range must be one of: 15m, 1h, 24h")
			return
		}

		summary, err := db.GetStatsSummary(r.Context(), from, rangeName)
		if err != nil {
			log.Error("stats query failed", "err", err)
			writeErr(w, http.StatusInternalServerError, "database error")
			return
		}
		writeJSON(w, http.StatusOK, summary)
	}))

	mux.HandleFunc("/tenants", adminAuth(adminSecret, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			tenants, err := db.ListTenants(r.Context())
			if err != nil {
				writeErr(w, http.StatusInternalServerError, "database error")
				return
			}
			writeJSON(w, http.StatusOK, tenants)
		case http.MethodPost:
			r.Body = http.MaxBytesReader(w, r.Body, maxAdminBodyBytes)
			var req struct {
				Name                string `json:"name"`
				QuotaRPM            int    `json:"quota_rpm"`
				QuotaRPS            int    `json:"quota_rps"`
				QuotaDaily          int    `json:"quota_daily"`
				Plan                string `json:"plan"`
				BlockchainNetworkID string `json:"blockchain_network_id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeErr(w, http.StatusBadRequest, "invalid json")
				return
			}
			if req.Name == "" {
				writeErr(w, http.StatusBadRequest, "name is required")
				return
			}
			if req.Plan == "" {
				req.Plan = "free"
			}
			if req.QuotaRPM <= 0 {
				req.QuotaRPM = 100
			}
			if req.QuotaRPS <= 0 {
				req.QuotaRPS = 10
			}
			if req.QuotaDaily <= 0 {
				req.QuotaDaily = 10000
			}
			if req.BlockchainNetworkID == "" {
				defaultCfg, err := db.GetDefaultBlockchainConfig(r.Context())
				if err != nil || defaultCfg == nil {
					writeErr(w, http.StatusBadRequest, "no blockchain network available — configure one first")
					return
				}
				req.BlockchainNetworkID = defaultCfg.ID
			}

			key := util.GenerateAPIKey()
			tenant, err := db.CreateTenantWithKey(
				r.Context(),
				req.Name,
				req.BlockchainNetworkID,
				req.QuotaRPM,
				req.QuotaRPS,
				req.QuotaDaily,
				req.Plan,
				key,
			)
			if err != nil {
				writeErr(w, http.StatusInternalServerError, "database error")
				return
			}
			writeJSON(w, http.StatusCreated, map[string]any{
				"id":                    tenant.ID,
				"name":                  tenant.Name,
				"api_key":               key,
				"quota_rpm":             tenant.QuotaRPM,
				"quota_rps":             tenant.QuotaRPS,
				"quota_daily":           tenant.QuotaDaily,
				"plan":                  tenant.Plan,
				"blockchain_network_id": tenant.BlockchainNetworkID,
				"created_at":            tenant.CreatedAt,
			})
		default:
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	}))

	mux.HandleFunc("/tenants/", adminAuth(adminSecret, func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/tenants/")
		parts := strings.Split(path, "/")
		if len(parts) == 0 || parts[0] == "" {
			writeErr(w, http.StatusBadRequest, "tenant id required")
			return
		}
		tenantID := parts[0]

		if len(parts) == 1 {
			switch r.Method {
			case http.MethodGet:
				tenant, err := db.GetTenantByID(r.Context(), tenantID)
				if err != nil {
					writeErr(w, http.StatusNotFound, "tenant not found")
					return
				}
				writeJSON(w, http.StatusOK, tenant)
			case http.MethodPut:
				r.Body = http.MaxBytesReader(w, r.Body, maxAdminBodyBytes)
				existing, err := db.GetTenantByID(r.Context(), tenantID)
				if err != nil {
					writeErr(w, http.StatusNotFound, "tenant not found")
					return
				}
				var req struct {
					Name                string `json:"name"`
					QuotaRPM            int    `json:"quota_rpm"`
					QuotaRPS            int    `json:"quota_rps"`
					QuotaDaily          int    `json:"quota_daily"`
					Plan                string `json:"plan"`
					BlockchainNetworkID string `json:"blockchain_network_id"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					writeErr(w, http.StatusBadRequest, "invalid json")
					return
				}
				if req.Name == "" {
					req.Name = existing.Name
				}
				if req.QuotaRPM <= 0 {
					req.QuotaRPM = existing.QuotaRPM
				}
				if req.QuotaRPS <= 0 {
					req.QuotaRPS = existing.QuotaRPS
				}
				if req.QuotaDaily <= 0 {
					req.QuotaDaily = existing.QuotaDaily
				}
				if req.Plan == "" {
					req.Plan = existing.Plan
				}
				if req.BlockchainNetworkID == "" {
					req.BlockchainNetworkID = existing.BlockchainNetworkID
				}

				err = db.UpdateTenant(
					r.Context(),
					tenantID,
					req.Name,
					req.BlockchainNetworkID,
					req.QuotaRPM,
					req.QuotaRPS,
					req.QuotaDaily,
					req.Plan,
				)
				if err != nil {
					writeErr(w, http.StatusInternalServerError, "database error")
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{"updated": true})
			case http.MethodDelete:
				err := db.DeleteTenant(r.Context(), tenantID)
				if err != nil {
					writeErr(w, http.StatusInternalServerError, "database error")
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
			default:
				writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "rotate-key" {
			if r.Method != http.MethodPost {
				writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			key := util.GenerateAPIKey()
			err := db.RotateAPIKey(r.Context(), tenantID, key)
			if err != nil {
				writeErr(w, http.StatusInternalServerError, "database error")
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"api_key": key,
			})
			return
		}

		// NEW: Admin usage lookup by tenant ID
		if len(parts) == 2 && parts[1] == "usage" {
			if r.Method != http.MethodGet {
				writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			dayStr := r.URL.Query().Get("day")
			day := time.Now()
			if dayStr != "" {
				var err error
				day, err = time.Parse("2006-01-02", dayStr)
				if err != nil {
					writeErr(w, http.StatusBadRequest, "invalid day format, use YYYY-MM-DD")
					return
				}
			}

			// Verify tenant exists
			_, err := db.GetTenantByID(r.Context(), tenantID)
			if err != nil {
				writeErr(w, http.StatusNotFound, "tenant not found")
				return
			}

			usage, err := db.GetDailyUsage(r.Context(), tenantID, day)
			if err != nil {
				writeErr(w, http.StatusInternalServerError, "database error")
				return
			}
			writeJSON(w, http.StatusOK, usage)
			return
		}

		writeErr(w, http.StatusNotFound, "not found")
	}))

	// REMOVED: The old /usage endpoint that required the API key in the query string

	mux.HandleFunc("/blocks", func(w http.ResponseWriter, r *http.Request) {
		blocks, err := db.ListBlocks(r.Context(), 50)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "database error")
			return
		}
		writeJSON(w, http.StatusOK, blocks)
	})

	mux.HandleFunc("/blockchain/test", adminAuth(adminSecret, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxAdminBodyBytes)
		var req struct {
			RPCEndpoint1 string `json:"rpc_endpoint_1"`
			RPCEndpoint2 string `json:"rpc_endpoint_2"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid json")
			return
		}
		if req.RPCEndpoint1 == "" {
			writeErr(w, http.StatusBadRequest, "rpc_endpoint_1 is required")
			return
		}

		endpoints := []string{req.RPCEndpoint1}
		if req.RPCEndpoint2 != "" {
			endpoints = append(endpoints, req.RPCEndpoint2)
		}

		testClient := blockchain.New(endpoints)
		resp, err := testClient.Call(r.Context(), "eth_chainId")
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"connected": false,
				"error":     err.Error(),
			})
			return
		}

		var rpcResp blockchain.RPCResponse
		if err := json.Unmarshal(resp, &rpcResp); err != nil || rpcResp.Error != nil {
			msg := "invalid rpc response"
			if rpcResp.Error != nil {
				msg = rpcResp.Error.Message
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"connected": false,
				"error":     msg,
			})
			return
		}

		chainIDHex := strings.Trim(string(rpcResp.Result), `"`)
		chainIDDec, _ := strconv.ParseInt(chainIDHex, 0, 64)

		writeJSON(w, http.StatusOK, map[string]any{
			"connected": true,
			"chain_id":  strconv.FormatInt(chainIDDec, 10),
		})
	}))

	mux.HandleFunc("/blockchain", adminAuth(adminSecret, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			configs, err := db.ListBlockchainConfigs(r.Context())
			if err != nil {
				writeErr(w, http.StatusInternalServerError, "database error")
				return
			}
			writeJSON(w, http.StatusOK, configs)
		case http.MethodPost:
			r.Body = http.MaxBytesReader(w, r.Body, maxAdminBodyBytes)
			var req struct {
				Name         string `json:"name"`
				RPCEndpoint1 string `json:"rpc_endpoint_1"`
				RPCEndpoint2 string `json:"rpc_endpoint_2"`
				ChainID      string `json:"chain_id"`
				Enabled      bool   `json:"enabled"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeErr(w, http.StatusBadRequest, "invalid json")
				return
			}
			if req.Name == "" || req.RPCEndpoint1 == "" {
				writeErr(w, http.StatusBadRequest, "name and rpc_endpoint_1 are required")
				return
			}
			cfg := &model.BlockchainConfig{
				Name:         req.Name,
				RPCEndpoint1: req.RPCEndpoint1,
				RPCEndpoint2: req.RPCEndpoint2,
				ChainID:      req.ChainID,
				Enabled:      req.Enabled,
			}
			saved, err := db.SaveBlockchainConfig(r.Context(), cfg)
			if err != nil {
				writeErr(w, http.StatusInternalServerError, "database error")
				return
			}
			writeJSON(w, http.StatusCreated, saved)
		default:
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	}))

	mux.HandleFunc("/blockchain/", adminAuth(adminSecret, func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/blockchain/")
		if id == "" {
			writeErr(w, http.StatusBadRequest, "network id required")
			return
		}
		switch r.Method {
		case http.MethodGet:
			cfg, err := db.GetBlockchainConfig(r.Context(), id)
			if err != nil {
				writeErr(w, http.StatusNotFound, "not found")
				return
			}
			writeJSON(w, http.StatusOK, cfg)
		case http.MethodPut:
			r.Body = http.MaxBytesReader(w, r.Body, maxAdminBodyBytes)
			var req struct {
				Name         string `json:"name"`
				RPCEndpoint1 string `json:"rpc_endpoint_1"`
				RPCEndpoint2 string `json:"rpc_endpoint_2"`
				ChainID      string `json:"chain_id"`
				Enabled      bool   `json:"enabled"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeErr(w, http.StatusBadRequest, "invalid json")
				return
			}
			if req.Name == "" || req.RPCEndpoint1 == "" {
				writeErr(w, http.StatusBadRequest, "name and rpc_endpoint_1 are required")
				return
			}
			cfg := &model.BlockchainConfig{
				ID:           id,
				Name:         req.Name,
				RPCEndpoint1: req.RPCEndpoint1,
				RPCEndpoint2: req.RPCEndpoint2,
				ChainID:      req.ChainID,
				Enabled:      req.Enabled,
			}
			if err := db.UpdateBlockchainConfig(r.Context(), cfg); err != nil {
				writeErr(w, http.StatusInternalServerError, "database error")
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"updated": true})
		case http.MethodDelete:
			if err := db.DeleteBlockchainConfig(r.Context(), id); err != nil {
				writeErr(w, http.StatusInternalServerError, "database error")
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
		default:
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	}))

	// Public proxy: forward node health from gateway so unauthenticated viewers can see it
	mux.HandleFunc("/gateway/health/nodes", func(w http.ResponseWriter, r *http.Request) {
		gatewayAddr := os.Getenv("GATEWAY_ADDR")
		if gatewayAddr == "" {
			gatewayAddr = "http://localhost:8080"
		}

		resp, err := http.Get(gatewayAddr + "/health/nodes")
		if err != nil {
			writeErr(w, http.StatusBadGateway, "gateway unreachable")
			return
		}
		defer resp.Body.Close()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	})

	srv := &http.Server{
		Addr:              ":8081",
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

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