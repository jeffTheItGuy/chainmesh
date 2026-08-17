package blockchain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/yourname/blockmesh/shared/logger"
	"github.com/yourname/blockmesh/shared/metrics"
	"github.com/yourname/blockmesh/shared/requestid"
)

// EndpointHealth holds real-time health and latency data for a single RPC endpoint.
type EndpointHealth struct {
	URL              string        `json:"url"`
	Healthy          bool          `json:"healthy"`
	Latency          time.Duration `json:"latency"`
	LastCheck        time.Time     `json:"last_check"`
	ConsecutiveFails int           `json:"consecutive_fails"`
	TotalRequests    int64         `json:"total_requests"`
	TotalFailures    int64         `json:"total_failures"`
}

// Client is a health-aware JSON-RPC client that supports multiple upstream
// endpoints, periodic health checks, automatic failover, and fastest-node
// selection.
type Client struct {
	endpoints     []string
	http          *http.Client
	log           *slog.Logger
	networkID     string
	healthMu      sync.RWMutex
	health        map[string]*EndpointHealth
	checkInterval time.Duration
	checkCancel   context.CancelFunc
	checkWg       sync.WaitGroup
}

// New creates a new blockchain RPC client.
func New(endpoints []string) *Client {
	health := make(map[string]*EndpointHealth, len(endpoints))
	for _, ep := range endpoints {
		health[ep] = &EndpointHealth{URL: ep, Healthy: true}
	}
	return &Client{
		endpoints:     endpoints,
		http:          &http.Client{Timeout: 10 * time.Second},
		log:           logger.New(),
		health:        health,
		checkInterval: 10 * time.Second,
	}
}

// SetNetworkID sets the network label used in metrics and logs.
func (c *Client) SetNetworkID(networkID string) {
	c.networkID = networkID
}

// StartHealthChecks begins a background goroutine that probes every endpoint
// at the given interval.
func (c *Client) StartHealthChecks(ctx context.Context, interval time.Duration) {
	c.checkInterval = interval
	ctx, cancel := context.WithCancel(ctx)
	c.checkCancel = cancel
	c.checkWg.Add(1)
	go func() {
		defer c.checkWg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		// Run an immediate check so we don't wait for the first tick.
		c.runHealthCheck(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.runHealthCheck(ctx)
			}
		}
	}()
}

// StopHealthChecks gracefully stops the background health-check goroutine.
func (c *Client) StopHealthChecks() {
	if c.checkCancel != nil {
		c.checkCancel()
		c.checkWg.Wait()
	}
}

// HealthyEndpoints returns a snapshot of the current health state for all
// configured endpoints.
func (c *Client) HealthyEndpoints() []EndpointHealth {
	c.healthMu.RLock()
	defer c.healthMu.RUnlock()
	out := make([]EndpointHealth, 0, len(c.health))
	for _, h := range c.health {
		out = append(out, *h)
	}
	return out
}

// runHealthCheck probes every endpoint concurrently with eth_chainId.
func (c *Client) runHealthCheck(ctx context.Context) {
	reqBody, _ := json.Marshal(RPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_chainId",
		Params:  []any{},
		ID:      1,
	})
	var wg sync.WaitGroup
	for _, ep := range c.endpoints {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			start := time.Now()
			req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			// FIX: Add User-Agent to prevent WAF/Cloudflare blocks on public RPCs
			req.Header.Set("User-Agent", "BlockMesh-Gateway/1.0")

			resp, err := c.http.Do(req)
			latency := time.Since(start)

			c.healthMu.Lock()
			h := c.health[url]
			if h == nil {
				h = &EndpointHealth{URL: url}
				c.health[url] = h
			}
			h.LastCheck = start
			h.Latency = latency

			if err != nil {
				h.ConsecutiveFails++
				if h.ConsecutiveFails >= 3 {
					h.Healthy = false
				}
				healthy := h.Healthy
				c.healthMu.Unlock()
				metrics.NodeHealthy.WithLabelValues(c.networkID, url).Set(boolToFloat(healthy))
				c.log.Warn(
					"health check failed",
					"network_id", c.networkID,
					"endpoint", url,
					"latency", latency,
					"err", err,
					"consecutive_fails", h.ConsecutiveFails,
				)
				return
			}

			// Drain body so the connection can be reused.
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()

			h.ConsecutiveFails = 0
			h.Healthy = true
			healthy := h.Healthy
			c.healthMu.Unlock()
			metrics.NodeHealthy.WithLabelValues(c.networkID, url).Set(boolToFloat(healthy))
		}(ep)
	}
	wg.Wait()
}

