package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var skipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"dist":         true,
	".next":        true,
	".nuxt":        true,
	"__pycache__":  true,
	".venv":        true,
	"vendor":       true,
	".cache":       true,
	"build":        true,
}

var ListFilesDefinition = ToolDefinition{
	Name:        "list_files",
	Description: "List files and directories at a given path. If no path is provided, list files in the current directory",
	InputSchema: ListFilesInputSchema,
	Function:    RunListFilesTool,
}

type ListFilesInput struct {
	Path string `json:"path,omitempty" jsonschema_description:"Optional relative path to list files from. Defaults to current directory if not provided."`
}

var ListFilesInputSchema = generate[ListFilesInput]()

func RunListFilesTool(ctx context.Context, args json.RawMessage) (string, error) {
	listFilesInput, err := decode[ListFilesInput](args)
	if err != nil {
		return "", fmt.Errorf("parse list_files input: %w", err)
	}

	dir := "."
	if listFilesInput.Path != "" {
		dir = listFilesInput.Path
	}

	var fileNames []string

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && path != dir && (skipDirs[info.Name()] || strings.HasPrefix(info.Name(), ".")) {
			return filepath.SkipDir
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		if relPath != "." {
			if info.IsDir() {
				fileNames = append(fileNames, relPath+"/")
			} else {
				fileNames = append(fileNames, relPath)
			}
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	result, err := json.Marshal(fileNames)
	if err != nil {
		return "", err
	}

	return string(result), nil
}
