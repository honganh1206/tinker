package mcp

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestClient() (*client, io.ReadCloser, io.WriteCloser) {
	// clientReader <- serverWriter (server sends responses to client)
	// serverReader <- clientWriter (client sends requests to server)
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()

	c := newClient(clientReader, clientWriter)
	go c.listen()

	return c, serverReader, serverWriter
}

func TestCall_Success(t *testing.T) {
	c, serverReader, serverWriter := setupTestClient()
	defer c.close()
	defer serverReader.Close()
	defer serverWriter.Close()

	type testResult struct {
		Value string `json:"value"`
	}

	// Server goroutine: read request, send response.
	go func() {
		dec := json.NewDecoder(serverReader)
		var req request
		if err := dec.Decode(&req); err != nil {
			t.Errorf("server: failed to decode request: %v", err)
			return
		}

		// ID comes as float64 from JSON, but the encoder writes it as int64.
		id := req.ID

		result := json.RawMessage(`{"value":"hello"}`)
		enc := json.NewEncoder(serverWriter)
		enc.Encode(response{
			JSONRPC: "2.0",
			Result:  &result,
			ID:      id,
		})
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var result testResult
	err := c.call(ctx, "test.method", map[string]string{"key": "val"}, &result)

	require.NoError(t, err)
	assert.Equal(t, "hello", result.Value)
}

func TestCall_ServerError(t *testing.T) {
	c, serverReader, serverWriter := setupTestClient()
	defer c.close()
	defer serverReader.Close()
	defer serverWriter.Close()

	go func() {
		dec := json.NewDecoder(serverReader)
		var req request
		if err := dec.Decode(&req); err != nil {
			t.Errorf("server: failed to decode request: %v", err)
			return
		}

		enc := json.NewEncoder(serverWriter)
		enc.Encode(response{
			JSONRPC: "2.0",
			Error:   &rpcError{Code: -32600, Message: "invalid request"},
			ID:      req.ID,
		})
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := c.call(ctx, "test.method", nil, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid request")
	assert.Contains(t, err.Error(), "-32600")
}

func TestCall_ContextCancellation(t *testing.T) {
	c, serverReader, serverWriter := setupTestClient()
	defer c.close()
	defer serverReader.Close()
	defer serverWriter.Close()

	// Server goroutine: read request but never respond.
	go func() {
		dec := json.NewDecoder(serverReader)
		var req request
		dec.Decode(&req)
		// Intentionally not responding.
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := c.call(ctx, "test.slow", nil, nil)

	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestNotify_Success(t *testing.T) {
	c, serverReader, serverWriter := setupTestClient()
	defer c.close()
	defer serverReader.Close()
	defer serverWriter.Close()

	done := make(chan struct{})

	go func() {
		defer close(done)
		dec := json.NewDecoder(serverReader)

		// Decode into a raw map to inspect the ID field.
		var raw map[string]any
		if err := dec.Decode(&raw); err != nil {
			t.Errorf("server: failed to decode notification: %v", err)
			return
		}

		// Notifications must not have an ID field (or it should be nil/zero).
		id, exists := raw["id"]
		assert.False(t, exists && id != nil, "notification should not have an ID, got: %v", id)

		assert.Equal(t, "2.0", raw["jsonrpc"])
		assert.Equal(t, "notifications/initialized", raw["method"])
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := c.notify(ctx, "notifications/initialized", nil)
	require.NoError(t, err)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for server to read notification")
	}
}

func TestListen_IgnoresNotifications(t *testing.T) {
	c, serverReader, serverWriter := setupTestClient()
	defer c.close()
	defer serverReader.Close()
	defer serverWriter.Close()

	go func() {
		dec := json.NewDecoder(serverReader)
		var req request
		if err := dec.Decode(&req); err != nil {
			t.Errorf("server: failed to decode request: %v", err)
			return
		}

		enc := json.NewEncoder(serverWriter)

		// First, send a server notification (has method, no ID) — should be ignored.
		enc.Encode(map[string]any{
			"jsonrpc": "2.0",
			"method":  "server/log",
			"params":  map[string]any{"level": "info", "message": "hello"},
		})

		// Then send the actual response.
		result := json.RawMessage(`{"ok":true}`)
		enc.Encode(response{
			JSONRPC: "2.0",
			Result:  &result,
			ID:      req.ID,
		})
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var result struct {
		OK bool `json:"ok"`
	}
	err := c.call(ctx, "test.method", nil, &result)

	require.NoError(t, err)
	assert.True(t, result.OK)
}

func TestClose_UnblocksPendingCalls(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()

	c := newClient(clientReader, clientWriter)
	go c.listen()

	// Server goroutine: read request but never respond.
	go func() {
		dec := json.NewDecoder(serverReader)
		var req request
		dec.Decode(&req)
	}()

	errCh := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		errCh <- c.call(ctx, "test.blocked", nil, nil)
	}()

	// Give the call time to be sent and registered as pending.
	time.Sleep(100 * time.Millisecond)

	// Close the reader pipe to unblock the listen goroutine's Decode call,
	// then close the client to cancel context and clean up pending calls.
	clientReader.Close()
	c.close()

	defer serverReader.Close()
	defer serverWriter.Close()
	defer clientWriter.Close()

	select {
	case err := <-errCh:
		require.Error(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for pending call to unblock after close")
	}
}
