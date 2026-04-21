package mcp

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/invopop/jsonschema"
)

// Manager owns all MCP runtime state — server processes, tool routing, etc.
type Manager struct {
	servers map[string]*server
	routes  map[string]route
}

type route struct {
	srv        *server
	remoteName string
}

// MCPTool represents an MCP tool exposed to the agent with a prefixed name.
type MCPTool struct {
	Name        string
	Description string
	InputSchema *jsonschema.Schema
}

type server struct {
	id        string
	proc      *exec.Cmd
	rpcClient *client
	closer    io.Closer
}

// stdioReadWriteCloser bundles stdin/stdout pipes.
type stdioReadWriteCloser struct {
	io.Reader
	io.Writer
	stdinCloser  io.Closer
	stdoutCloser io.Closer
}

func (s *stdioReadWriteCloser) Close() error {
	stdinCloseErr := s.stdinCloser.Close()
	stdoutCloseErr := s.stdoutCloser.Close()
	if stdinCloseErr != nil {
		return stdinCloseErr
	}
	return stdoutCloseErr
}

type initializeParams struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ClientInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"clientInfo"`
}

type initializeResult struct {
	Capabilities map[string]any `json:"capabilities,omitempty"`
}

type toolsCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type toolsCallResult struct {
	Content []ToolResultContent `json:"content"`
	IsError bool                `json:"isError"`
}

type toolsListParams struct {
	Cursor string `json:"cursor,omitempty"`
}

type toolsListResult struct {
	Tools      []*Tool `json:"tools"`
	NextCursor string  `json:"nextCursor,omitempty"`
}

func NewManager() *Manager {
	return &Manager{
		servers: make(map[string]*server),
		routes:  make(map[string]route),
	}
}

func newServer(id, command string, args []string) *server {
	return &server{
		id:   id,
		proc: exec.Command(command, args...),
	}
}

func (s *server) start(ctx context.Context) error {
	stdin, err := s.proc.StdinPipe()
	if err != nil {
		return fmt.Errorf("mcp server: failed to get stdin pipe: %w", err)
	}

	stdout, err := s.proc.StdoutPipe()
	if err != nil {
		return fmt.Errorf("mcp server: failed to get stdout pipe: %w", err)
	}

	rwc := &stdioReadWriteCloser{
		Reader:       stdout,
		Writer:       stdin,
		stdinCloser:  stdin,
		stdoutCloser: stdout,
	}
	s.closer = rwc

	s.rpcClient = newClient(stdout, stdin)

	if err := s.proc.Start(); err != nil {
		return fmt.Errorf("mcp server: failed to start server process: %w", err)
	}

	go func() {
		if err := s.rpcClient.listen(); err != nil && err != io.EOF && err != context.Canceled && !strings.Contains(err.Error(), "file already closed") {
			fmt.Fprintf(os.Stderr, "MCP client listener error: %v\n", err)
		}
	}()

	params := &initializeParams{
		ProtocolVersion: "2024-11-05",
		Capabilities:    map[string]any{},
		ClientInfo: struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		}{
			Name:    "tinker-mcp-client",
			Version: "0.1.0",
		},
	}

	var result initializeResult
	if err := s.rpcClient.call(ctx, "initialize", params, &result); err != nil {
		_ = s.close()
		return fmt.Errorf("mcp server: jsonrpc call to 'initialize' failed: %w", err)
	}

	if err := s.rpcClient.notify(ctx, "notifications/initialized", nil); err != nil {
		_ = s.close()
		return fmt.Errorf("mcp server: jsonrpc notify to 'notifications/initialized' failed: %w", err)
	}

	return nil
}

func (s *server) listTools(ctx context.Context) ([]*Tool, error) {
	var result toolsListResult
	if err := s.rpcClient.call(ctx, "tools/list", &toolsListParams{}, &result); err != nil {
		return nil, fmt.Errorf("mcp server: jsonrpc call to 'tools/list' failed: %w", err)
	}
	return result.Tools, nil
}

func (s *server) call(ctx context.Context, toolName string, args map[string]any) ([]ToolResultContent, error) {
	var result toolsCallResult
	if err := s.rpcClient.call(ctx, "tools/call", &toolsCallParams{Name: toolName, Arguments: args}, &result); err != nil {
		return nil, fmt.Errorf("mcp server: jsonrpc call to 'tools/call' (tool: %s) failed: %w", toolName, err)
	}
	if result.IsError {
		if len(result.Content) > 0 && result.Content[0].Type == "text" {
			return result.Content, fmt.Errorf("mcp server: tool call for '%s' failed with server-side error: %s", toolName, result.Content[0].Text)
		}
		return result.Content, fmt.Errorf("mcp server: tool call for '%s' failed with server-side error", toolName)
	}
	return result.Content, nil
}

func (s *server) close() error {
	var firstErr error

	if s.rpcClient != nil {
		if err := s.rpcClient.close(); err != nil {
			firstErr = fmt.Errorf("mcp server: failed to close rpc client: %w", err)
		}
	}

	if s.closer != nil {
		if err := s.closer.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("mcp server: failed to close server pipes: %w", err)
		}
	}

	if s.proc != nil && s.proc.Process != nil {
		if err := s.proc.Process.Signal(os.Interrupt); err != nil {
			if killErr := s.proc.Process.Kill(); killErr != nil && firstErr == nil {
				firstErr = fmt.Errorf("mcp server: failed to kill server process: %w", killErr)
			}
		}
		_ = s.proc.Wait()
	}

	return firstErr
}

func (m *Manager) Start(ctx context.Context, configs []ServerConfig) ([]MCPTool, error) {
	var exposed []MCPTool

	for _, cfg := range configs {
		parts := strings.Fields(cfg.Command)
		if len(parts) == 0 {
			return nil, fmt.Errorf("mcp server %s: command cannot be empty", cfg.ID)
		}

		srv := newServer(cfg.ID, parts[0], parts[1:])
		if err := srv.start(ctx); err != nil {
			return nil, fmt.Errorf("mcp server %s: %w", cfg.ID, err)
		}
		m.servers[cfg.ID] = srv

		tools, err := srv.listTools(ctx)
		if err != nil {
			return nil, fmt.Errorf("mcp server %s: %w", cfg.ID, err)
		}

		for _, t := range tools {
			prefixed := fmt.Sprintf("%s_%s", cfg.ID, t.Name)
			m.routes[prefixed] = route{srv: srv, remoteName: t.Name}
			exposed = append(exposed, MCPTool{
				Name:        prefixed,
				Description: t.Description,
				InputSchema: t.InputSchema,
			})
		}
	}

	return exposed, nil
}

func (m *Manager) Call(ctx context.Context, name string, args map[string]any) ([]ToolResultContent, error) {
	r, ok := m.routes[name]
	if !ok {
		return nil, fmt.Errorf("mcp: unknown tool %q", name)
	}
	return r.srv.call(ctx, r.remoteName, args)
}

func (m *Manager) HasTool(name string) bool {
	_, ok := m.routes[name]
	return ok
}

func (m *Manager) Close() error {
	var firstErr error
	for _, srv := range m.servers {
		if err := srv.close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
