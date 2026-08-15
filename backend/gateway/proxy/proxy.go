package proxy

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/yourname/blockmesh/gateway/middleware"
	"github.com/yourname/blockmesh/shared/blockchain"
	"github.com/yourname/blockmesh/shared/model"
	"github.com/yourname/blockmesh/shared/storage/postgres"
	"github.com/yourname/blockmesh/shared/storage/redis"
)

type Proxy struct {
	bc    *blockchain.Client
	db    *postgres.DB
	cache *redis.Client
	log   *slog.Logger
}

func New(bc *blockchain.Client, db *postgres.DB, cache *redis.Client, log *slog.Logger) *Proxy {
	return &Proxy{bc: bc, db: db, cache: cache, log: log}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.TenantFromContext(r.Context())
	if tenant == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	body, _ := io.ReadAll(r.Body)
	defer r.Body.Close()

	var req blockchain.RPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	cacheKey := "rpc:" + req.Method
	if len(req.Params) > 0 {
		paramsJSON, _ := json.Marshal(req.Params)
		cacheKey += ":" + string(paramsJSON)
	}

	if req.Method == "eth_chainId" || req.Method == "eth_blockNumber" || req.Method == "eth_getBalance" {
		if cached, err := p.cache.Get(r.Context(), cacheKey); err == nil && cached != "" {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Cache", "HIT")
			w.Write([]byte(cached))
			go p.recordUsage(r.Context(), tenant, req.Method, int64(len(body)))
			return
		}
	}

	resp, err := p.bc.Call(r.Context(), req.Method, req.Params...)
	if err != nil {
		p.log.Error("upstream failed", "err", err)
		http.Error(w, `{"error":"upstream unavailable"}`, http.StatusBadGateway)
		return
	}

	var rpcResp blockchain.RPCResponse
	if err := json.Unmarshal(resp, &rpcResp); err == nil && rpcResp.Error != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(resp)
		go p.recordUsage(r.Context(), tenant, req.Method, int64(len(body)))
		return
	}

	if req.Method == "eth_chainId" {
		p.cache.Set(r.Context(), cacheKey, string(resp), 24*time.Hour)
	} else if req.Method == "eth_blockNumber" {
		p.cache.Set(r.Context(), cacheKey, string(resp), 2*time.Second)
	} else if req.Method == "eth_getBalance" {
		p.cache.Set(r.Context(), cacheKey, string(resp), 30*time.Second)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.Write(resp)

	go p.recordUsage(r.Context(), tenant, req.Method, int64(len(body)))
}

func (p *Proxy) recordUsage(ctx context.Context, tenant *model.Tenant, method string, bytesIn int64) {
	err := p.db.RecordUsage(ctx, &model.Usage{
		TenantID: tenant.ID,
		Method:   method,
		Count:    1,
		BytesIn:  bytesIn,
		Period:   time.Now().Truncate(time.Minute),
	})
	if err != nil {
		p.log.Error("usage recording failed", "err", err)
	}
}
