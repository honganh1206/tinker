package tools

import (
	_ "embed"
	"fmt"
	"os/exec"
	"strings"

	"github.com/honganh1206/tinker/schema"
)

//go:embed finder.md
var finderPrompt string

var FinderDefinition = ToolDefinition{
	Name:        ToolNameFinder,
	Description: finderPrompt,
	InputSchema: FinderInputSchema,
	Function:    finderFunction,
}

type FinderInput struct {
	Query string `json:"query" jsonschema_description:"The search query describing what you're looking for in the codebase. Be specific and include context."`
}

var FinderInputSchema = schema.Generate[FinderInput]()

func finderFunction(input ToolInput) (string, error) {
	finderInput, err := schema.DecodeRaw[FinderInput](input.RawInput)
	if err != nil {
		return "", fmt.Errorf("failed to parse finder input: %w", err)
	}

	cmd := exec.Command("rg", "--no-heading", "--line-number", "--color", "never", "-e", finderInput.Query, ".")
	output, err := cmd.CombinedOutput()
	result := string(output)

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "No results found for: " + finderInput.Query, nil
		}
		return "", fmt.Errorf("search failed: %w", err)
	}

	lines := strings.Split(result, "\n")
	if len(lines) > 100 {
		result = strings.Join(lines[:100], "\n") + fmt.Sprintf("\n... (%d more results truncated)", len(lines)-100)
	}

	return result, nil
}
