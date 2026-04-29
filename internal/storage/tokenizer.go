package storage

import (
	"strings"
	"sync"

	"github.com/peterheb/gotoken"
)

type TokenReporter interface {
	TotalTokens() int
	LiveTokens() (int, error)
	TokenUsage() (TokenUsage, error)
}

type TokenUsage struct {
	// Tokens currently in context window
	Live int
	// Cumulative tokens used across all calls
	Total int
	// Maximum tokens allowed in context window
	Max int
	// live/max as percentage (0.0-1.0)
	Percent float64
}

func TokenCount(s string) int {
	tokOnce.Do(func() {
		tok, tokErr = gotoken.GetTokenizer("cl100k_base")
	})
	if tokErr != nil {
		return len(strings.Fields(s))
	}
	return tok.Count(s)
}

var (
	tok gotoken.Tokenizer
	// Ensure estimating token operation happens once only
	tokOnce sync.Once
	tokErr  error
)
