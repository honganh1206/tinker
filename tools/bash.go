// Package tools provides tool definitions for the Tinker CLI agent system.
package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

//go:embed bash.md
var bashPrompt string

const maxOutputLines = 200

type BashInput struct {
	Command string `json:"command" jsonschema_description:"The bash command to execute."`
}

var BashInputSchema = generate[BashInput]()

var BashDefinition = ToolDefinition{
	Name:        "bash",
	Description: bashPrompt,
	InputSchema: BashInputSchema,
	Function:    RunBashTool,
}

func RunBashTool(ctx context.Context, args json.RawMessage) (string, error) {
	bashInput, err := decode[BashInput](args)
	if err != nil {
		return "", fmt.Errorf("parse bash input: %w", err)
	}

	cmd := exec.Command("bash", "-c", bashInput.Command)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("Command failed with error: %s\nOutput: %s", err.Error(), string(output)), nil
	}

	result := strings.TrimSpace(string(output))
	lines := strings.Split(result, "\n")
	if len(lines) > maxOutputLines {
		truncated := len(lines) - maxOutputLines
		result = fmt.Sprintf("(%d lines truncated)\n%s", truncated, strings.Join(lines[truncated:], "\n"))
	}

	return result, err
}
