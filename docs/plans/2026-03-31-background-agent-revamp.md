# Background Agent Revamp — Level 1 (Option B)

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Strip tinker from a TUI-based interactive agent into a headless background agent that takes a single prompt, runs to completion with bounded retry, and exits with a structured result.

**Architecture:** Replace the interactive TUI/CLI loop with a one-shot `tinker run "<prompt>"` command. The agent loop stays intact but runs headlessly — no user input after the initial prompt. A new `RunSession` orchestrates: agentic loop → optional verification (test/lint) → bounded retry (max 2 attempts) → structured JSON output. The HTTP server, conversation persistence, and inference layer are preserved. The TUI, interactive CLI, spinner, markdown renderer, and plan UI are deleted.

**Tech Stack:** Go, cobra CLI, SQLite, Anthropic/Gemini SDKs (unchanged). New: structured `SessionResult` JSON output, `slog` for logging.

---

## What Gets Deleted

| Package/File | Reason |
|---|---|
| `cmd/tui.go` | TUI is gone |
| `cmd/cli.go` | Interactive REPL is gone |
| `cmd/interactive.go` | Orchestration replaced by `RunSession` |
| `cmd/logo.txt` | TUI asset |
| `ui/spinner.go` | TUI-only |
| `ui/markdown.go` | TUI-only rendering |
| `ui/format.go` | TUI color formatting (tview tags) |
| `ui/state.go` | TUI state pub/sub controller |
| `agent/subagent.go` + `agent/subagent_test.go` | Finder subagent — fold into main agent or remove for now |
| `tools/plan_read.go`, `tools/plan_write.go`, `tools/plan_write_test.go` | Plan tools are TUI-interactive, remove for v1 |

## What Gets Kept (as-is or with minor changes)

| Package | Status |
|---|---|
| `inference/` | Keep entirely — both providers, streaming, snapshot |
| `message/` | Keep entirely |
| `tools/` (bash, read_file, edit_file, grep, list_files, finder) | Keep. Finder becomes a regular tool (not subagent) |
| `mcp/` | Keep entirely |
| `server/` + `server/data/` + `server/db/` | Keep — persistence still needed |
| `schema/` | Keep |
| `utils/` | Keep |
| `prompts/` | Keep, minor update to system prompt |

## What Gets Created

| File | Purpose |
|---|---|
| `session/session.go` | `RunSession` — the blueprint orchestrator |
| `session/result.go` | `SessionResult` struct + JSON output |
| `session/log.go` | `slog`-based structured logger for headless output |
| `cmd/cmd.go` (rewrite) | New `run` subcommand, stripped flags |

---

### Task 1: Delete TUI and Interactive Packages

**Files:**
- Delete: `cmd/tui.go`, `cmd/cli.go`, `cmd/interactive.go`, `cmd/logo.txt`
- Delete: `ui/spinner.go`, `ui/markdown.go`, `ui/format.go`, `ui/state.go`
- Delete: `agent/subagent.go`, `agent/subagent_test.go`
- Delete: `tools/plan_read.go`, `tools/plan_write.go`, `tools/plan_write_test.go`

**Step 1: Delete the files**

```bash
rm cmd/tui.go cmd/cli.go cmd/interactive.go cmd/logo.txt
rm ui/spinner.go ui/markdown.go ui/format.go ui/state.go
rm agent/subagent.go agent/subagent_test.go
rm tools/plan_read.go tools/plan_write.go tools/plan_write_test.go
```

