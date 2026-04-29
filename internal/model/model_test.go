package model

import (
	"context"

	"github.com/honganh1206/tinker/internal/storage"
)

type MockModel struct {
	LastOptsDisableTools bool
	events               []storage.Record
}

func (m *MockModel) Call(ctx context.Context, inputs []storage.Record) ([]storage.Record, int, error) {
	m.LastOptsDisableTools = false // Default behavior
	return m.events, 0, nil
}

type dummyModel struct {
	cw      *ContextWindow
	events  []storage.Record
	closeDB bool
}

func (m *dummyModel) Call(ctx context.Context, inputs []storage.Record) ([]storage.Record, int, error) {
	if m.closeDB && m.cw != nil {
		m.cw.db.Close()
	}
	return m.events, 0, nil
}
