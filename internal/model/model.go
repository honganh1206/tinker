package model

import (
	"context"

	"github.com/honganh1206/tinker/internal/storage"
	_ "github.com/mattn/go-sqlite3"
)

// Model abstracts out an LLM client library
type Model interface {
	// Call sends the message and return model reply and token usage
	Call(ctx context.Context, inputs []storage.Record) (events []storage.Record, tokenUsed int, err error)
}


