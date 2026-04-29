package model

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	_ "embed"

	"github.com/google/uuid"
	"github.com/honganh1206/tinker/storage"
	"github.com/honganh1206/tinker/tools"
	_ "github.com/mattn/go-sqlite3"
)

//go:embed system.md
var systemPrompt string

type (
	ProviderName string
	ModelVersion string
)

// ContextWindow holds the LLM context manager state
type ContextWindow struct {
	model           Model
	db              *sql.DB
	maxTokens       int
	currentContext  string
	registeredTools map[string]tools.ToolDefinition
	toolRunners     map[string]tools.ToolRunner
	metrics         *storage.Metrics
}

// NewContextWindow initializes a ContextWindow.
// The caller is responsible for closing the database.
func NewContextWindow(
	db *sql.DB,
	model Model,
	contextName string,
) (*ContextWindow, error) {
	if contextName == "" {
		contextName = uuid.NewString()
	}

	cw := &ContextWindow{
		model:           model,
		db:              db,
		maxTokens:       4096,
		currentContext:  contextName,
		registeredTools: make(map[string]tools.ToolDefinition),
		toolRunners:     make(map[string]tools.ToolRunner),
		metrics:         &storage.Metrics{},
	}

	// Unnecessary check since most models can execute tools
	if toolCapable, ok := model.(tools.ToolCapable); ok {
		toolCapable.SetToolExecutor(cw)
	}

	// Check if context exists and to be loaded into the context window
	_, err := storage.GetContextByName(db, contextName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			_, err = storage.CreateContext(db, contextName)
			if err != nil {
				return nil, fmt.Errorf("create context: %w", err)
			}
			// NOTE: For now, each session only has one context
			// so we set the system prompt as the 1st record
			if err := cw.setSystemPrompt(); err != nil {
				return nil, fmt.Errorf("set system prompt: %w", err)
			}

		} else {
			return nil, fmt.Errorf("get context: %w", err)
		}
	}

	return cw, nil
}

// HasContext true if this context exists
func (cw *ContextWindow) HasContext() (bool, error) {
	if cw.currentContext != "" {
		return true, nil
	}
	return false, nil
}

// Close closes the database connection.
func (cw *ContextWindow) Close() error {
	if cw.db == nil {
		return nil
	}
	return cw.db.Close()
}

// LiveRecords retrieves all "live" records from the context.
// NOTE: This is an important function, since it's what gets sent to the LLM.
func (cw *ContextWindow) LiveRecords() ([]storage.Record, error) {
	contextID, err := storage.GetContextIDByName(cw.db, cw.currentContext)
	if err != nil {
		return nil, fmt.Errorf("live records: %w", err)
	}
	recs, err := storage.ListLiveRecords(cw.db, contextID)
	if err != nil {
		return nil, fmt.Errorf("live records: %w", err)
	}
	return recs, nil
}

// AddPrompt logs a user prompt to the current context
func (cw *ContextWindow) AddPrompt(text string) error {
	contextID, err := storage.GetContextIDByName(cw.db, cw.currentContext)
	if err != nil {
		return fmt.Errorf("add prompt: %w", err)
	}
	_, err = storage.InsertRecord(cw.db, contextID, storage.Prompt, text, true)
	if err != nil {
		return fmt.Errorf("add prompt: %w", err)
	}
	return nil
}

// AddToolCall logs a tool invocation to the current context.
func (cw *ContextWindow) AddToolCall(name, args string) error {
	contextID, err := storage.GetContextIDByName(cw.db, cw.currentContext)
	if err != nil {
		return fmt.Errorf("add tool call: %w", err)
	}
	content := fmt.Sprintf("%s(%s)", name, args)
	_, err = storage.InsertRecord(cw.db, contextID, storage.ToolUse, content, true)
	if err != nil {
		return fmt.Errorf("add tool call: %w", err)
	}
	return nil
}

// AddToolOutput logs a tool's output to the current context.
func (cw *ContextWindow) AddToolOutput(output string) error {
	contextID, err := storage.GetContextIDByName(cw.db, cw.currentContext)
	if err != nil {
		return fmt.Errorf("add tool output: %w", err)
	}
	_, err = storage.InsertRecord(cw.db, contextID, storage.ToolResult, output, true)
	if err != nil {
		return fmt.Errorf("add tool output: %w", err)
	}
	return nil
}

// setSystemPrompt sets the system prompt for the current context.
func (cw *ContextWindow) setSystemPrompt() error {
	sp := strings.TrimSpace(systemPrompt)

	contextID, err := storage.GetContextIDByName(cw.db, cw.currentContext)
	if err != nil {
		return fmt.Errorf("set system prompt: %w", err)
	}

	tx, err := cw.db.Begin()
	if err != nil {
		return fmt.Errorf("set system prompt: %w", err)
	}
	defer tx.Rollback()

	// Set live = false since the system prompt should only be sent once at the start of the session
	_, err = tx.Exec(`UPDATE records SET live = 0 WHERE context_id = ? AND source = ?`, contextID, storage.SystemPrompt)
	if err != nil {
		return fmt.Errorf("set system prompt: %w", err)
	}

	_, err = storage.InsertRecordTx(tx, contextID, storage.SystemPrompt, sp, true)
	if err != nil {
		return fmt.Errorf("set system prompt: %w", err)
	}

	return tx.Commit()
}

