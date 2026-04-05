# Inference Layer Refactor — Stateless Strategy Pattern

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Strip the inference layer down to a clean, stateless Strategy interface with two methods (`Generate`, `CountTokens`), remove all streaming code, fix all critical bugs, and normalize role semantics.

**Architecture:** Replace the current mutable-state adapter pattern with a stateless request/response model. Each provider receives a `Request` struct per call and converts internally. Conversation policy (`SummarizeHistory`, `TruncateMessage`) moves to the `agent` package. Provider-specific role mapping (`model` vs `assistant`) is handled inside each adapter, not in the shared domain model.

**Tech Stack:** Go, `github.com/anthropics/anthropic-sdk-go`, `google.golang.org/genai`, `github.com/stretchr/testify`

---

## Task 1: Define the new `Request` struct and slim `LLMClient` interface

**Files:**
- Modify: `inference/inference.go:16-33` (replace interface)
- Modify: `inference/inference.go:35-39` (remove `BaseLLMClient`)

**Step 1: Replace the `LLMClient` interface**

Replace the current 9-method interface with:

```go
type Request struct {
    Messages     []*message.Message
    Tools        []*tools.ToolDefinition
    SystemPrompt string
    MaxTokens    int64
}

type LLMClient interface {
    Generate(ctx context.Context, req Request) (*message.Message, error)
    CountTokens(ctx context.Context, req Request) (int, error)
    Provider() string
    Model() string
}
```

Remove:
- `RunInference`
- `SummarizeHistory`
- `TruncateMessage`
- `ToNativeHistory`, `ToNativeMessage`, `ToNativeTools`
- `ProviderName()` → rename to `Provider()`
- `ModelName()` → rename to `Model()`

**Step 2: Remove `BaseLLMClient` struct**

Delete the `BaseLLMClient` struct entirely (`inference.go:35-39`). Provider and model info will live on each concrete client.

**Step 3: Run tests to verify they fail**

Run: `go test ./... 2>&1 | head -40`
Expected: Compilation errors everywhere — this is the foundation change.

**Step 4: Commit**

```bash
git add inference/inference.go
git commit -m "refactor(inference): define stateless LLMClient interface with Request struct"
```

---

## Task 2: Move `SummarizeHistory` and `TruncateMessage` to the `agent` package

**Files:**
- Modify: `agent/agent.go` (add functions)
- Modify: `inference/inference.go:110-149` (delete `BaseSummarizeHistory` and `BaseTruncateMessage`)

**Step 1: Write failing tests for the moved functions**

Add tests in `agent/agent_test.go` for:
- `SummarizeHistory` — below threshold returns unchanged, above threshold keeps first + last N
- `TruncateMessage` — short tool results unchanged, long ones truncated, **fix the early-return bug**: all tool result blocks must be checked, not just the first short one

Port the existing tests from `inference/inference_test.go:112-301` but fix the early-return bug:
- Current code at `inference/inference.go:133-136` returns early on the first short `ToolResultBlock`
- Fixed version: use `continue` instead of `return msg` so all blocks are processed

**Step 2: Run tests to verify they fail**

Run: `go test ./agent/ -run "TestSummarizeHistory|TestTruncateMessage" -v`
Expected: FAIL — functions don't exist yet

**Step 3: Implement the functions as package-level functions in `agent/agent.go`**

```go
func SummarizeHistory(history []*message.Message, threshold int) []*message.Message {
    // Same logic as BaseSummarizeHistory, moved here
}

func TruncateMessage(msg *message.Message, threshold int) *message.Message {
    // Same logic as BaseTruncateMessage, but fix: continue instead of return on short blocks
}
```

**Step 4: Delete from inference package**

- Delete `BaseSummarizeHistory` and `BaseTruncateMessage` from `inference/inference.go:110-149`
- Delete `SummarizeHistory` and `TruncateMessage` delegation from `anthropic.go:48-54` and `gemini.go:56-62`
- Delete all `TestBaseLLMClient_BaseSummarize*` and `TestBaseLLMClient_BaseTruncate*` tests from `inference/inference_test.go:112-301`

**Step 5: Update `agent.Run()` to call the local function**

Change `agent.go:58`:
```go
// Before:
a.Conv.Messages = a.LLM.SummarizeHistory(a.Conv.Messages, 20)
// After:
a.Conv.Messages = SummarizeHistory(a.Conv.Messages, 20)
```

**Step 6: Run tests to verify they pass**

Run: `go test ./agent/ -v`
Expected: PASS

**Step 7: Commit**

