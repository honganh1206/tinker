package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

//go:embed grep.md
var grepSearchPrompt string

var GrepSearchDefinition = ToolDefinition{
	Name:        ToolNameGrepSearch,
	Description: grepSearchPrompt,
	InputSchema: GrepSearchInputSchema,
	Function:    RunGrepSearchTool,
}

type GrepSearchInput struct {
	Pattern   string `json:"pattern" jsonschema_description:"The regexp pattern to search for."`
	Directory string `json:"directory,omitempty" jsonschema_description:"Optional directory to scope the search."`
}

var GrepSearchInputSchema = generate[GrepSearchInput]()

func RunGrepSearchTool(ctx context.Context, args json.RawMessage) (string, error) {
	searchInput, err := decode[GrepSearchInput](args)
	if err != nil {
		return "", fmt.Errorf("parse grep_search input: %w", err)
	}

	if searchInput.Pattern == "" {
		return "", fmt.Errorf("invalid pattern parameter")
	}

	searchArgs := []string{"rg", "--json", searchInput.Pattern}

	if searchInput.Directory != "" {
		searchArgs = append(searchArgs, searchInput.Directory)
	}

	cmd := exec.Command(searchArgs[0], searchArgs[1:]...)
	if output, err := cmd.CombinedOutput(); err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if ok && exitErr.ExitCode() == 1 {
			// Empty result
			return "[]", nil
		}
		return "", fmt.Errorf("failed to run command '%s': %w (output: %s)", strings.Join(searchArgs, " "), err, output)
	} else {
		outputStr := strings.TrimSpace(string(output))
		lines := strings.Split(outputStr, "\n")
		arr := "[" + strings.Join(lines, ",") + "]"

		return arr, nil
	}
}
