# File-Based Sessions Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the HTTP server + SQLite persistence layer with file-based session storage. Sessions are JSON files in `~/.tinker/sessions/`. The agent runs in-memory only — persistence happens once at the end via a `Store` interface.

**Architecture:** Delete the entire `server/` package (HTTP server, client, data models, mocks). Move `Conversation` into `message/` (it's just a message container). Create a `store/` package with a `Store` interface and `FileStore` implementation. The agent no longer holds a storage client — it just mutates its in-memory conversation. `session.RunSession` handles persistence after the agent finishes.

**Tech Stack:** Go, JSON files, no SQLite.

---

## Task 1: Create `store/` package

Create the Store interface and FileStore implementation.

**Files:** Create `store/store.go`, `store/fs.go`, `store/fs_test.go`

## Task 2: Move Conversation out of `server/data` into `message/`

Conversation is just a container of messages — it belongs with the message package. Simplify it (remove SQLite model methods).

**Files:** Modify `message/message.go`, delete references

## Task 3: Remove `server/` package entirely

Delete server/, update all imports.

## Task 4: Remove storage from agent — make it in-memory only

Agent no longer holds a client. It just runs the loop and mutates Conv.

## Task 5: Update session to use Store + new Conversation

Session creates conversation in-memory, runs agent, saves via Store.

## Task 6: Update cmd — remove serve/conversation commands, add sessions

## Task 7: Remove SQLite dependency from go.mod

## Task 8: Build + test verification