```bash
git add agent/agent.go agent/agent_test.go inference/inference.go inference/anthropic.go inference/gemini.go inference/inference_test.go
git commit -m "refactor: move SummarizeHistory and TruncateMessage to agent package, fix early-return bug"
```

---

## Task 3: Refactor `AnthropicClient` to be stateless

**Files:**
- Modify: `inference/anthropic.go` (remove stored state, implement `Generate`/`CountTokens`)

**Step 1: Remove stored state fields**

Remove from `AnthropicClient` struct:
- `history []anthropic.MessageParam` (line 21)
- `tools []anthropic.ToolUnionParam` (line 22)
- `BaseLLMClient` embedding (line 16)

Keep: `client`, `model`, `maxTokens`, `cache`, `systemPrompt`

Add plain fields: `provider string`, `modelVersion string`

**Step 2: Implement `Generate(ctx, Request)` method**

This method should:
1. Call `toAnthropicTools(req.Tools)` → `[]anthropic.ToolUnionParam` (pure, returns value)
2. Call `toAnthropicHistory(req.Messages)` → `[]anthropic.MessageParam` (pure, returns value)
3. Build `anthropic.MessageNewParams` with converted history, tools, system prompt
4. Use system prompt from `req.SystemPrompt` if provided, otherwise fall back to `c.systemPrompt`
5. Call `c.client.Messages.New(ctx, params)` (snapshot only — no streaming)
6. Convert response via `toGenericMessage` and return
7. **Fix:** Return all errors properly — no swallowing

**Step 3: Implement `CountTokens(ctx, Request)` method**

Convert `req.Messages` to native history inline, pass to `c.client.Messages.CountTokens`.

**Step 4: Fix `toAnthropicTool` — stop swallowing errors**

At `anthropic.go:323-330`, the two error paths are swallowed with commented-out returns. Uncomment them so schema errors are surfaced:

```go
func toAnthropicTool(tool *tools.ToolDefinition) (anthropic.ToolUnionParam, error) {
    schema, err := json.Marshal(tool.InputSchema)
    if err != nil {
        return anthropic.ToolUnionParam{}, fmt.Errorf("failed to marshal tool schema: %w", err)
    }
    var anthropicSchema anthropic.ToolInputSchemaParam
    if err := json.Unmarshal(schema, &anthropicSchema); err != nil {
        return anthropic.ToolUnionParam{}, fmt.Errorf("failed to unmarshal to Anthropic schema: %w", err)
    }
    // ... rest unchanged
}
```

**Step 5: Make `toAnthropicHistory` a pure function**

Extract from `ToNativeHistory`/`ToNativeMessage` into a single pure function:

```go
func toAnthropicHistory(messages []*message.Message) ([]anthropic.MessageParam, error) {
    // Convert all messages, return the slice
    // No mutation of client fields
}
```

**Step 6: Make `toAnthropicTools` a pure function**

```go
func toAnthropicTools(tools []*tools.ToolDefinition) ([]anthropic.ToolUnionParam, error) {
    // Convert all tools, return the slice
    // No mutation of client fields
}
```

**Step 7: Delete all streaming code**

Remove:
- `runInferenceStream` method (`anthropic.go:112-179`)
- `RunInference` method (`anthropic.go:81-110`) — replaced by `Generate`
- The streaming branch and `onDelta` parameter
- Dead `strings.Builder` code (`anthropic.go:148-154`, `163-171`)

**Step 8: Delete old methods**

Remove: `ToNativeHistory`, `ToNativeMessage`, `ToNativeTools`, `SummarizeHistory`, `TruncateMessage`, `ProviderName`, `ModelName`

Replace with: `Provider()` and `Model()` returning plain strings

**Step 9: Verify compilation**

Run: `go build ./inference/`
Expected: Compiles (other packages will break, fixed in later tasks)

**Step 10: Commit**

```bash
git add inference/anthropic.go
git commit -m "refactor(anthropic): stateless Generate/CountTokens, fix swallowed errors, remove streaming"
```

---

## Task 4: Refactor `GeminiClient` to be stateless

**Files:**
- Modify: `inference/gemini.go` (remove stored state, implement `Generate`/`CountTokens`)

**Step 1: Remove stored state fields**

Remove from `GeminiClient` struct:
- `contents []*genai.Content` (line 23)
- `tools []*genai.Tool` (line 24)
- `BaseLLMClient` embedding (line 19)

Keep: `client`, `model`, `maxTokens`, `systemPrompt`

**Step 2: Normalize roles**

