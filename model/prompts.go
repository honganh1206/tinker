package model

import (
	_ "embed"
	"strings"
)

//go:embed system.md
var systemPrompt string

func SystemPrompt() string {
	trimmedPrompt := strings.TrimSpace(systemPrompt)
	if len(trimmedPrompt) == 0 {
		return systemPrompt
	}

	return trimmedPrompt
}