// Call executes the JSON-RPC method against the best available endpoint.
func (c *Client) Call(ctx context.Context, method string, params ...any) (json.RawMessage, error) {
	requestID := requestid.FromContext(ctx)
	log := c.log.With(
		"request_id", requestID,
		"network_id", c.networkID,
		"method", method,
	)
	reqBody, _ := json.Marshal(RPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	})
	endpoints := c.endpointsForCall()
	for _, ep := range endpoints {
		start := time.Now()
		req, _ := http.NewRequestWithContext(ctx, "POST", ep, bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		// FIX: Add User-Agent to prevent WAF/Cloudflare blocks on public RPCs
		req.Header.Set("User-Agent", "BlockMesh-Gateway/1.0")

		if requestID != "" {
			req.Header.Set("X-Request-ID", requestID)
		}

		resp, err := c.http.Do(req)
		latency := time.Since(start)

		if err != nil {
			metrics.UpstreamErrorsTotal.WithLabelValues(
				c.networkID,
				ep,
				method,
				"transport",
			).Inc()
			log.Warn(
				"rpc call failed",
				"endpoint", ep,
				"err", err,
			)
			c.markFailure(ep)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var rpcResp RPCResponse
		if err := json.Unmarshal(body, &rpcResp); err != nil {
			metrics.UpstreamErrorsTotal.WithLabelValues(
				c.networkID,
				ep,
				method,
				"invalid_response",
			).Inc()
			log.Warn(
				"rpc invalid response",
				"endpoint", ep,
				"err", err,
			)
			c.markFailure(ep)
			continue
		}

		metrics.UpstreamRequestDurationSeconds.WithLabelValues(
			c.networkID,
			ep,
			method,
		).Observe(latency.Seconds())

		// An RPC-level error is still a valid node response.
		// The node is alive, so we mark success but label the upstream status.
		if rpcResp.Error != nil {
			metrics.UpstreamRequestsTotal.WithLabelValues(
				c.networkID,
				ep,
				method,
				"rpc_error",
			).Inc()
			c.markSuccess(ep, latency)
			return body, nil
		}

		metrics.UpstreamRequestsTotal.WithLabelValues(
			c.networkID,
			ep,
			method,
			"success",
		).Inc()
		c.markSuccess(ep, latency)
		return body, nil
	}
	return nil, fmt.Errorf("all endpoints failed")
}

// endpointsForCall returns the endpoint list ordered for optimal routing:
// 1. Healthy endpoints sorted by latency ascending.
// 2. Unhealthy endpoints as last-resort fallback.
func (c *Client) endpointsForCall() []string {
	c.healthMu.RLock()
	defer c.healthMu.RUnlock()
	type scored struct {
		url     string
		healthy bool
		latency time.Duration
	}
	scoredList := make([]scored, 0, len(c.endpoints))
	for _, ep := range c.endpoints {
		h, ok := c.health[ep]
		if !ok {
			scoredList = append(scoredList, scored{url: ep, healthy: true, latency: 0})
			continue
		}
		scoredList = append(scoredList, scored{
			url:     ep,
			healthy: h.Healthy,
			latency: h.Latency,
		})
	}
	healthy := make([]scored, 0)
	unhealthy := make([]scored, 0)
	for _, s := range scoredList {
		if s.healthy {
			healthy = append(healthy, s)
		} else {
			unhealthy = append(unhealthy, s)
		}
	}
	sort.Slice(healthy, func(i, j int) bool {
		return healthy[i].latency < healthy[j].latency
	})
	result := make([]string, 0, len(c.endpoints))
	for _, s := range healthy {
		result = append(result, s.url)
	}
	for _, s := range unhealthy {
		result = append(result, s.url)
	}
	return result
}

func (c *Client) markSuccess(url string, latency time.Duration) {
	c.healthMu.Lock()
	defer c.healthMu.Unlock()
	h, ok := c.health[url]
	if !ok {
		h = &EndpointHealth{URL: url}
		c.health[url] = h
	}
	h.Healthy = true
	h.ConsecutiveFails = 0
	h.Latency = latency
	h.TotalRequests++
	h.LastCheck = time.Now()
	metrics.NodeHealthy.WithLabelValues(c.networkID, url).Set(1)
}

func (c *Client) markFailure(url string) {
	c.healthMu.Lock()
	defer c.healthMu.Unlock()
	h, ok := c.health[url]
	if !ok {
		h = &EndpointHealth{URL: url}
		c.health[url] = h
	}
	h.ConsecutiveFails++
	h.TotalFailures++
	if h.ConsecutiveFails >= 3 {
		h.Healthy = false
	}
	h.LastCheck = time.Now()
	metrics.NodeHealthy.WithLabelValues(c.networkID, url).Set(boolToFloat(h.Healthy))
}

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}