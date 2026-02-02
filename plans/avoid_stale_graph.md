# Plan: Avoid Stale Graph (Placeholder)

**Status:** Deferred
**Goal:** Prevent the "Stale Graph" problem where the Graph DB contains file line numbers that no longer match the code on disk due to edits made after the last graph ingestion.

## Problem Description
The `enrich_vectors.js` script (and potentially future tools) relies on reading source code from disk using `start_line` and `end_line` properties stored in the Graph. If the user edits a file (adding/deleting lines) without rebuilding the graph, these line numbers become incorrect. This leads to:
1.  Vector embeddings representing the wrong code (or comments/whitespace).
2.  LLM context containing incorrect code snippets.

## Proposed Strategies (for future release)

### 1. Hash Verification (The "Strict" Approach)
*   **Graph Schema:** Add `file_hash` (SHA256) to `File` nodes.
*   **Runtime Check:** Before reading a file, calculate its current hash. If it doesn't match the graph's `file_hash`, abort or warn.
*   **Pros:** Guarantees data integrity.
*   **Cons:** Requires hashing every file on every run (can be slow).

### 2. Modification Time (The "Fast" Approach)
*   **Graph Schema:** Add `last_modified` timestamp to `File` nodes.
*   **Runtime Check:** Compare file system `mtime` with graph `last_modified`.
*   **Pros:** Very fast.
*   **Cons:** Can be flaky (git checkout, touch).

### 3. Just-In-Time Re-Indexing (The "Smart" Approach)
*   If a mismatch is detected, trigger a mini-parser for just that file to update its nodes in real-time.
*   **Pros:** Seamless UX.
*   **Cons:** High implementation complexity.

## Current Mitigation
For now, users are advised to run `npm run graph:build` (or equivalent) after significant code changes before running vector enrichment.
