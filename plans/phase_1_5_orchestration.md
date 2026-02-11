# Phase 1.5: Orchestration (Walker + Worker Pool + Main)

## Goals
Implement the orchestration layer that connects file walking, parsing, embedding, and storage.

## Tasks

- [x] **Step 1.5.A (Red):** Create E2E/Integration tests for Walker/Worker.
    - [x] Create `test/e2e/walker_test.go`.
    - [x] Implement Mock `Emitter` and `Embedder`.
    - [x] Define test case to walk `test/fixtures/`, parse, embed, and emit.

- [x] **Step 1.5.B (Green):** Implement Worker and Walker.
    - [x] Create `internal/ingest/worker.go` (WorkerPool).
    - [x] Create `internal/ingest/walker.go` (Walker).
    - [x] Create `cmd/graphdb/main.go`.

- [x] **Verify:**
    - [x] Run tests `go test ./test/e2e/...`.
    - [x] Manual verification with `cmd/graphdb`.
