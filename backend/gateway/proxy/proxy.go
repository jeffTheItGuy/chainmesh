package proxy

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

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

	resp, err := p.bc.Call(r.Context(), req.Method, req.Params...)
	if err != nil {
		p.log.Error("upstream failed", "err", err)
		http.Error(w, `{"error":"upstream unavailable"}`, http.StatusBadGateway)
		return
	}

	go p.db.RecordUsage(r.Context(), &model.Usage{
		TenantID: tenant.ID,
		Method:   req.Method,
		Count:    1,
		BytesIn:  int64(len(body)),
	})

	w.Header().Set("Content-Type", "application/json")
	w.Write(resp)
}
