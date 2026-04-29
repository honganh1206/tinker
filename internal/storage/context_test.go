package storage

import (
	"database/sql"
	"testing"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	threadID := "1234567890"
	db, err := NewSession(":memory:", threadID)
	if err != nil {
		t.Fatalf("new test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestCreateContext(t *testing.T) {
	db := newTestDB(t)

	ctx, err := CreateContext(db, "my-context")
	if err != nil {
		t.Fatalf("create context: %v", err)
	}
	if ctx.Name != "my-context" {
		t.Errorf("expected name %q, got %q", "my-context", ctx.Name)
	}
	if ctx.ID == "" {
		t.Error("expected non-empty ID")
	}
	if ctx.StartTime.IsZero() {
		t.Error("expected non-zero StartTime")
	}
}

func TestCreateContext_EmptyName(t *testing.T) {
	db := newTestDB(t)

	_, err := CreateContext(db, "")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestCreateContext_Duplicate(t *testing.T) {
	db := newTestDB(t)

	_, err := CreateContext(db, "dup")
	if err != nil {
		t.Fatalf("first create: %v", err)
	}

	_, err = CreateContext(db, "dup")
	if err == nil {
		t.Fatal("expected error for duplicate context name")
	}
}

func TestGetContext(t *testing.T) {
	db := newTestDB(t)

	created, err := CreateContext(db, "lookup")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := GetContext(db, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID mismatch: %q vs %q", got.ID, created.ID)
	}
	if got.Name != created.Name {
		t.Errorf("Name mismatch: %q vs %q", got.Name, created.Name)
	}
}

func TestGetContext_NotFound(t *testing.T) {
	db := newTestDB(t)

	_, err := GetContext(db, "nonexistent-id")
	if err == nil {
		t.Fatal("expected error for missing context")
	}
}

func TestGetContextByName(t *testing.T) {
	db := newTestDB(t)

	created, err := CreateContext(db, "named")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := GetContextByName(db, "named")
	if err != nil {
		t.Fatalf("get by name: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID mismatch: %q vs %q", got.ID, created.ID)
	}
}

func TestGetContextByName_NotFound(t *testing.T) {
	db := newTestDB(t)

	_, err := GetContextByName(db, "missing")
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestGetContextIDByName(t *testing.T) {
	db := newTestDB(t)

	created, err := CreateContext(db, "id-lookup")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	id, err := GetContextIDByName(db, "id-lookup")
	if err != nil {
		t.Fatalf("get id: %v", err)
	}
	if id != created.ID {
		t.Errorf("expected %q, got %q", created.ID, id)
	}
}

func TestInsertRecord(t *testing.T) {
	db := newTestDB(t)

	ctx, err := CreateContext(db, "rec-ctx")
	if err != nil {
		t.Fatalf("create context: %v", err)
	}

	rec, err := InsertRecord(db, ctx.ID, Prompt, "hello world", true)
	if err != nil {
		t.Fatalf("insert record: %v", err)
	}
	if rec.Content != "hello world" {
		t.Errorf("expected content %q, got %q", "hello world", rec.Content)
	}
	if rec.Source != Prompt {
		t.Errorf("expected source %v, got %v", Prompt, rec.Source)
	}
	if !rec.Live {
		t.Error("expected record to be live")
	}
	if rec.EstTokens <= 0 {
		t.Error("expected positive token estimate")
	}
	if rec.ContextID != ctx.ID {
		t.Errorf("context ID mismatch: %q vs %q", rec.ContextID, ctx.ID)
	}
}

func TestInsertRecordTx(t *testing.T) {
	db := newTestDB(t)

	ctx, err := CreateContext(db, "tx-ctx")
	if err != nil {
		t.Fatalf("create context: %v", err)
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	rec, err := InsertRecordTx(tx, ctx.ID, ModelResp, "response text", true)
	if err != nil {
		tx.Rollback()
		t.Fatalf("insert record tx: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if rec.Content != "response text" {
		t.Errorf("expected content %q, got %q", "response text", rec.Content)
	}
	if rec.Source != ModelResp {
		t.Errorf("expected source %v, got %v", ModelResp, rec.Source)
	}
}

func TestListLiveRecords(t *testing.T) {
	db := newTestDB(t)

	ctx, err := CreateContext(db, "live-ctx")
	if err != nil {
		t.Fatalf("create context: %v", err)
	}

	_, err = InsertRecord(db, ctx.ID, Prompt, "live msg", true)
	if err != nil {
		t.Fatalf("insert live: %v", err)
	}
	_, err = InsertRecord(db, ctx.ID, Prompt, "dead msg", false)
	if err != nil {
		t.Fatalf("insert dead: %v", err)
	}

	records, err := ListLiveRecords(db, ctx.ID)
	if err != nil {
		t.Fatalf("list live: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 live record, got %d", len(records))
	}
	if records[0].Content != "live msg" {
		t.Errorf("expected %q, got %q", "live msg", records[0].Content)
	}
}

func TestListContexts(t *testing.T) {
	db := newTestDB(t)

	_, err := CreateContext(db, "ctx-a")
	if err != nil {
		t.Fatalf("create a: %v", err)
	}
	_, err = CreateContext(db, "ctx-b")
	if err != nil {
		t.Fatalf("create b: %v", err)
	}

	contexts, err := ListContexts(db)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(contexts) != 2 {
		t.Fatalf("expected 2 contexts, got %d", len(contexts))
	}
}

func TestAddContextTool(t *testing.T) {
	db := newTestDB(t)

	ctx, err := CreateContext(db, "tool-ctx")
	if err != nil {
		t.Fatalf("create context: %v", err)
	}

	ct, err := AddContextTool(db, ctx.ID, "bash")
	if err != nil {
		t.Fatalf("add tool: %v", err)
	}
	if ct.ToolName != "bash" {
		t.Errorf("expected tool %q, got %q", "bash", ct.ToolName)
	}
	if ct.ContextID != ctx.ID {
		t.Errorf("context ID mismatch: %q vs %q", ct.ContextID, ctx.ID)
	}
}

func TestAddContextTool_Duplicate(t *testing.T) {
	db := newTestDB(t)

	ctx, err := CreateContext(db, "dup-tool-ctx")
	if err != nil {
		t.Fatalf("create context: %v", err)
	}

	_, err = AddContextTool(db, ctx.ID, "bash")
	if err != nil {
		t.Fatalf("first add: %v", err)
	}

	_, err = AddContextTool(db, ctx.ID, "bash")
	if err == nil {
		t.Fatal("expected error for duplicate tool")
	}
}

func TestDeleteContext(t *testing.T) {
	db := newTestDB(t)

	ctx, err := CreateContext(db, "del-ctx")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_, err = InsertRecord(db, ctx.ID, Prompt, "some content", true)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	err = DeleteContext(db, ctx.ID)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err = GetContext(db, ctx.ID)
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestDeleteContextByName(t *testing.T) {
	db := newTestDB(t)

	_, err := CreateContext(db, "named-del")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	err = DeleteContextByName(db, "named-del")
	if err != nil {
		t.Fatalf("delete by name: %v", err)
	}

	_, err = GetContextByName(db, "named-del")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestDeleteContextByName_NotFound(t *testing.T) {
	db := newTestDB(t)

	err := DeleteContextByName(db, "ghost")
	if err == nil {
		t.Fatal("expected error for missing context name")
	}
}
