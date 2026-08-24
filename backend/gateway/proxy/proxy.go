package proxy

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jeffTheItGuy/chainmesh/gateway/middleware"
	"github.com/jeffTheItGuy/chainmesh/shared/blockchain"
	"github.com/jeffTheItGuy/chainmesh/shared/metrics"
	"github.com/jeffTheItGuy/chainmesh/shared/model"
	"github.com/jeffTheItGuy/chainmesh/shared/requestid"
	"github.com/jeffTheItGuy/chainmesh/shared/storage/postgres"
	"github.com/jeffTheItGuy/chainmesh/shared/storage/redis"
	"github.com/jeffTheItGuy/chainmesh/shared/telemetry"
)

const maxBodyBytes = 2 << 20 // 2 MB

type NetworkManager interface {
	Get(networkID string) (*blockchain.Client, bool)
}

var cachePolicies = map[string]time.Duration{
	"eth_chainId":              24 * time.Hour,
	"eth_blockNumber":          2 * time.Second,
	"eth_getBalance":           30 * time.Second,
	"eth_gasPrice":             15 * time.Second,
	"eth_maxPriorityFeePerGas": 15 * time.Second,
}

type Proxy struct {
	manager   NetworkManager
	db        *postgres.DB
	cache     *redis.Client
	log       *slog.Logger
	telemetry *telemetry.Recorder
}

func New(
	manager NetworkManager,
	db *postgres.DB,
	cache *redis.Client,
	log *slog.Logger,
	telemetryRecorder *telemetry.Recorder,
) *Proxy {
	return &Proxy{
		manager:   manager,
		db:        db,
		cache:     cache,
		log:       log,
		telemetry: telemetryRecorder,
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestID := requestid.FromContext(r.Context())
	log := p.log.With("request_id", requestID)

	tenant := middleware.TenantFromContext(r.Context())
	if tenant == nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	body, err := io.ReadAll(r.Body)
	defer r.Body.Close()

	if err != nil {
		p.recordOutcome(tenant, requestID, "", "", "invalid_request", start, false, 0)
		writeJSONError(w, http.StatusRequestEntityTooLarge, "request body too large")
		return
	}

	bytesIn := int64(len(body))

	var req blockchain.RPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		p.recordOutcome(tenant, requestID, "", "", "invalid_request", start, false, bytesIn)
		writeJSONError(w, http.StatusBadRequest, "invalid json")
		return
	}

	req.Method = strings.TrimSpace(req.Method)
	if req.JSONRPC != "2.0" || req.Method == "" {
		p.recordOutcome(tenant, requestID, "", "", "invalid_request", start, false, bytesIn)
		writeJSONError(w, http.StatusBadRequest, "invalid json-rpc request")
		return
	}

	networkID := tenant.BlockchainNetworkID
	if networkID == "" {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defaultCfg, err := p.db.GetDefaultBlockchainConfig(ctx)
		cancel()

		if err != nil || defaultCfg == nil {
			p.recordOutcome(tenant, requestID, "", req.Method, "network_unavailable", start, false, bytesIn)
			writeJSONError(w, http.StatusInternalServerError, "no blockchain network configured")
			return
		}

		networkID = defaultCfg.ID
	}

	bc, ok := p.manager.Get(networkID)
	if !ok {
		p.recordOutcome(tenant, requestID, networkID, req.Method, "network_unavailable", start, false, bytesIn)
		writeJSONError(w, http.StatusServiceUnavailable, "blockchain network unavailable")
		return
	}

	cacheKey := "rpc:" + networkID + ":" + req.Method
	if len(req.Params) > 0 {
		paramsJSON, _ := json.Marshal(req.Params)
		cacheKey += ":" + string(paramsJSON)
	}

	if _, cacheable := cachePolicies[req.Method]; cacheable {
		cached, err := p.cache.Get(r.Context(), cacheKey)
		if err == nil && cached != "" {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Cache", "HIT")
			w.Write([]byte(cached))

			p.recordOutcome(tenant, requestID, networkID, req.Method, "success", start, true, bytesIn)
			return
		}
	}

	resp, err := bc.Call(r.Context(), req.Method, req.Params...)
	if err != nil {
		log.Error(
			"upstream failed",
			"network_id", networkID,
			"method", req.Method,
			"err", err,
		)

		p.recordOutcome(tenant, requestID, networkID, req.Method, "upstream_error", start, false, bytesIn)
		writeJSONError(w, http.StatusBadGateway, "upstream unavailable")
		return
	}

	var rpcResp blockchain.RPCResponse
	if err := json.Unmarshal(resp, &rpcResp); err == nil && rpcResp.Error != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(resp)

		p.recordOutcome(tenant, requestID, networkID, req.Method, "rpc_error", start, false, bytesIn)
		return
	}

	if ttl, ok := cachePolicies[req.Method]; ok {
		p.cache.Set(r.Context(), cacheKey, string(resp), ttl)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.Write(resp)

	p.recordOutcome(tenant, requestID, networkID, req.Method, "success", start, false, bytesIn)
}

func (p *Proxy) recordOutcome(
	tenant *model.Tenant,
	requestID string,
	networkID string,
	method string,
	status string,
	start time.Time,
	cacheHit bool,
	bytesIn int64,
) {
	latency := time.Since(start)

	methodLabel := methodLabel(method)
	cacheLabel := boolLabel(cacheHit)

	metrics.RequestDurationSeconds.WithLabelValues(
		networkID,
		methodLabel,
		status,
		cacheLabel,
	).Observe(latency.Seconds())

	metrics.RequestsTotal.WithLabelValues(
		networkID,
		methodLabel,
		status,
		cacheLabel,
	).Inc()

	if cacheHit {
		metrics.CacheHitsTotal.WithLabelValues(networkID, methodLabel).Inc()
	} else if _, cacheable := cachePolicies[method]; cacheable && status == "success" {
		metrics.CacheMissesTotal.WithLabelValues(networkID, methodLabel).Inc()
	}

	if p.telemetry != nil {
		if method != "" {
			p.telemetry.RecordUsage(&model.Usage{
				TenantID: tenant.ID,
				Method:   method,
				Count:    1,
				BytesIn:  bytesIn,
				Period:   start.Truncate(time.Minute),
			})
		}

		p.telemetry.RecordRequestLog(&model.RequestLog{
			TenantID:  tenant.ID,
			NetworkID: networkID,
			Method:    method,
			Status:    status,
			LatencyMS: latency.Milliseconds(),
			CacheHit:  cacheHit,
			BytesIn:   bytesIn,
			RequestID: requestID,
		})

		return
	}

	// Fallback for nil telemetry recorder.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if method != "" {
			err := p.db.RecordUsage(ctx, &model.Usage{
				TenantID: tenant.ID,
				Method:   method,
				Count:    1,
				BytesIn:  bytesIn,
				Period:   start.Truncate(time.Minute),
			})
			if err != nil {
				p.log.Error("usage recording failed", "err", err)
			}
		}

		err := p.db.RecordRequestLog(ctx, &model.RequestLog{
			TenantID:  tenant.ID,
			NetworkID: networkID,
			Method:    method,
			Status:    status,
			LatencyMS: latency.Milliseconds(),
			CacheHit:  cacheHit,
			BytesIn:   bytesIn,
			RequestID: requestID,
		})
		if err != nil {
			p.log.Error("request log recording failed", "err", err)
		}
	}()
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func methodLabel(method string) string {
	if method == "" {
		return "unknown"
	}
	return method
}

func boolLabel(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