Remove `ModelRole = "model"` from `message/message.go:28`. Gemini adapter should map `assistant` → `model` internally:

In `toGeminiHistory` (new pure function):
```go
role := msg.Role
if role == message.AssistantRole {
    role = "model"
}
```

In `toGenericMessage` (Gemini response → generic):
- Always set `Role: message.AssistantRole`, not `message.ModelRole`

Also update `session/session.go:138` — `extractFinalMessage` currently checks both `AssistantRole` and `ModelRole`. After this change, only check `AssistantRole`.

**Step 3: Implement `Generate(ctx, Request)` method**

1. Convert `req.Messages` via pure `toGeminiHistory(messages)` → `[]*genai.Content`
2. Convert `req.Tools` via pure `toGeminiTools(tools)` → `[]*genai.Tool`
3. Build `genai.GenerateContentConfig`
4. Call `c.client.Models.GenerateContent(ctx, modelName, contents, config)` (snapshot only)
5. Convert response to generic `*message.Message` with `Role: message.AssistantRole`
6. **Fix:** Surface JSON unmarshal errors in `toParts` instead of `continue` (currently `gemini.go:318-321`)

**Step 4: Implement `CountTokens(ctx, Request)` method**

Convert `req.Messages` inline, pass to `c.client.Models.CountTokens`.

**Step 5: Make `toGeminiHistory` a pure function**

Replace `ToNativeHistory`/`ToNativeMessage` with:
```go
func toGeminiHistory(messages []*message.Message) ([]*genai.Content, error) {
    // Pure conversion, no client mutation
    // Map "assistant" → "model" here
}
```

**Step 6: Make `toGeminiTools` a pure function**

Replace `ToNativeTools` with:
```go
func toGeminiTools(tools []*tools.ToolDefinition) ([]*genai.Tool, error) {
    // Pure conversion, no client mutation
}
```

**Step 7: Delete all streaming code**

Remove:
- `runInferenceStream` method (`gemini.go:93-179`)
- `RunInference` method (`gemini.go:64-91`) — replaced by `Generate`
- Dead `outputContents` variable
- `getGeminiModelName` helper (just use `string(c.model)` inline)

**Step 8: Delete old methods**

Same as Anthropic — remove `ToNativeHistory`, `ToNativeMessage`, `ToNativeTools`, `SummarizeHistory`, `TruncateMessage`, `ProviderName`, `ModelName`

Replace with: `Provider()` and `Model()` returning plain strings

**Step 9: Verify compilation**

Run: `go build ./inference/`
Expected: Compiles

**Step 10: Commit**

```bash
git add inference/gemini.go message/message.go session/session.go
git commit -m "refactor(gemini): stateless Generate/CountTokens, normalize roles, remove streaming"
```

---

## Task 5: Fix the `Init` factory — remove `log.Fatal`

**Files:**
- Modify: `inference/inference.go:41-59`

**Step 1: Replace `log.Fatal` with error return**

Change `inference.go:52-54`:
```go
// Before:
client, err := genai.NewClient(ctx, &genai.ClientConfig{...})
if err != nil {
    log.Fatal(err)
}
// After:
client, err := genai.NewClient(ctx, &genai.ClientConfig{...})
if err != nil {
    return nil, fmt.Errorf("failed to create Gemini client: %w", err)
}
```

**Step 2: Update constructor signatures**

Both `NewAnthropicClient` and `NewGeminiClient` should accept only the SDK client, model version, max tokens, and system prompt. Remove any `BaseLLMClient` initialization from them.

Also: move system prompt creation into `Init`:
```go
case AnthropicProvider:
    client := anthropic.NewClient()
    sysPrompt := prompts.ClaudeSystemPrompt()
    return NewAnthropicClient(&client, ModelVersion(llm.Model), llm.TokenLimit, sysPrompt), nil
case GoogleProvider:
    // ...
    sysPrompt := prompts.GeminiSystemPrompt()
    return NewGeminiClient(client, ModelVersion(llm.Model), llm.TokenLimit, sysPrompt), nil
```

Gemini's `NewGeminiClient` currently calls `prompts.GeminiSystemPrompt()` internally (`gemini.go:30`). Move it to the factory for consistency with Anthropic (which already receives it from the factory at `inference.go:45-46`).

**Step 3: Remove unused imports**

After removing `BaseLLMClient` and streaming, clean up: remove `"log"`, and any unused SDK imports from `inference.go`.

**Step 4: Delete `GetDefaultModelSubagent`**

