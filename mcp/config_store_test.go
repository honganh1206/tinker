package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Helpers ---

func newTestStore(t *testing.T) *FileConfigStore {
	t.Helper()
	return NewFileConfigStore(t.TempDir())
}

func cfg(id string) ServerConfig {
	return ServerConfig{
		ID:      id,
		Command: "cmd-" + id,
	}
}

// --- Tests ---

func TestFileConfigStore_Save(t *testing.T) {
	t.Parallel()

	t.Run("persists and loads correctly", func(t *testing.T) {
		t.Parallel()

		cs := newTestStore(t)
		expected := ServerConfig{ID: "fetch", Command: "uvx mcp-server-fetch"}

		require.NoError(t, cs.Save(expected))

		loaded, err := cs.Load("fetch")
		require.NoError(t, err)
		assert.Equal(t, expected, loaded)
	})

	t.Run("overwrite existing config", func(t *testing.T) {
		t.Parallel()

		cs := newTestStore(t)

		require.NoError(t, cs.Save(cfg("s1")))
		require.NoError(t, cs.Save(ServerConfig{ID: "s1", Command: "new"}))

		loaded, err := cs.Load("s1")
		require.NoError(t, err)
		assert.Equal(t, ServerConfig{ID: "s1", Command: "new"}, loaded)
	})

	t.Run("reject empty ID", func(t *testing.T) {
		t.Parallel()

		cs := newTestStore(t)

		err := cs.Save(ServerConfig{ID: "", Command: "cmd"})
		assert.Error(t, err)
	})
}

func TestFileConfigStore_Load(t *testing.T) {
	t.Parallel()

	t.Run("returns persisted config", func(t *testing.T) {
		t.Parallel()

		cs := newTestStore(t)
		expected := cfg("s1")

		require.NoError(t, cs.Save(expected))

		loaded, err := cs.Load("s1")
		require.NoError(t, err)
		assert.Equal(t, expected, loaded)
	})

	t.Run("persists across store instances", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		cs1 := NewFileConfigStore(dir)

		expected := cfg("persist")
		require.NoError(t, cs1.Save(expected))

		// simulate restart
		cs2 := NewFileConfigStore(dir)

		loaded, err := cs2.Load("persist")
		require.NoError(t, err)
		assert.Equal(t, expected, loaded)
	})
}

func TestFileConfigStore_NotFound(t *testing.T) {
	t.Parallel()

	cs := newTestStore(t)

	tests := []struct {
		name string
		op   func() error
	}{
		{
			name: "load",
			op: func() error {
				_, err := cs.Load("missing")
				return err
			},
		},
		{
			name: "delete",
			op: func() error {
				return cs.Delete("missing")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.op()
			assert.Error(t, err)

			assert.Contains(t, err.Error(), "not found")
		})
	}
}

func TestFileConfigStore_List(t *testing.T) {
	t.Parallel()

	t.Run("returns all configs", func(t *testing.T) {
		t.Parallel()

		cs := newTestStore(t)

		require.NoError(t, cs.Save(cfg("s1")))
		require.NoError(t, cs.Save(cfg("s2")))

		configs, err := cs.List()
		require.NoError(t, err)

		assert.ElementsMatch(t, []ServerConfig{
			cfg("s1"),
			cfg("s2"),
		}, configs)
	})

	t.Run("returns empty when no configs", func(t *testing.T) {
		t.Parallel()

		cs := newTestStore(t)

		configs, err := cs.List()
		require.NoError(t, err)
		assert.Empty(t, configs)
	})

	t.Run("skips corrupted files", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		cs := NewFileConfigStore(dir)

		require.NoError(t, cs.Save(cfg("valid")))

		// create corrupted file
		badFile := filepath.Join(dir, "bad.json")
		require.NoError(t, os.WriteFile(badFile, []byte("{invalid-json"), 0o644))

		configs, err := cs.List()
		require.NoError(t, err)

		assert.ElementsMatch(t, []ServerConfig{
			cfg("valid"),
		}, configs)
	})
}

func TestFileConfigStore_Delete(t *testing.T) {
	t.Parallel()

	t.Run("removes existing config", func(t *testing.T) {
		t.Parallel()

		cs := newTestStore(t)

		require.NoError(t, cs.Save(cfg("del")))

		require.NoError(t, cs.Delete("del"))

		_, err := cs.Load("del")
		assert.Error(t, err)
	})

	t.Run("delete is idempotent or returns not found", func(t *testing.T) {
		t.Parallel()

		cs := newTestStore(t)

		err := cs.Delete("missing")
		assert.Error(t, err)

		assert.Contains(t, err.Error(), "not found")
	})
}
