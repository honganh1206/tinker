package tools

import (
	_ "embed"
	"fmt"
	"os"
	"strings"

	"github.com/honganh1206/tinker/schema"
)

//go:embed read_file.md
var readFilePrompt string

const defaultMaxLines = 500

type ReadFileInput struct {
	Path      string `json:"path" jsonschema_description:"The absolute path of a file in the working directory."`
	StartLine int    `json:"start_line,omitempty" jsonschema_description:"The 1-indexed line number to start reading from. Defaults to 1."`
	EndLine   int    `json:"end_line,omitempty" jsonschema_description:"The 1-indexed line number to stop reading at (inclusive). Defaults to start_line + 499."`
}

var ReadFileInputSchema = schema.Generate[ReadFileInput]()

var ReadFileDefinition = ToolDefinition{
	Name:        ToolNameReadFile,
	Description: readFilePrompt,
	InputSchema: ReadFileInputSchema,
	Function:    ReadFile,
}

func ReadFile(input ToolInput) (string, error) {
	readFileInput, err := schema.DecodeRaw[ReadFileInput](input.RawInput)
	if err != nil {
		return "", fmt.Errorf("failed to parse read_file input: %w", err)
	}

	content, err := os.ReadFile(readFileInput.Path)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(content), "\n")
	totalLines := len(lines)

	start := readFileInput.StartLine
	if start <= 0 {
		start = 1
	}

	end := readFileInput.EndLine
	if end <= 0 {
		end = start + defaultMaxLines - 1
	}

	if start > totalLines {
		return fmt.Sprintf("(File has %d lines, start_line %d is beyond end of file)", totalLines, start), nil
	}

	if end > totalLines {
		end = totalLines
	}

	var sb strings.Builder
	for i := start; i <= end; i++ {
		fmt.Fprintf(&sb, "%d: %s\n", i, lines[i-1])
	}

	if end < totalLines {
		fmt.Fprintf(&sb, "\n(%d lines remaining, file has %d total lines)", totalLines-end, totalLines)
	}

	return sb.String(), nil
}
