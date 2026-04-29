package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_HasTool(t *testing.T) {
	m := NewManager()
	m.routes["myserver_foo"] = route{remoteName: "foo"}
	m.routes["myserver_bar"] = route{remoteName: "bar"}

	assert.True(t, m.HasTool("myserver_foo"))
	assert.True(t, m.HasTool("myserver_bar"))
	assert.False(t, m.HasTool("myserver_baz"))
	assert.False(t, m.HasTool("foo"))
	assert.False(t, m.HasTool(""))
}

func TestManager_Call_UnknownTool(t *testing.T) {
	m := NewManager()

	_, err := m.Call(context.Background(), "nonexistent", map[string]any{"key": "value"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tool")
}

func TestManager_Call_UsesRemoteName(t *testing.T) {
	// Create pipes: client writes to serverReader, server writes to clientReader.
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()

	// Build the MCP client that talks over the pipes.
	c := newClient(clientReader, clientWriter)
	go func() {
		_ = c.listen()
	}()

	// Cleanup: close pipes first so listen() unblocks, then close the client.
	defer func() {
		clientReader.Close()
		serverWriter.Close()
		serverReader.Close()
		clientWriter.Close()
		c.close()
	}()

	// Build a server struct with just the rpcClient (no real process).
	srv := &server{
		id:        "myserver",
		rpcClient: c,
	}

	// Wire up the manager routes.
	m := NewManager()
	m.routes["myserver_mytool"] = route{srv: srv, remoteName: "mytool"}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Run the Call in a goroutine so we can act as the "server" side.
	type callResult struct {
		content []ToolResultContent
		err     error
	}
	resultCh := make(chan callResult, 1)
	go func() {
		content, err := m.Call(ctx, "myserver_mytool", map[string]any{"hello": "world"})
		resultCh <- callResult{content: content, err: err}
	}()

	// Act as the MCP server: read the JSON-RPC request from the pipe.
	decoder := json.NewDecoder(serverReader)
	var req struct {
		Method string `json:"method"`
		Params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		} `json:"params"`
		ID float64 `json:"id"`
	}

	require.NoError(t, decoder.Decode(&req))

	// The key assertion: the server must receive the remote name, not the prefixed name.
	assert.Equal(t, "tools/call", req.Method)
	assert.Equal(t, "mytool", req.Params.Name, "server should receive remote name, not prefixed name")
	assert.Equal(t, "world", req.Params.Arguments["hello"])

	// Send a valid response back.
	resp := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"result":{"content":[{"type":"text","text":"ok"}],"isError":false}}`, int(req.ID))
	_, err := fmt.Fprintln(serverWriter, resp)
	require.NoError(t, err)

	// Wait for the Call to complete.
	select {
	case r := <-resultCh:
		require.NoError(t, r.err)
		require.Len(t, r.content, 1)
		assert.Equal(t, "text", r.content[0].Type)
		assert.Equal(t, "ok", r.content[0].Text)
	case <-ctx.Done():
		t.Fatal("timed out waiting for Call to complete")
	}
}

func TestManager_Close_Empty(t *testing.T) {
	m := NewManager()
	assert.NoError(t, m.Close())
}
