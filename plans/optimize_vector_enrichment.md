# Plan: Optimize Vector Enrichment Performance

**Goal:** Drastically reduce the time required to generate embeddings for the codebase (currently projected to be very slow for 75k functions) by optimizing I/O and API concurrency.

**Target Metrics:**
*   **Throughput:** Increase from ~1 function/sec (serial) to ~20-50 functions/sec.
*   **I/O:** Reduce file reads by factor of X (where X is avg functions per file).

---

## 1. Analysis of Bottlenecks

### 1.1 Serial API Calls (`VectorService.js`)
*   **Current:** `embedDocuments` loops through the input array and `await`s the API call one by one.
*   **Impact:** Latency is `N * (Network + Inference Time)`.
*   **Fix:** Parallelize requests with a concurrency limit (e.g., 10 concurrent requests) to mask network latency.

### 1.2 Redundant File I/O (`enrich_vectors.js`)
*   **Current:** For *each* function in a batch, the script:
    1.  Resolves path.
    2.  `fs.readFileSync(absPath)` (Reads entire file).
    3.  Extracts lines.
*   **Impact:** If `utils.js` has 50 functions, it is read from disk 50 times per batch.
*   **Fix:** "Group by File" strategy. Read `utils.js` once, extract all 50 segments in memory.

---

## 2. Test-Driven Strategy

We will follow strict TDD. We cannot refactor the logic until we have tests proving the current logic works (and fails if broken).

### Existing Tests
*   `tests/VectorService.test.js` (Needs verification of batching/concurrency correctness).
*   `tests/EnrichmentLogic.test.js` (Needs verification of source extraction accuracy).

---

## 3. Implementation Steps

### Phase 1: Test Coverage & Baselines
*   [ ] **1.1 Audit Tests:** Check `tests/VectorService.test.js` and `tests/EnrichmentLogic.test.js`.
*   [ ] **1.2 Benchmark:** Create a small script `scripts/benchmark_enrichment.js` to measure processing speed of 50 items.
*   [ ] **1.3 Hardening:** Ensure `VectorService.test.js` mocks the Google GenAI `embedContent` correctly to verify we get the right vectors back in the right order.

### Phase 2: VectorService Optimization (Concurrency)
*   [ ] **2.1 Refactor `embedDocuments`:**
    *   Introduce a concurrency control (e.g., `Promise.all` with a chunking semaphore).
    *   **Constraint:** Ensure the output array order matches the input array order 1:1.
    *   **Constraint:** Respect Rate Limits (Backoff logic must remain).
*   [ ] **2.2 Verify:** Run `tests/VectorService.test.js`.

### Phase 3: Enrichment Script Refactor (I/O)
*   [ ] **3.1 Logic Extraction:**
    *   Create a helper function `extractFunctionsFromFiles(fileGroups)` in a new or existing module.
*   [ ] **3.2 Optimization:**
    *   Implement `groupBy(file)` logic.
    *   Read file once per group.
    *   Slice lines for all requested functions in that file.
*   [ ] **3.3 Integration:**
    *   Update `enrich_vectors.js` main loop to use this new logic.
    *   Increase `BATCH_SIZE` to 200.
*   [ ] **3.4 Verify:** Run `tests/EnrichmentLogic.test.js` to ensure the code extracted matches the expected lines.

### Phase 4: Final Verification
*   [ ] **4.1 Re-run Benchmark:** Compare against Phase 1 baseline.
*   [ ] **4.2 Commit:** Finalize changes.

---

## 4. Risks
*   **Rate Limits:** Higher concurrency might hit Vertex AI quotas (60-600 QPM depending on region).
    *   *Mitigation:* The `VectorService` already has exponential backoff. We will verify this in Phase 2.
*   **Memory:** Reading 200 files into memory?
    *   *Mitigation:* We are reading source code files (KB), not huge binaries. Even 200 files @ 50KB is only 10MB. Negligible.
