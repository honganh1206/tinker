package agent

import (
	"context"
	"fmt"

	"github.com/honganh1206/tinker/logger"
	"github.com/honganh1206/tinker/mcp"
	"github.com/honganh1206/tinker/model"
	"github.com/honganh1206/tinker/tools"
)

type Agent struct {
	CW     *model.ContextWindow
	MCP    *mcp.Manager
	Logger *logger.Logger
}

type Config struct {
	ContextWindow *model.ContextWindow
	MCPConfigs    []mcp.ServerConfig
	Logger        *logger.Logger
}

func New(config *Config) *Agent {
	log := config.Logger
	if log == nil {
		log = logger.NewDefaultLogger()
	}

	a := &Agent{
		CW:     config.ContextWindow,
		Logger: log,
	}

	if len(config.MCPConfigs) > 0 {
		a.MCP = mcp.NewManager()
	}

	return a
}

func (a *Agent) StartMCP(ctx context.Context, configs []mcp.ServerConfig) error {
	if a.MCP == nil {
		return nil
	}

	mcpTools, err := a.MCP.Start(ctx, configs)
	if err != nil {
		return err
	}

	for _, t := range mcpTools {
		// runner := &tools.MCPToolRunner{Manager: a.MCP, Name: t.Name}
		if err := a.CW.RegisterTool(tools.ToolDefinition{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		}); err != nil {
			return fmt.Errorf("register MCP tool %s: %w", t.Name, err)
		}
	}

	return nil
}

// Run handles a single user message, invokes the model (which handles the
// tool-use loop internally via ContextWindow as ToolExecutor), persists all
// returned records, and returns the final text response.
func (a *Agent) Run(ctx context.Context, userInput string) (string, error) {
	if err := a.CW.AddPrompt(userInput); err != nil {
		return "", fmt.Errorf("add prompt: %w", err)
	}

	response, err := a.CW.CallModel(ctx)
	if err != nil {
		return "", fmt.Errorf("model call: %w", err)
	}

	return response, nil
}
