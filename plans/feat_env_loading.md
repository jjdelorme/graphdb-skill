# Feature Implementation Plan: Implicit .env Loading

## 📋 Todo Checklist
- [x] Install `github.com/joho/godotenv` dependency
- [x] Implement `LoadEnv` in `internal/config/loader.go` with recursive directory search
- [x] Integrate `LoadEnv` into `cmd/graphdb/main.go` initialization
- [x] Verify functionality with a test case running from a nested directory

## 🔍 Analysis & Investigation

### Context
The `graphdb` skill is distributed as a compiled Go binary located at `.gemini/skills/graphdb/scripts/graphdb`. When executed by the agent or user scripts, the working directory may vary (e.g., project root vs. skill directory). The application currently relies on standard environment variables (`NEO4J_URI`, etc.) but does not automatically load them from a `.env` file, leading to configuration failures if variables are not exported in the shell.

### Problem
*   The Go binary does not load `.env` files.
*   If run from a subdirectory (like `.gemini/skills/graphdb/scripts/`), a standard `.env` loader looking in the current directory will fail to find the project root's `.env`.
*   Users must manually source `.env` before running the tool, which is error-prone.

### Solution Strategy
1.  **Dependency:** Use `github.com/joho/godotenv` to parse `.env` files.
2.  **Logic:** Implement a robust `LoadEnv()` function that:
    *   Attempts to load `.env` from the current working directory.
    *   If not found, recursively traverses up the directory tree until it finds a `.env` file or reaches the root.
    *   This ensures the `.env` file in the project root is found regardless of where the binary is invoked from.
3.  **Integration:** Call this `LoadEnv()` function at the very beginning of the `main()` function in `cmd/graphdb/main.go` to ensure all subsequent configuration loading (via `os.Getenv`) works as expected.

## 📝 Implementation Plan

### Prerequisites
*   Go toolchain installed.
*   Network access to fetch dependencies.

### Step-by-Step Implementation

#### Phase 1: Dependency & Core Logic
1.  **Step 1.A (The Harness):** Create a reproduction test case.
    *   *Action:* Create `test/env_loading_test.go` (or similar standalone test).
    *   *Goal:* This test should create a dummy `.env` in a parent directory, change the working directory to a child, and assert that `LoadEnv` finds the variable.
2.  **Step 1.B (The Implementation):** Add dependency and implement logic.
    *   *Action:* Run `go get github.com/joho/godotenv`.
    *   *Action:* Modify `internal/config/loader.go`:
        *   Import `github.com/joho/godotenv`.
        *   Import `os`, `path/filepath`.
        *   Add function `LoadEnv() error`.
        *   Implement the recursive search logic (max depth ~5 or until root).
        *   Update `LoadConfig()` to optionally call `LoadEnv()` or let `main` handle it. (Decision: Let `main` handle it for clarity, or `LoadConfig` for ease of use. *Decision:* Add `LoadEnv()` as a public helper, call it in `main`).
3.  **Step 1.C (The Verification):** Verify the harness.
    *   *Action:* Run `go test ./test/...`.
    *   *Success:* The test passes, confirming recursive loading works.

#### Phase 2: Integration
1.  **Step 2.A (The Harness):** Verify end-to-end.
    *   *Action:* Create a shell script `test/verify_env_loading.sh`.
    *   *Content:*
        1. Create a `.env` with a unique `TEST_VAR=loaded`.
        2. Build the binary.
        3. `cd` into a deep subdirectory (e.g., `internal/config`).
        4. Run the binary (e.g., with a hidden `--check-env` flag or by checking if it logs the loaded env).
    *   *Refinement:* Since we can't easily change the binary's flags without more work, we can just rely on `LoadConfig` working.
    *   *Revised Action:* Trust unit test from Phase 1, but ensure `main.go` calls the function.
2.  **Step 2.B (The Implementation):** Hook into `main`.
    *   *Action:* Modify `cmd/graphdb/main.go`.
    *   *Detail:* Add `config.LoadEnv()` as the first line in `main()`.
3.  **Step 2.C (The Verification):**
    *   *Action:* Run `go run cmd/graphdb/main.go --help` (sanity check).
    *   *Action:* Check if `NEO4J_URI` is picked up from `.env` by running a command that requires it (e.g., `import` with bad args but checking for "URI not set" vs "Connection failed").

### Testing Strategy
*   **Unit Test:** `internal/config/loader_test.go` should test `LoadEnv` logic by mocking file system or using temp dirs.
*   **Manual/Integration:** Verify the built binary works from `.gemini/skills/graphdb/scripts/` without manually sourcing `.env`.

## 🎯 Success Criteria
*   The `graphdb` binary automatically loads environment variables from the project root `.env` file.
*   This works even when the binary is executed from a deep subdirectory (like the skill scripts folder).
*   No "missing environment variable" errors occur for variables present in `.env`.
