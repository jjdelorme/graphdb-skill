# Plan: Hide "mock" embeddings using Go build tags

- [x] 1. Read `cmd/graphdb/main.go` to extract existing mocks.
- [x] 2. Create `cmd/graphdb/mocks.go` (build tag `test_mocks`).
- [x] 3. Create `cmd/graphdb/setup_prod.go` (build tag `!test_mocks`).
- [x] 4. Create `cmd/graphdb/setup_mock_mode.go` (build tag `test_mocks`).
- [x] 5. Modify `cmd/graphdb/main.go` to remove mock flags and structs.
- [x] 6. Modify `test/e2e/cli_test.go` to use build tags and env vars.
- [x] 7. Verify build and tests.
