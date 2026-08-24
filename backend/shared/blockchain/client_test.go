package blockchain

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/jeffTheItGuy/chainmesh/shared/requestid"
)

func TestEndpointsForCall_OrdersByLatencyAndHealth(t *testing.T) {
	c := New([]string{"http://slow", "http://fast", "http://dead"})

	c.markSuccess("http://fast", 5*time.Millisecond)
	c.markSuccess("http://slow", 50*time.Millisecond)
	c.markFailure("http://dead")
	c.markFailure("http://dead")
	c.markFailure("http://dead")

	eps := c.endpointsForCall()

	require.Len(t, eps, 3)
	assert.Equal(t, "http://fast", eps[0])
	assert.Equal(t, "http://slow", eps[1])
	assert.Equal(t, "http://dead", eps[2])
}

func TestHealthCheckStateMachine(t *testing.T) {
	c := New([]string{"http://node"})

	c.markSuccess("http://node", 10*time.Millisecond)
	assert.True(t, c.health["http://node"].Healthy)

	c.markFailure("http://node")
	assert.True(t, c.health["http://node"].Healthy, "1 fail still healthy")

	c.markFailure("http://node")
	assert.True(t, c.health["http://node"].Healthy, "2 fails still healthy")

	c.markFailure("http://node")
	assert.False(t, c.health["http://node"].Healthy, "3 fails becomes unhealthy")

	c.markSuccess("http://node", 10*time.Millisecond)
	assert.True(t, c.health["http://node"].Healthy, "success resets immediately")
	assert.Equal(t, 0, c.health["http://node"].ConsecutiveFails)
}

func TestCall_FailoverOnBadResponse(t *testing.T) {
	calls := 0
	badSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("bad gateway"))
	}))
	defer badSrv.Close()

	goodSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		json.NewEncoder(w).Encode(RPCResponse{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`"0x1"`),
			ID:      1,
		})
	}))
	defer goodSrv.Close()

	c := New([]string{badSrv.URL, goodSrv.URL})

	resp, err := c.Call(context.Background(), "eth_chainId")
	require.NoError(t, err)
	assert.Equal(t, json.RawMessage(`{"jsonrpc":"2.0","result":"0x1","id":1}`+"\n"), resp)
	assert.Equal(t, 2, calls, "should have tried bad then good")
}

func TestCall_RPCErrorReturned(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(RPCResponse{
			JSONRPC: "2.0",
			Error:   &RPCError{Code: -32000, Message: "execution reverted"},
			ID:      1,
		})
	}))
	defer srv.Close()

	c := New([]string{srv.URL})

	resp, err := c.Call(context.Background(), "eth_call")
	require.NoError(t, err, "RPC-level errors should not return Go error")

	var rpcResp RPCResponse
	require.NoError(t, json.Unmarshal(resp, &rpcResp))
	require.NotNil(t, rpcResp.Error)
	assert.Equal(t, "execution reverted", rpcResp.Error.Message)
}

func TestCall_AllEndpointsFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := New([]string{srv.URL})
	c.markFailure(srv.URL)
	c.markFailure(srv.URL)
	c.markFailure(srv.URL)

	_, err := c.Call(context.Background(), "eth_chainId")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all endpoints failed")
}

// ---------------------------------------------------------------------------
// New coverage tests
// ---------------------------------------------------------------------------

func TestClient_HealthCheckLifecycle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(RPCResponse{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`"0x1"`),
			ID:      1,
		})
	}))
	defer srv.Close()

	c := New([]string{srv.URL})
	ctx, cancel := context.WithCancel(context.Background())
	c.StartHealthChecks(ctx, 100*time.Millisecond)
	time.Sleep(150 * time.Millisecond)

	health := c.HealthyEndpoints()
	require.Len(t, health, 1)
	assert.True(t, health[0].Healthy)

	cancel()
	c.StopHealthChecks()
}

func TestClient_HealthCheckNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := New([]string{srv.URL})
	c.StartHealthChecks(context.Background(), 50*time.Millisecond)
	defer c.StopHealthChecks()

	time.Sleep(200 * time.Millisecond)

	health := c.HealthyEndpoints()
	require.Len(t, health, 1)
	assert.False(t, health[0].Healthy)
}

func TestClient_HealthCheckInvalidJSONRPC(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	c := New([]string{srv.URL})
	c.StartHealthChecks(context.Background(), 50*time.Millisecond)
	defer c.StopHealthChecks()

	time.Sleep(200 * time.Millisecond)

	health := c.HealthyEndpoints()
	require.Len(t, health, 1)
	assert.False(t, health[0].Healthy)
}

func TestClient_Call_RequestIDPropagation(t *testing.T) {
	var receivedRequestID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedRequestID = r.Header.Get("X-Request-ID")
		json.NewEncoder(w).Encode(RPCResponse{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`"0x1"`),
			ID:      1,
		})
	}))
	defer srv.Close()

	c := New([]string{srv.URL})
	ctx := requestid.NewContext(context.Background(), "trace-123")
	_, err := c.Call(ctx, "eth_chainId")
	require.NoError(t, err)
	assert.Equal(t, "trace-123", receivedRequestID)
}

func TestClient_ConcurrentMarkFailureSuccess(t *testing.T) {
	c := New([]string{"http://node"})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			c.markSuccess("http://node", 10*time.Millisecond)
		}()
		go func() {
			defer wg.Done()
			c.markFailure("http://node")
		}()
	}
	wg.Wait()

	// After mixed concurrent calls, health state should be valid (no panic, no data race)
	health := c.HealthyEndpoints()
	require.Len(t, health, 1)
}