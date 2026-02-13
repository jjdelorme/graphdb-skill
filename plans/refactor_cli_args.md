# Feature Implementation Plan: Refactor CLI Arguments

## 📋 Todo Checklist
- [x] Phase 1: Verification (Characterize current state)
- [x] Phase 2: Implementation (Remove flags and update logic)
- [x] Phase 3: Verification (Ensure flags are gone and defaults work)
- [x] Final Review

## 🔍 Analysis & Investigation
The `graphdb` CLI currently exposes `-location` and `-model` flags for the `ingest` and `enrich-features` commands. These flags overlap with the configuration loaded from environment variables via `internal/config`. To simplify the CLI and enforce a single source of truth for configuration, these flags should be removed.

### Current State
*   **File:** `cmd/graphdb/main.go`
*   **Command:** `ingest`
    *   Flags: `-location`, `-model`
    *   Usage: Used to initialize `Embedder`.
*   **Command:** `enrich-features`
    *   Flags: `-location`, `-model`
    *   Usage: Used to initialize `Embedder`, `Extractor`, and `Summarizer`.

### Dependencies
*   `internal/config`: providing `GoogleCloudLocation` and `GeminiEmbeddingModel`.
*   `cmd/graphdb/setup_prod.go`: `setupEmbedder`, `setupSummarizer`, `setupExtractor` functions (no changes needed, just call sites).

## 📝 Implementation Plan

### Prerequisites
*   Ensure `.env` file or environment variables are set for `GOOGLE_CLOUD_LOCATION` and `GEMINI_EMBEDDING_MODEL` if running actual ingestion/enrichment (mock mode handles this for tests).

### Step-by-Step Implementation

#### Phase 1: Verification
1.  **Step 1.A (Characterize):** Run help commands to confirm existence of flags.
    *   *Action:* Run `go run ./cmd/graphdb ingest --help`.
    *   *Action:* Run `go run ./cmd/graphdb enrich-features --help`.
    *   *Goal:* Record output showing `-location` and `-model`.

#### Phase 2: Implementation
1.  **Step 2.A (Refactor `ingest`):** Remove flags from `handleIngest`.
    *   *Action:* Modify `cmd/graphdb/main.go`.
    *   *Detail:*
        *   Remove `locationPtr := fs.String("location", ...)`
        *   Remove `modelPtr := fs.String("model", ...)`
        *   Retrieve values from `cfg`:
            ```go
            loc := cfg.GoogleCloudLocation
            if loc == "" { loc = "us-central1" } // Default fallback
            
            model := cfg.GeminiEmbeddingModel
            if model == "" { model = "gemini-embedding-001" } // Default fallback
            ```
        *   Pass `loc` and `model` to `setupEmbedder`.
2.  **Step 2.B (Refactor `enrich-features`):** Remove flags from `handleEnrichFeatures`.
    *   *Action:* Modify `cmd/graphdb/main.go`.
    *   *Detail:*
        *   Remove `locationPtr` and `modelPtr`.
        *   Implement same fallback logic as above.
        *   Pass `loc` and `model` to `setupExtractor`, `setupEmbedder`, `setupSummarizer`.

#### Phase 3: Verification
1.  **Step 3.A (Verify Removal):** Run help commands again.
    *   *Action:* Run `go run ./cmd/graphdb ingest --help`.
    *   *Action:* Run `go run ./cmd/graphdb enrich-features --help`.
    *   *Success:* Output should NOT contain `-location` or `-model`.
2.  **Step 3.B (Verify Regressions):** Run existing tests.
    *   *Action:* Run `go test ./test/e2e/...`
    *   *Success:* All tests pass.

## 🎯 Success Criteria
*   `ingest` and `enrich-features` commands no longer accept `-location` and `-model`.
*   Application logic correctly uses `internal/config` (Env vars) for these values.
*   Defaults ("us-central1", "gemini-embedding-001") are preserved if env vars are missing.
