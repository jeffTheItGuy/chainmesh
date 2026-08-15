package blockchain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/yourname/blockmesh/shared/logger"
)

type Client struct {
	endpoints []string
	http      *http.Client
	log       *slog.Logger
}

func New(endpoints []string) *Client {
	return &Client{
		endpoints: endpoints,
		http:      &http.Client{Timeout: 10 * time.Second},
		log:       logger.New(),
	}
}

func (c *Client) Call(ctx context.Context, method string, params ...any) (json.RawMessage, error) {
	reqBody, _ := json.Marshal(RPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	})

	for _, ep := range c.endpoints {
		req, _ := http.NewRequestWithContext(ctx, "POST", ep, bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.http.Do(req)
		if err != nil {
			c.log.Warn("rpc failed", "endpoint", ep, "err", err)
			continue
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		return body, nil
	}
	return nil, fmt.Errorf("all endpoints failed")
}