Remove `inference.go:99-108`. Not used anywhere except the old plan doc.

**Step 5: Fix provider name inconsistency**

In constructors, `AnthropicClient` sets `Provider: AnthropicModelName` ("Claude") but `Init` switches on `AnthropicProvider` ("anthropic"). Make `Provider()` return the routing ID, not the display name:

```go
func (c *AnthropicClient) Provider() string { return AnthropicProvider }  // "anthropic"
func (c *GeminiClient) Provider() string { return GoogleProvider }         // "google"
```

**Step 6: Unskip the Google missing API key test**

Rewrite `inference_test.go:70-96` (`TestInit_GoogleProvider_MissingAPIKey`) — it should now work because `log.Fatal` is gone:

```go
func TestInit_GoogleProvider_MissingAPIKey(t *testing.T) {
    os.Unsetenv("GOOGLE_API_KEY")
    llm := BaseLLMClient{Provider: GoogleProvider, Model: string(Gemini25Pro), TokenLimit: 8192}
    client, err := Init(context.Background(), llm)
    assert.Error(t, err)
    assert.Nil(t, client)
}
```

Note: `BaseLLMClient` is kept only as an input config struct for `Init` / CLI flags — rename it to `ClientConfig` for clarity since it no longer embeds into providers.

**Step 7: Run tests**

Run: `go test ./inference/ -v`
Expected: PASS

**Step 8: Commit**

```bash
git add inference/inference.go inference/inference_test.go inference/gemini.go
git commit -m "fix(inference): replace log.Fatal with error return, fix provider name inconsistency"
```

---

## Task 6: Update `agent.go` — adapt to new interface

**Files:**
- Modify: `agent/agent.go`
- Modify: `agent/agent_test.go`

**Step 1: Simplify `Agent.Run()`**

The new `Run` signature removes `onDelta`:
```go
func (a *Agent) Run(ctx context.Context, userInput string) error
```

Rewrite the loop body:
1. Remove `a.LLM.SummarizeHistory(...)` call — use local `SummarizeHistory()` (from Task 2)
2. Remove all `ToNativeHistory`, `ToNativeMessage`, `ToNativeTools` calls
3. Build a `Request` with the current conversation messages, tools, system prompt
4. Replace `a.streamResponse(ctx, onDelta)` with direct `a.LLM.Generate(ctx, req)`
5. Remove `onDelta` from `executeTool` signature
6. **Fix:** handle `CountTokens` with the new `Request`-based signature

The loop becomes:
```go
func (a *Agent) Run(ctx context.Context, userInput string) error {
    a.Conv.Messages = SummarizeHistory(a.Conv.Messages, 20)

    for {
        // append user message to conversation
        // build Request from a.Conv.Messages + a.ToolBox.Tools
        // call a.LLM.Generate(ctx, req)
        // append response to conversation
        // execute tool calls, append results
        // if no tool calls, break
    }

    count, err := a.LLM.CountTokens(ctx, req)
    // ...
}
```

**Step 2: Delete `streamResponse` method**

Remove `agent.go:206-225` entirely.

**Step 3: Remove `onDelta` from `executeTool`**

Change signature from:
```go
func (a *Agent) executeTool(id, name string, input json.RawMessage, onDelta func(string)) message.ContentBlock
```
to:
```go
func (a *Agent) executeTool(id, name string, input json.RawMessage) message.ContentBlock
```

**Step 4: Fix `executeMCPTool` — stop swallowing JSON error and use ctx**

At `agent.go:149-152`, return the error:
```go
if err != nil {
    return message.NewToolResultBlock(id, name, fmt.Sprintf("failed to parse tool input: %v", err), true)
}
```

At `agent.go:158`, use `ctx` instead of `context.Background()`.

**Step 5: Update mock and tests in `agent_test.go`**

- Update `mockLLMClient` to implement new 4-method interface (`Generate`, `CountTokens`, `Provider`, `Model`)
- Remove mocks for: `RunInference`, `SummarizeHistory`, `TruncateMessage`, `ToNativeHistory`, `ToNativeMessage`, `ToNativeTools`, `ProviderName`, `ModelName`
- Update all test call sites — remove `onDelta`, change `RunInference` expectations to `Generate`
- Delete `TestAgent_streamResponse_*` tests (method no longer exists)

**Step 6: Run tests**

Run: `go test ./agent/ -v`
Expected: PASS

**Step 7: Commit**

```bash
git add agent/agent.go agent/agent_test.go
git commit -m "refactor(agent): adapt to stateless LLMClient, remove streaming, fix swallowed errors"
```

