package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/honganh1206/tinker/logger"
	"github.com/honganh1206/tinker/mcp"
	"github.com/honganh1206/tinker/model"
	"github.com/honganh1206/tinker/storage"
	"github.com/honganh1206/tinker/tools"
)

// mockModel implements model.Model for testing.
type mockModel struct {
	callFn func(ctx context.Context, inputs []storage.Record) ([]storage.Record, int, error)
}

func (m *mockModel) Call(ctx context.Context, inputs []storage.Record) ([]storage.Record, int, error) {
	return m.callFn(ctx, inputs)
}

func newTestAgent(t *testing.T, mm *mockModel) *Agent {
	t.Helper()

	db, err := storage.NewContextDB(":memory:")
	require.NoError(t, err)

	cw, err := model.NewContextWindow(db, mm, "test")
	require.NoError(t, err)

	testRunner := &mockToolRunner{output: "test result"}
	testDef := tools.ToolDefinition{
		Name:        "test_tool",
		Description: "A test tool",
		Function:    testRunner.Run,
	}
	require.NoError(t, cw.RegisterTool(testDef))

	return New(&Config{
		ContextWindow: cw,
		Logger:        logger.NewDefaultLogger(),
	})
}

type mockToolRunner struct {
	output string
}

func (r *mockToolRunner) Run(ctx context.Context, args json.RawMessage) (string, error) {
	return r.output, nil
}

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		mcpCount int
	}{
		{"creates agent with MCP configs", 2},
		{"creates agent without MCP configs", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := storage.NewContextDB(":memory:")
			require.NoError(t, err)

			mm := &mockModel{
				callFn: func(ctx context.Context, inputs []storage.Record) ([]storage.Record, int, error) {
					return nil, 0, nil
				},
			}
			cw, err := model.NewContextWindow(db, mm, "default")
			require.NoError(t, err)

			mcpConfigs := make([]mcp.ServerConfig, tt.mcpCount)
			a := New(&Config{
				ContextWindow: cw,
				MCPConfigs:    mcpConfigs,
			})

			assert.NotNil(t, a)
			assert.Equal(t, cw, a.CW)
			assert.NotNil(t, a.Logger)
			if tt.mcpCount > 0 {
				assert.NotNil(t, a.MCP)
			} else {
				assert.Nil(t, a.MCP)
			}
		})
	}
}

func TestAgent_Run_SimpleTextResponse(t *testing.T) {
	mm := &mockModel{
		callFn: func(ctx context.Context, inputs []storage.Record) ([]storage.Record, int, error) {
			return []storage.Record{
				{Source: storage.ModelResp, Content: "Hello, how can I help?"},
			}, 100, nil
		},
	}
	a := newTestAgent(t, mm)

	result, err := a.Run(context.Background(), "Hello")

	assert.NoError(t, err)
	assert.Equal(t, "Hello, how can I help?", result)
}

func TestAgent_Run_WithToolUseEvents(t *testing.T) {
	mm := &mockModel{
		callFn: func(ctx context.Context, inputs []storage.Record) ([]storage.Record, int, error) {
			return []storage.Record{
				{Source: storage.ToolUse, Content: `test_tool({"query":"test"})`},
				{Source: storage.ModelResp, Content: "Tool executed successfully"},
			}, 200, nil
		},
	}
	a := newTestAgent(t, mm)

	result, err := a.Run(context.Background(), "Use the test tool")

	assert.NoError(t, err)
	assert.Equal(t, "Tool executed successfully", result)
}

func TestAgent_Run_ModelError(t *testing.T) {
	mm := &mockModel{
		callFn: func(ctx context.Context, inputs []storage.Record) ([]storage.Record, int, error) {
			return nil, 0, assert.AnError
		},
	}
	a := newTestAgent(t, mm)

	result, err := a.Run(context.Background(), "Hello")

	assert.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "model call")
}
