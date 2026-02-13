# Feature Implementation Plan: Remove CLI Project Flag

## 📋 Todo Checklist
- [x] **Phase 1: Verification Harness**: Verify current CLI behavior regarding project flag.
- [x] **Phase 2: Remove Flag**: Update `cmd/graphdb/main.go` to remove `-project` flag and use `config.GoogleCloudProject`.
- [x] **Phase 3: Update Documentation**: Update `SKILL.md` to remove `-project` from examples.
- [x] **Final Review**: Ensure CLI builds and runs correctly without the flag.

## 🔍 Analysis & Investigation
The `-project` flag is currently defined in `cmd/graphdb/main.go` for the following commands:
1.  `ingest`: Used to initialize `VertexEmbedder`.
2.  `enrich-features`: Used to initialize `VertexEmbedder`, `VertexSummarizer`, and `LLMFeatureExtractor`.
3.  `query`: Used to initialize `VertexEmbedder` for semantic search types.

The flag allows overriding the GCP Project ID. However, the modernization strategy dictates that configuration should be centralized. The `internal/config` package already loads `GOOGLE_CLOUD_PROJECT` from the environment (or `.env` file) into the `Config` struct.

**Impact:**
- Removing the flag simplifies the CLI interface.
- It enforces the use of environment variables for infrastructure configuration, consistent with 12-factor app principles.
- Existing scripts or documentation using `-project` will break and need updating.

**Dependencies:**
- `internal/config`: Must correctly load `GOOGLE_CLOUD_PROJECT`. Verified: `Config` struct has the field and `LoadConfig` populates it.

## 📝 Implementation Plan

### Prerequisites
- Ensure `.env` file exists or `GOOGLE_CLOUD_PROJECT` is set in the environment for local testing.

### Step-by-Step Implementation

#### Phase 1: Verification Harness
1.  **Step 1.A (The Harness):** Create a reproduction test case.
    *   *Action:* Create `test/e2e/cli_project_flag_test.go`.
    *   *Goal:* Assert that the CLI currently accepts `-project` (optional, but good for baseline) and that it works with ONLY the env var.
    *   *Detail:* Since we are removing the flag, the test should primarily verify that setting `GOOGLE_CLOUD_PROJECT` works for a command that requires it (e.g., `ingest` or `query` with semantic search). Note: Real `ingest` requires GCP creds, so we might use `GRAPHDB_MOCK_ENABLED` but mock mode ignores the project.
    *   *Refined Strategy:* We can't easily test "it works with GCP" without credentials. We will rely on unit/integration tests that check if the config is passed correctly. For the plan, we will create a test that verifies `ingest` runs *without* the `-project` flag (using mocks) to ensure no regression in argument parsing.

#### Phase 2: Refactor `main.go`
1.  **Step 2.A (Refactor `handleIngest`):**
    *   *Action:* Edit `cmd/graphdb/main.go`.
    *   *Detail:* Remove `projectPtr := fs.String("project", ...)` in `handleIngest`.
    *   *Detail:* Replace usage of `*projectPtr` with `cfg.GoogleCloudProject`. Ensure `cfg` is loaded (it is).
2.  **Step 2.B (Refactor `handleEnrichFeatures`):**
    *   *Action:* Edit `cmd/graphdb/main.go`.
    *   *Detail:* Remove `projectPtr` definition.
    *   *Detail:* Replace usage with `cfg.GoogleCloudProject`.
3.  **Step 2.C (Refactor `handleQuery`):**
    *   *Action:* Edit `cmd/graphdb/main.go`.
    *   *Detail:* Remove `projectPtr` definition.
    *   *Detail:* Replace usage with `cfg.GoogleCloudProject`.

#### Phase 3: Update Documentation & Examples
1.  **Step 3.A (Update SKILL.md):**
    *   *Action:* Edit `.gemini/skills/graphdb/SKILL.md`.
    *   *Detail:* Remove `-project $GOOGLE_CLOUD_PROJECT` from `ingest`, `enrich-features`, and `query` examples.
    *   *Detail:* Remove mentions of `-project` in option lists.
2.  **Step 3.B (Update README.md - Optional but recommended):**
    *   *Action:* Check `README.md` for stale examples and update if necessary.

### Testing Strategy
1.  **Build CLI:** `go build -tags test_mocks -o bin/graphdb_test cmd/graphdb/main.go`.
2.  **Run Ingest (Mock):** `./bin/graphdb_test ingest -dir test/fixtures/typescript -output /dev/null`.
    *   *Expectation:* Should run successfully without `-project` flag (picking up mock mode or env var).
3.  **Run Query (Mock):** `./bin/graphdb_test query -type search-features -target "test"`.
    *   *Expectation:* Should run without error (using mock embedder).

## 🎯 Success Criteria
- The `graphdb` CLI builds successfully.
- `ingest`, `enrich-features`, and `query` commands run without the `-project` flag.
- The `-project` flag is no longer recognized (passing it should result in "flag provided but not defined" error or similar, depending on flag handling, or simply ignored if not parsed - typically `flag.ExitOnError` is used so it will fail).
- Documentation in `SKILL.md` is clean.