**Step 2: Delete the `ui/` directory entirely (it's now empty)**

```bash
rmdir ui/
```

**Step 3: Verify the project still compiles (it won't yet — that's expected)**

```bash
go build ./... 2>&1 | head -30
```

Expected: Compile errors referencing deleted packages. We'll fix these in subsequent tasks.

**Step 4: Commit the deletions**

```bash
git add -u
git commit -m "chore: strip TUI, interactive CLI, subagent, and plan tools"
```

---

### Task 2: Remove `ui` Dependencies from `agent/agent.go`

**Files:**
- Modify: `agent/agent.go`
- Modify: `go.mod` (later, after all tasks — `go mod tidy`)

The agent currently depends on `ui.Controller` for TUI state pub/sub and on `ui.FormatToolResult` for formatting. Both must go. The agent should use a simple callback for logging tool results instead.

**Step 1: Rewrite `agent/agent.go`**

Remove these from the `Agent` struct:
- `ctl *ui.Controller`
- `Sub *Subagent`
- The `streaming` field (background agent always streams to a log, not a TUI)

Remove the `ui` import entirely. Remove all `a.ctl.Publish(...)` calls. Remove `a.Sub` usage. Remove `FormatToolResultMessage` (it uses `ui.FormatToolResult`).

The `onDelta func(string)` callback stays — but in background mode it writes to the logger instead of a TUI. The `Config` struct loses `Controller` and gains a `Logger *slog.Logger`.

Replace the tool result formatting with a plain-text logger-friendly format.

Key changes to `Run()`:
- Remove the `go func() { a.ctl.Publish(...) }()` calls
- Remove the `a.runSubagent` code path in `executeLocalTool`
- Remove `IsSubTool` handling
- Remove plan tool special-casing (plan tools are deleted)

```go
// New Agent struct
type Agent struct {
	LLM     inference.LLMClient
	ToolBox *tools.ToolBox
	Conv    *data.Conversation
	client  server.APIClient
	MCP     mcp.Config
	Logger  *slog.Logger
}

// New Config struct
type Config struct {
	LLM          inference.LLMClient
	Conversation *data.Conversation
	ToolBox      *tools.ToolBox
	Client       server.APIClient
	MCPConfigs   []mcp.ServerConfig
	Logger       *slog.Logger
}
```

**Step 2: Verify it compiles**

```bash
go build ./agent/...
```

Expected: PASS (may still fail on `cmd/` — that's Task 4).

**Step 3: Run existing agent tests**

```bash
go test ./agent/... -v
```

**Step 4: Commit**

```bash
git add agent/agent.go
git commit -m "refactor: remove TUI dependencies from agent, add slog logger"
```

---

### Task 3: Create the `session` Package — Core Blueprint

**Files:**
- Create: `session/result.go`
- Create: `session/log.go`
- Create: `session/session.go`
- Create: `session/session_test.go`

This is the heart of the revamp. `RunSession` is the blueprint orchestrator that replaces `interactive()`.

#### Step 1: Write `session/result.go`

The structured output for every run:

```go
package session

import "time"

type Status string

const (
	StatusSuccess Status = "success"
	StatusPartial Status = "partial"
	StatusFailed  Status = "failed"
)

type SessionResult struct {
	SessionID      string        `json:"session_id"`
	ConversationID string        `json:"conversation_id"`
	Status         Status        `json:"status"`
	StartedAt      time.Time     `json:"started_at"`
	CompletedAt    time.Time     `json:"completed_at"`
	DurationMs     int64         `json:"duration_ms"`
	TokensUsed     int           `json:"tokens_used"`
	RetryCount     int           `json:"retry_count"`
	FinalMessage   string        `json:"final_message"`
	Error          string        `json:"error,omitempty"`
	Model          string        `json:"model"`
	Provider       string        `json:"provider"`
}
```

#### Step 2: Write `session/log.go`

A thin wrapper that creates a structured logger for headless output. All agent activity (tool calls, deltas, errors) goes through this instead of a TUI.

```go
package session

import (
	"io"
	"log/slog"
)

func NewLogger(w io.Writer, verbose bool) *slog.Logger {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level}))
}
```

#### Step 3: Write `session/session.go`

The blueprint:

```
[Deterministic] Create conversation via server API
[Deterministic] Initialize LLM + tools + MCP
[Agentic]       Run agent loop (agent.Run with prompt)
[Deterministic] Check if verification command is set
[Agentic]       If verify failed, run agent loop again with failure output (retry 1)
[Deterministic] If still failing after retry, mark as partial/failed
[Deterministic] Save conversation, count tokens, build SessionResult
[Deterministic] Output SessionResult as JSON to stdout
```

Key design:
- `RunSession(ctx, cfg) (*SessionResult, error)` is the single entry point
- Verification is optional — only runs if `--verify-cmd` flag is provided (e.g., `--verify-cmd "go test ./..."`)
- Max retry is 1 (so max 2 total agent runs, matching Stripe's "at most two rounds")
- The `onDelta` callback writes to the logger at DEBUG level

```go
package session

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/honganh1206/tinker/agent"
	"github.com/honganh1206/tinker/inference"
	"github.com/honganh1206/tinker/mcp"
	"github.com/honganh1206/tinker/server"
	"github.com/honganh1206/tinker/tools"
)

const MaxRetries = 1

type SessionConfig struct {
	LLMBase      inference.BaseLLMClient
	MCPConfigs   []mcp.ServerConfig
	Prompt       string
	VerifyCmd    string // e.g. "go test ./..."
	ServerURL    string
	Verbose      bool
}

func RunSession(ctx context.Context, cfg SessionConfig) (*SessionResult, error) {
	startedAt := time.Now()
	logger := NewLogger(os.Stderr, cfg.Verbose)

	// [Deterministic] Init LLM
	llm, err := inference.Init(ctx, cfg.LLMBase)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize LLM: %w", err)
	}

	// [Deterministic] Init server client + conversation
	client := server.NewClient(cfg.ServerURL)
	conv, err := client.CreateConversation()
	if err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	// [Deterministic] Init tools
	toolBox := &tools.ToolBox{
		Tools: []*tools.ToolDefinition{
			&tools.ReadFileDefinition,
			&tools.ListFilesDefinition,
			&tools.EditFileDefinition,
			&tools.GrepSearchDefinition,
			&tools.FinderDefinition,
			&tools.BashDefinition,
		},
	}

	// [Deterministic] Create agent
	a := agent.New(&agent.Config{
		LLM:          llm,
		Conversation: conv,
		ToolBox:      toolBox,
		Client:       client,
		MCPConfigs:   cfg.MCPConfigs,
		Logger:       logger,
	})

	a.RegisterMCPServers()
	defer a.ShutdownMCPServers()

	onDelta := func(delta string) {
		logger.Debug("agent", "delta", delta)
	}

	// [Agentic] Run agent loop
	logger.Info("running agent", "prompt", cfg.Prompt)
	err = a.Run(ctx, cfg.Prompt, onDelta)
	if err != nil {
		return buildResult(conv, startedAt, StatusFailed, 0, err.Error(), llm), nil
	}

	// [Deterministic] Verify if command is set
	retryCount := 0
	if cfg.VerifyCmd != "" {
		for attempt := 0; attempt <= MaxRetries; attempt++ {
			logger.Info("running verification", "cmd", cfg.VerifyCmd, "attempt", attempt+1)
			output, verifyErr := runVerifyCmd(ctx, cfg.VerifyCmd)

			if verifyErr == nil {
				logger.Info("verification passed")
				break
			}

			logger.Warn("verification failed", "error", verifyErr, "output", output)
			retryCount++

			if attempt < MaxRetries {
				// [Agentic] Feed failure back to agent
				retryPrompt := fmt.Sprintf(
					"The verification command `%s` failed. Output:\n\n```\n%s\n```\n\nPlease fix the issues and try again.",
					cfg.VerifyCmd, output,
				)
				logger.Info("retrying agent with failure context", "attempt", attempt+2)
				err = a.Run(ctx, retryPrompt, onDelta)
				if err != nil {
					return buildResult(conv, startedAt, StatusFailed, retryCount, err.Error(), llm), nil
				}
			} else {
				// Max retries exhausted
				return buildResult(conv, startedAt, StatusPartial, retryCount, "verification failed after max retries", llm), nil
			}
		}
	}

	return buildResult(conv, startedAt, StatusSuccess, retryCount, "", llm), nil
}

func runVerifyCmd(ctx context.Context, cmdStr string) (string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func buildResult(conv *data.Conversation, startedAt time.Time, status Status, retryCount int, errMsg string, llm inference.LLMClient) *SessionResult {
	completedAt := time.Now()
	finalMessage := ""
	// Extract last assistant text from conversation
	for i := len(conv.Messages) - 1; i >= 0; i-- {
		if conv.Messages[i].Role == "assistant" || conv.Messages[i].Role == "model" {
			for _, block := range conv.Messages[i].Content {
				if tb, ok := block.(message.TextBlock); ok {
					finalMessage = tb.Text
					break
				}
			}
			if finalMessage != "" {
				break
			}
		}
	}

	return &SessionResult{
		SessionID:      conv.ID,
		ConversationID: conv.ID,
		Status:         status,
		StartedAt:      startedAt,
		CompletedAt:    completedAt,
		DurationMs:     completedAt.Sub(startedAt).Milliseconds(),
		TokensUsed:     conv.TokenCount,
		RetryCount:     retryCount,
		FinalMessage:   finalMessage,
		Error:          errMsg,
		Model:          llm.ModelName(),
		Provider:       llm.ProviderName(),
	}
}

func OutputResult(result *SessionResult) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
```

#### Step 4: Write `session/session_test.go`

Test the `buildResult` helper and `SessionResult` JSON marshaling.

```go
package session

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSessionResultJSON(t *testing.T) {
	r := &SessionResult{
		SessionID:      "test-123",
		ConversationID: "test-123",
		Status:         StatusSuccess,
		StartedAt:      time.Now(),
		CompletedAt:    time.Now(),
		DurationMs:     1500,
		TokensUsed:     4200,
		RetryCount:     0,
		FinalMessage:   "Done",
		Model:          "claude-4-sonnet",
		Provider:       "Claude",
	}

	data, err := json.Marshal(r)
	assert.NoError(t, err)

	var decoded SessionResult
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, StatusSuccess, decoded.Status)
	assert.Equal(t, "test-123", decoded.SessionID)
}
```

**Step 5: Verify it compiles**

```bash
go build ./session/...
go test ./session/... -v
```

**Step 6: Commit**

```bash
git add session/
git commit -m "feat: add session package — blueprint orchestrator with bounded retry"
```

---

### Task 4: Rewrite `cmd/cmd.go` — The New CLI

**Files:**
- Rewrite: `cmd/cmd.go`

Strip down to two commands:
1. `tinker run "<prompt>"` — the headless agent (default)
2. `tinker serve` — the persistence server (kept as-is)

Plus `tinker version`, `tinker model`, `tinker mcp`, `tinker conversation` (all kept).

The `run` command:
- Takes a prompt as a positional arg or `--prompt` flag
- `--provider`, `--model`, `--max-tokens` (kept)
- `--verify-cmd` (new — e.g., `--verify-cmd "go test ./..."`)
- `--verbose` (kept)
- Outputs `SessionResult` JSON to stdout
- All logs go to stderr

**Step 1: Rewrite `cmd/cmd.go`**

Remove: `ChatHandler`, `useTUI`, `continueConv`, `convID`, `llmSub` vars. Remove the `interactive()` call path.

Replace `rootCmd.RunE` with a `RunHandler` that calls `session.RunSession`.

```go
func RunHandler(cmd *cobra.Command, args []string) error {
	prompt, _ := cmd.Flags().GetString("prompt")
	if prompt == "" && len(args) > 0 {
		prompt = strings.Join(args, " ")
	}
	if prompt == "" {
		return fmt.Errorf("prompt is required: tinker run \"your prompt here\"")
	}

	verifyCmd, _ := cmd.Flags().GetString("verify-cmd")

	provider := inference.ProviderName(llm.Provider)
	if llm.Model == "" {
		llm.Model = string(inference.GetDefaultModel(provider))
	}
	if llm.TokenLimit == 0 {
		llm.TokenLimit = 8192
	}

	cfg := session.SessionConfig{
		LLMBase:    llm,
		MCPConfigs: mcpServerConfigs,
		Prompt:     prompt,
		VerifyCmd:  verifyCmd,
		Verbose:    verbose,
	}

	result, err := session.RunSession(cmd.Context(), cfg)
	if err != nil {
		return err
	}

	return session.OutputResult(result)
}
```

The root command becomes:
```go
rootCmd := &cobra.Command{
	Use:   "tinker",
	Short: "A background coding agent",
	RunE:  RunHandler,
}
rootCmd.Flags().StringP("prompt", "p", "", "The task prompt")
rootCmd.Flags().String("verify-cmd", "", "Verification command to run after agent completes (e.g., 'go test ./...')")
```

**Step 2: Verify full build**

```bash
go build ./...
```

**Step 3: Commit**

```bash
git add cmd/cmd.go
git commit -m "feat: rewrite CLI for headless background agent with run command"
```

---

### Task 5: Clean Up Agent — Remove Plan Tool and FormatToolResult

**Files:**
- Modify: `agent/agent.go` — remove `executePlanTool`, `FormatToolResultMessage`, plan imports
- Modify: `tools/tools.go` — remove `ToolNamePlanRead`, `ToolNamePlanWrite` constants
- Modify: `tools/finder.go` — remove `IsSubTool: true` if present, make finder a regular tool

**Step 1: Clean agent.go**

Remove `executePlanTool()` method. Remove `FormatToolResultMessage()` function. Remove the plan-specific imports (`server/data`). Simplify `executeLocalTool` to remove the `IsSubTool` branch entirely.

Replace the `onDelta` call in `executeTool` with a simple logger call:

```go
func (a *Agent) executeTool(id, name string, input json.RawMessage, onDelta func(string)) message.ContentBlock {
	var result message.ContentBlock
	if execDetails, isMCPTool := a.MCP.ToolMap[name]; isMCPTool {
		result = a.executeMCPTool(id, name, input, execDetails)
	} else {
		result = a.executeLocalTool(id, name, input)
	}

	isError := false
	if toolResult, ok := result.(message.ToolResultBlock); ok && toolResult.IsError {
		isError = true
	}
	a.Logger.Info("tool executed", "name", name, "error", isError)

	return result
}
```

**Step 2: Clean tools/tools.go**

Remove `ToolNamePlanRead` and `ToolNamePlanWrite` constants. Remove `ToolObject` and the `*ToolObject` embed from `ToolInput`.

```go
type ToolInput struct {
	RawInput json.RawMessage
}
```

**Step 3: Update finder to not be a subtool**

Check `tools/finder.go` — if `IsSubTool: true`, change to `false` or remove the field. Finder will just run as a regular tool using the bash tool internally (or grep).

**Step 4: Verify build + tests**

```bash
go build ./...
go test ./... -v
```

**Step 5: Commit**

```bash
git add agent/agent.go tools/tools.go tools/finder.go
git commit -m "refactor: remove plan tools and subagent from agent, simplify tool execution"
```

---

### Task 6: Remove TUI Dependencies from `go.mod`

**Files:**
- Modify: `go.mod` / `go.sum`

**Step 1: Run go mod tidy**

```bash
go mod tidy
```

This should remove:
- `github.com/gdamore/tcell/v2`
- `github.com/rivo/tview`
- Various transitive deps (lucasb-eyer/go-colorful, gdamore/encoding, etc.)

**Step 2: Verify build**

```bash
go build ./...
go test ./...
```

**Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: remove TUI dependencies (tview, tcell)"
```

---

### Task 7: Update Makefile and README

**Files:**
- Modify: `Makefile`
- Modify: `README.md`

**Step 1: Update Makefile**

Add a `run` target:

```makefile
run:
	$(GORUN) ./main.go "$(PROMPT)"
serve:
	$(GORUN) ./main.go serve
```

**Step 2: Update README**

Replace TUI-focused docs with background agent usage:

```markdown
## Usage

### Run a task (headless)
```bash
tinker "fix the linting errors in cmd/"
```

### Run with verification
```bash
tinker --verify-cmd "go test ./..." "add unit tests for the user service"
```

### Run with specific model
```bash
tinker --provider anthropic --model claude-4-sonnet "refactor the auth module"
```

Output is structured JSON on stdout. Logs go to stderr.
```

**Step 3: Commit**

```bash
git add Makefile README.md
git commit -m "docs: update Makefile and README for background agent"
```

---

### Task 8: End-to-End Smoke Test

**Step 1: Start the server**

```bash
go run ./main.go serve &
```

**Step 2: Run a simple headless task**

```bash
go run ./main.go "list the files in the current directory and tell me what this project does"
```

Expected: JSON output on stdout with `status: "success"`, logs on stderr.

**Step 3: Run with verification**

```bash
go run ./main.go --verify-cmd "go test ./..." "add a comment to main.go explaining what it does"
```

Expected: Agent runs, then verification runs `go test ./...`, if it passes → `status: "success"`.

**Step 4: Verify the result JSON is parseable**

```bash
go run ./main.go "what is 2+2" 2>/dev/null | jq .
```

**Step 5: Final commit if any fixes were needed**

```bash
git add -u
git commit -m "fix: smoke test fixes for background agent"
```

---

## Summary of the New Architecture

```
tinker "fix the bug in auth.go" --verify-cmd "go test ./auth/..."
    │
    ▼
┌─────────────────────────────────────┐
│  session.RunSession()               │
│                                     │
│  [D] Init LLM, tools, MCP, conv    │
│  [A] agent.Run(prompt)              │  ← agentic loop (tool use, streaming)
│  [D] Run verify-cmd (if set)        │
│  [A] agent.Run(retry) (if failed)   │  ← bounded retry (max 1)
│  [D] Build SessionResult JSON       │
│  [D] Output to stdout               │
└─────────────────────────────────────┘
    │
    ▼
{"session_id":"...","status":"success","duration_ms":12340,...}
```

[D] = Deterministic, [A] = Agentic