// RegisterTool registers a tool with this ContextWindow instance
// and stores a tool name in the database.
// Use for new sessions only.
func (cw *ContextWindow) RegisterTool(toolDef tools.ToolDefinition) error {
	cw.LoadTool(toolDef)

	contextID, err := storage.GetContextIDByName(cw.db, cw.currentContext)
	if err != nil {
		return fmt.Errorf("register tool: %w", err)
	}

	_, err = storage.AddContextTool(cw.db, contextID, toolDef.Name)
	if err != nil {
		return fmt.Errorf("register tool: %w", err)
	}

	return nil
}

// LoadTool populates the in-memory tool maps without writing to the database.
// Use for returning sessions where tools are already persisted.
func (cw *ContextWindow) LoadTool(toolDef tools.ToolDefinition) {
	cw.registeredTools[toolDef.Name] = toolDef
	cw.toolRunners[toolDef.Name] = toolDef.Function
}

// ExecuteTool implements the ToolExecutor interface
func (cw *ContextWindow) ExecuteTool(ctx context.Context, name string, args json.RawMessage) (string, error) {
	runner, exists := cw.toolRunners[name]
	if !exists {
		return "", fmt.Errorf("tool %s not registered", name)
	}
	return runner.Run(ctx, args)
}

// GetRegisteredTools returns all registered tool definitions
func (cw *ContextWindow) GetRegisteredTools() []tools.ToolDefinition {
	var tools []tools.ToolDefinition
	for _, toolDef := range cw.registeredTools {
		tools = append(tools, toolDef)
	}
	return tools
}

// HasTool checks if a tool name is available in this context.
func (cw *ContextWindow) HasTool(name string) (bool, error) {
	contextID, err := storage.GetContextIDByName(cw.db, cw.currentContext)
	if err != nil {
		return false, fmt.Errorf("has tool: %w", err)
	}
	return storage.HasContextTool(cw.db, contextID, name)
}

// Model returns the underlying Model so callers can invoke Call directly.
func (cw *ContextWindow) Model() Model {
	return cw.model
}

// AddRecord inserts a record with an arbitrary source type and content.
func (cw *ContextWindow) AddRecord(source storage.RecordType, content string) error {
	contextID, err := storage.GetContextIDByName(cw.db, cw.currentContext)
	if err != nil {
		return fmt.Errorf("add record: %w", err)
	}
	_, err = storage.InsertRecord(cw.db, contextID, source, content, true)
	if err != nil {
		return fmt.Errorf("add record: %w", err)
	}
	return nil
}

// CallModel drives an LLM.
// It composes live messages, invokes cw.model.Call(), logs the response, update token count.
func (cw *ContextWindow) CallModel(ctx context.Context) (string, error) {
	contextID, err := storage.GetContextIDByName(cw.db, cw.currentContext)
	if err != nil {
		return "", fmt.Errorf("call model in context: %w", err)
	}

	recs, err := storage.ListLiveRecords(cw.db, contextID)
	if err != nil {
		return "", fmt.Errorf("list live records: %w", err)
	}

	events, tokensUsed, err := cw.Model().Call(ctx, recs)
	if err != nil {
		return "", fmt.Errorf("call model: %w", err)
	}

	cw.metrics.Add(tokensUsed)

	var lastMsg string

	for _, event := range events {
		_, err := storage.InsertRecord(cw.db, contextID, event.Source, event.Content, event.Live)
		if err != nil {
			return "", fmt.Errorf("insert model response: %w", err)
		}
		lastMsg = event.Content
	}

	return lastMsg, nil
}

// CreateContext creates a new named context window.
func (cw *ContextWindow) CreateContext(name string) error {
	_, err := storage.CreateContext(cw.db, name)
	if err != nil {
		return fmt.Errorf("create context: %w", err)
	}
	return nil
}

// ListContexts returns all available context windows.
func (cw *ContextWindow) ListContexts() ([]storage.Context, error) {
	contexts, err := storage.ListContexts(cw.db)
	if err != nil {
		return nil, fmt.Errorf("list contexts: %w", err)
	}
	return contexts, nil
}

// GetContext retrieves context metadata by name.
func (cw *ContextWindow) GetContext(name string) (storage.Context, error) {
	ctx, err := storage.GetContextByName(cw.db, name)
	if err != nil {
		return storage.Context{}, fmt.Errorf("get context: %w", err)
	}
	return ctx, nil
}

// DeleteContext removes a context and all its records.
func (cw *ContextWindow) DeleteContext(name string) error {
	if name == cw.currentContext {
		contexts, err := storage.ListContexts(cw.db)
		if err != nil {
			return fmt.Errorf("list contexts for deletion: %w", err)
		}
		if len(contexts) <= 1 {
			_, err := storage.CreateContext(cw.db, "default")
			if err != nil {
				return fmt.Errorf("create replacement context: %w", err)
			}
			cw.currentContext = "default"
		} else {
			for _, ctx := range contexts {
				if ctx.Name != name {
					cw.currentContext = ctx.Name
					break
				}
			}
		}
	}

	err := storage.DeleteContextByName(cw.db, name)
	if err != nil {
		return fmt.Errorf("delete context: %w", err)
	}
	return nil
}
