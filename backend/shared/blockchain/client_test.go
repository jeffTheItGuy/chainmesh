package blockchain

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