---

## Task 7: Update `session.go` and `cmd.go` — callers of the agent

**Files:**
- Modify: `session/session.go`
- Modify: `cmd/cmd.go`

**Step 1: Update `session.go`**

- Remove `onDelta` callback definition (`session.go:61-63`)
- Change `a.Run(ctx, cfg.Prompt, onDelta)` → `a.Run(ctx, cfg.Prompt)` (lines 66, 91)
- Remove `ModelRole` check from `extractFinalMessage` (`session.go:138`) — only check `AssistantRole`
- Update `SessionConfig.LLMBase` type if renamed to `ClientConfig` in Task 5

**Step 2: Update `cmd.go`**

- Update `SessionConfig` field name if `BaseLLMClient` was renamed to `ClientConfig`
- Update CLI flag binding at `cmd.go:17` accordingly

**Step 3: Run full test suite**

Run: `go test ./... -v`
Expected: PASS

**Step 4: Lint**

Run: `golangci-lint run`
Expected: No errors

**Step 5: Commit**

```bash
git add session/session.go cmd/cmd.go
git commit -m "refactor(session,cmd): adapt to new agent.Run signature and ClientConfig"
```

---

## Task 8: Clean up `types.go` and dead constants

**Files:**
- Modify: `inference/types.go`

**Step 1: Remove `ModelRole` from `message/message.go`**

Already done in Task 4, but verify it's gone.

**Step 2: Clean up `types.go`**

- Remove `AnthropicModelName` and `GoogleModelName` constants (lines 4-5) — no longer used since `Provider()` returns the provider ID directly
- Keep `AnthropicProvider` and `GoogleProvider`
- Keep all `ModelVersion` constants and `ProviderName` type
- Remove model versions from `types.go` that are listed in `ListAvailableModels` but not in `types.go` constants (check for consistency — currently `Claude41Opus`, `Claude45Opus` are in constants but not in `ListAvailableModels`)

**Step 3: Update `ListAvailableModels` to include all declared constants**

Make sure models declared in `types.go` are listed in `ListAvailableModels`, or remove unused constants.

**Step 4: Run full test suite + lint**

Run: `go test ./... && golangci-lint run`
Expected: All pass, no lint errors

**Step 5: Commit**

```bash
git add inference/types.go message/message.go
git commit -m "chore: clean up dead constants and normalize type naming"
```

---

## Summary of what gets deleted

| What | Where | Why |
|---|---|---|
| `BaseLLMClient` struct | `inference.go:35-39` | Replaced by plain fields on each client |
| `BaseSummarizeHistory` | `inference.go:110-126` | Moved to `agent/` |
| `BaseTruncateMessage` | `inference.go:129-149` | Moved to `agent/`, early-return bug fixed |
| `runInferenceStream` | `anthropic.go:112-179`, `gemini.go:93-179` | No streaming |
| `RunInference` | both providers | Replaced by `Generate` |
| `ToNativeHistory/Message/Tools` | both providers | Made private, per-call |
| `streamResponse` | `agent.go:206-225` | Unnecessary goroutine wrapper |
| `onDelta` parameter | `agent.Run`, `executeTool` | No streaming |
| `ModelRole = "model"` | `message.go:28` | Normalized to `assistant` |
| `AnthropicModelName/GoogleModelName` | `types.go:4-5` | Display names no longer used |
| `GetDefaultModelSubagent` | `inference.go:99-108` | Unused |
| `log.Fatal` | `inference.go:53` | Library must not exit |
| Dead `strings.Builder` blocks | `anthropic.go:148-171` | Never used |
| Dead `outputContents` | `gemini.go:98` | Never used |

## Summary of bugs fixed

| Bug | Where |
|---|---|
| `log.Fatal` in library code | `inference.go:53` |
| Stream error returned as success | `anthropic.go:147-160` |
| Swallowed schema errors | `anthropic.go:323-330` |
| Swallowed JSON errors in `toParts` | `gemini.go:318-321` |
| Ignored `ToNativeHistory`/`ToNativeTools` errors | `agent.go:61-64` |
| `TruncateMessage` early return skips later blocks | `inference.go:133-136` |
| `executeMCPTool` swallows JSON error | `agent.go:149-152` |
| `executeMCPTool` ignores context | `agent.go:158` |
| `assistant` vs `model` role mismatch | `message.go:28`, `gemini.go:101,195` |
| Provider name inconsistency (display vs routing) | `anthropic.go:29`, `gemini.go:34` |
