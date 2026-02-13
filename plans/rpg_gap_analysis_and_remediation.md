# RPG Implementation: Gap Analysis & Remediation Plan

**Date:** February 11, 2026
**Source of Truth:** Research papers `RPG.pdf`, `RPG-Encoder.pdf`
**Validated Against:** Actual Go source code on branch `rpg` (verified Feb 11, 2026)

## 1. Executive Assessment

The RPG implementation is **structurally complete but semantically hollow**. Campaign 3.5 replaced the placeholder types (`SimpleDomainDiscoverer` -> `DirectoryDomainDiscoverer`, `SimpleClusterer` -> `FileClusterer`, `MockSummarizer` -> `VertexSummarizer`) and wired up persistence. However, the implementation fundamentally misses the core insight of the research papers.

**The research says:** Extract atomic features *per function* first (Verb-Object descriptors like "validate email", "hash password"), *then* cluster by semantic similarity to discover latent functional groups that differ from file structure.

**The implementation does:** Group functions by filename, *then* summarize the group with an LLM.

This is backwards. The result is a **file browser with LLM-generated labels** -- not a semantic feature hierarchy. The `RPG_ANALYSIS.md` document correctly identified this as the "Inverted RPG" risk, but the implementation proceeded with the structural approach anyway.

---

## 2. Gap Analysis (Ordered by Severity)

Each claim below includes the exact file and line numbers where it was verified.

### CRITICAL (System doesn't deliver on its promise)

#### C1: No Atomic Feature Extraction
- **Verified at:** `internal/rpg/enrich.go:22-46`
- **Current:** `Enricher.Enrich()` operates on a `*Feature` (cluster level), sampling up to 10 functions. There is no code anywhere in `internal/rpg/` that processes individual functions to extract intent descriptors.
- **Research (RPG-Encoder.pdf):** "Atomic Feature Extraction" -- LLM scans each function and generates normalized Verb-Object feature descriptors (e.g., `["verify credentials", "issue token"]`). These atomic descriptors are the foundation for semantic clustering.
- **Impact:** Without per-function features, clustering cannot be semantic. The entire RPG value proposition depends on this step.

#### C2: Feature Embeddings Never Populated
- **Verified at:** `internal/rpg/enrich.go:18-20` (Enricher struct has only `Client Summarizer`, no Embedder), `enrich.go:44-45` (sets only `feature.Name` and `feature.Description`), `cmd/graphdb/main.go:255-257` (creates Enricher with only `Client: summarizer`)
- **Current:** `Feature.Embedding` field exists (`schema.go:9`) but is never written to. The `Enricher` has no embedder and never generates embeddings.
- **Research:** Features must be embeddable for hierarchical vector search (`search-features`).
- **Impact:** The `SearchFeatures` query (`internal/query/neo4j.go:106-147`) calls `db.index.vector.queryNodes('feature_embeddings', ...)` which will return nothing because no Feature nodes have embeddings.

#### C3: IMPLEMENTS Edge Direction Inconsistent with Research
- **Verified at:** `internal/rpg/builder.go:66-72` -- comment says `// Implementation: Cluster IMPLEMENTS Function`, edge is `SourceID: child.ID` (Feature), `TargetID: fn.ID` (Function).
- **Research & Strategy Doc:** Both define the relationship as `(:Function)-[:IMPLEMENTS]->(:Feature)` -- i.e., the function is the source, the feature is the target. The research reads this as "Function implements Feature."
- **Current code reads as:** "Feature implements Function" -- which is semantically inverted.
- **Impact:** Graph traversal queries that follow IMPLEMENTS edges will traverse in the wrong direction. Any query asking "which functions implement this feature?" via outgoing edges from Feature will fail.

#### C4: Enrichment Only Runs on Domain Features, Not Children
- **Verified at:** `cmd/graphdb/main.go:259-264` -- `for i := range features` iterates over the `[]Feature` returned by `Builder.Build()`, which are only domain-level features (see `builder.go:78` where only `rootFeatures` is returned).
- **Current:** Child features (the per-file clusters created by `FileClusterer`) are never enriched. They retain bare filename-based names (e.g., "login", "utils") and have empty descriptions and nil embeddings.
- **Additionally:** `main.go:261` passes the full `functions` slice (all functions across all domains) to each domain's Enrich call, rather than the domain-scoped subset. So every domain feature is summarized using a sample from ALL functions, not just its own.
- **Research:** Every node in the Feature hierarchy should have a meaningful name and description.

### MAJOR (Key functionality missing)

#### M1: FileClusterer is Structural, Not Semantic
- **Verified at:** `internal/rpg/cluster.go:21-24` -- uses `filepath.Base(filePath)` stripped of extension as cluster key.
- **Current:** Pure filesystem grouping. Functions in `utils.go` cluster together regardless of what they do.
- **Research (RPG-Encoder.pdf, Appendix A.1.2):** Explicitly warns that flat/structural grouping "often overlooks logical rules." Clustering should use semantic similarity of extracted feature descriptors.
- **Impact:** This creates the exact "Inverted RPG" warned about in `RPG_ANALYSIS.md` -- a file browser, not a feature map.

#### M2: Hierarchy Too Shallow (Only 2 Levels)
- **Verified at:** `internal/rpg/builder.go:32-78` -- creates domain features (line 33) and child features per cluster (line 54). No third level.
- **Current:** Domain -> Cluster. That's it.
- **Research:** Root -> Domain -> Category -> Feature, with atomic leaf nodes mapping to 1-3 functions. The paper defines a "Leaf Node" as an atomic capability, not a file's worth of functions.
- **Impact:** For large codebases, clusters contain too many functions to be useful for agentic navigation. A file with 50 functions becomes one "feature."

#### M3: No Semantic Routing / Drift Detection
- **Verified at:** No routing or drift detection code exists anywhere in `internal/rpg/`.
- **Research (Algorithm 4):** Top-Down Semantic Routing -- when a function changes, traverse from Root asking "which child best fits this description?" to find its new semantic home. This is explicitly preferred over vector-distance comparison, which the paper notes is unreliable for code drift.
- **Impact:** The graph becomes stale as code evolves. No incremental update capability.

#### M4: No `explore-domain` Query
- **Verified at:** `internal/query/interface.go` -- no `ExploreDomain` method. `internal/query/neo4j.go` -- no hierarchy traversal implementation.
- **Research & Strategy Doc:** Proposes `explore-domain` to return child features, sibling features, and parent features for hierarchical navigation.
- **Impact:** Agents can search features but cannot explore the hierarchy. No way to drill down from Domain -> Category -> Feature.

### MINOR (Quality & cleanup)

| # | Gap | Verified At |
|---|-----|-------------|
| m1 | Enricher truncates to 1000 chars, samples max 10 functions | `enrich.go:28-35` |
| m2 | Dead code: `SimpleDomainDiscoverer`, `SimpleClusterer` defined but never used | `main.go:51-64` (grep confirms no instantiation anywhere) |
| m3 | `ScopePath` only set on domain features, not children | `builder.go:54-57` (child Feature has no ScopePath) |
| m4 | `search-features` query assumes `feature_embeddings` vector index but features have no embeddings (blocked by C2) | `neo4j.go:106-147` |
| m5 | Research status doc is stale -- describes pre-3.5 state | `plans/research/rpg_implementation_status.md` |

---

## 3. Root Cause: Why the Implementation Misses the Point

The research papers describe a **generative pipeline** with three distinct stages:

```
Stage 1: EXTRACT    ->   Stage 2: CLUSTER      ->   Stage 3: LABEL
(per-function)            (by semantic similarity)    (per-cluster)
```

The current implementation collapses stages 1 and 2:

```
Stage 1+2: GROUP BY FILENAME   ->   Stage 3: LABEL
(structural, no semantics)          (per-cluster, domain-only)
```

This means:
- Functions in `utils.go` cluster together despite doing unrelated things
- Functions implementing the same feature across multiple files are separated
- The hierarchy mirrors the file system, providing zero additional insight over `ls -R`
- Child features are never labeled or described -- only domains get LLM summaries

The `DirectoryDomainDiscoverer` is a reasonable heuristic for the top level (domains ~ packages), but the leaf-level grouping must be semantic, not structural, for the RPG to have value.

---

## 4. Implementation Plan

### Phase 1: Fix Structural Bugs (Foundation)

**Goal:** Make the existing graph correct and queryable before adding new capabilities.

#### Task 1.1: Fix IMPLEMENTS edge direction
- **File:** `internal/rpg/builder.go:66-72`
- **Change:** Swap `SourceID` and `TargetID` so edges flow Function -> Feature (per research papers)
- **File:** `internal/rpg/builder_test.go`
- **Change:** Update assertions to verify correct direction

#### Task 1.2: Generate Feature embeddings
- **File:** `internal/rpg/enrich.go`
- **Change:** Add `Embedder embedding.Embedder` field to `Enricher` struct. After LLM generates name+description, call `Embedder.EmbedBatch([]string{description})` and store in `feature.Embedding`
- **File:** `cmd/graphdb/main.go:255-257`
- **Change:** Pass embedder to Enricher
- **Test:** Unit test with mock embedder verifying `Embedding` is non-nil after `Enrich()`

#### Task 1.3: Fix enrichment to cover all features and scope correctly
- **File:** `cmd/graphdb/main.go:259-264`
- **Change:** Recurse into children so all features (not just domains) get enriched. Pass domain-scoped functions, not the full set.

#### Task 1.4: Populate ScopePath on child features
- **File:** `internal/rpg/builder.go:54-57`
- **Change:** Set `ScopePath` on child features inheriting from domain

#### Task 1.5: Remove dead code
- **File:** `cmd/graphdb/main.go:46-60`
- **Change:** Delete `SimpleDomainDiscoverer` and `SimpleClusterer` (confirmed unused)

---

### Phase 2: Atomic Feature Extraction (The Missing Foundation)

**Goal:** Implement per-function intent extraction -- the core RPG differentiator from the research.

#### Task 2.1: Create FeatureExtractor interface and LLM implementation
- **New file:** `internal/rpg/extractor.go`
- **Interface:**
  ```go
  type FeatureExtractor interface {
      Extract(code string, functionName string) ([]string, error)
  }
  ```
- **Implementation:** `LLMFeatureExtractor` using existing Vertex AI / Gemini pattern. Prompt generates Verb-Object descriptors per function.
- **New file:** `internal/rpg/extractor_test.go`

#### Task 2.2: Integrate extraction into the pipeline
- **File:** `cmd/graphdb/main.go` `handleEnrichFeatures()`
- **Change:** After loading functions, run extraction on each function and store result as `atomic_features` property on the node
- **Batching:** Process in batches to manage LLM rate limits; add `--batch-size` flag

---

### Phase 3: Semantic Clustering (Replace File Browser)

**Goal:** Replace `FileClusterer` with embedding-based clustering on extracted features.

#### Task 3.1: Implement EmbeddingClusterer
- **New file:** `internal/rpg/cluster_semantic.go`
- **Algorithm:**
  1. For each function, embed its `atomic_features` (join into single string)
  2. Run K-Means on embeddings within each domain (constrained clustering per RPG_ANALYSIS.md Recommendation 1)
  3. Auto-determine K based on function count (target 3-8 functions per cluster)
- **Dependencies:** K-Means implementation (simple custom impl or `gonum` library)
- **New file:** `internal/rpg/cluster_semantic_test.go`

#### Task 3.2: Add LLM-guided cluster refinement
- **File:** `internal/rpg/cluster_semantic.go`
- **Logic:** After K-Means, if any cluster has >15 functions, ask LLM: "Are these functionally coherent? If not, suggest splits." Re-cluster if needed.
- **This addresses:** RPG_ANALYSIS.md Recommendation 2 (Topic Nodes vs Leaf Features)

#### Task 3.3: Wire semantic clustering into pipeline
- **File:** `cmd/graphdb/main.go`
- **Change:** Replace `FileClusterer{}` with `EmbeddingClusterer{Embedder: embedder}` in `handleEnrichFeatures()`
- **Keep `FileClusterer` available** as `--cluster-mode=file` flag for fast/cheap runs

---

### Phase 4: Deepen the Hierarchy

**Goal:** Add a Category layer: Domain -> Category -> Feature (3 levels per research).

#### Task 4.1: Implement two-pass clustering in Builder
- **File:** `internal/rpg/builder.go`
- **Change:** Modify `Build()` to support optional `CategoryClusterer`:
  1. First pass: Cluster domain functions into Categories (coarse, ~10-20 per domain)
  2. Second pass: Cluster category functions into Features (fine, ~3-8 per category)
  3. Generate PARENT_OF edges for both levels
- **Backward compat:** If no CategoryClusterer provided, fall back to current 2-level behavior

#### Task 4.2: Improve enrichment sampling
- **File:** `internal/rpg/enrich.go`
- **Changes:**
  - Increase truncation from 1000 to 3000 chars
  - Smart sampling: prefer functions with more outgoing CALLS edges (likely core logic)
  - Use atomic_features as additional context in the summarization prompt

---

### Phase 5: Navigation & Routing (Advanced)

**Goal:** Complete the agent navigation toolkit and enable incremental updates.

#### Task 5.1: Implement `explore-domain` query
- **File:** `internal/query/interface.go` -- add `ExploreDomain(featureID string) (*DomainExplorationResult, error)`
- **File:** `internal/query/neo4j.go` -- implement with Cypher traversal of PARENT_OF edges
- **File:** `cmd/graphdb/main.go` -- add `explore-domain` to query command

#### Task 5.2: Implement Top-Down Semantic Router
- **New file:** `internal/rpg/router.go`
- **Algorithm (per research Algorithm 4):**
  1. Generate feature descriptors for changed function
  2. Start at root of hierarchy
  3. Ask LLM: "Which child best matches these descriptors?"
  4. Recurse until leaf
  5. Compare with current IMPLEMENTS edge target
  6. If different: update edge (re-route)

#### Task 5.3: Git-driven incremental sync (future)
- **New file:** `internal/rpg/sync.go`
- **Logic:** Accept git diff, identify changed functions, re-extract features, run router
- **Defer** until Phases 1-4 are solid

---

## 5. Critical Files

| File | Action | Phase |
|------|--------|-------|
| `internal/rpg/builder.go` | Fix edge direction, fix enrichment scoping, add multi-level hierarchy | 1, 4 |
| `internal/rpg/enrich.go` | Add embedder, improve sampling | 1, 4 |
| `internal/rpg/cluster.go` | Keep as fallback option | 3 |
| `internal/rpg/schema.go` | No changes needed (schema is correct) | -- |
| `internal/rpg/discovery.go` | No changes needed (DirectoryDomainDiscoverer is adequate) | -- |
| `internal/rpg/extractor.go` | **NEW** -- atomic feature extraction | 2 |
| `internal/rpg/cluster_semantic.go` | **NEW** -- embedding-based clustering | 3 |
| `internal/rpg/router.go` | **NEW** -- top-down semantic routing | 5 |
| `internal/query/interface.go` | Add ExploreDomain method | 5 |
| `internal/query/neo4j.go` | Implement ExploreDomain query | 5 |
| `cmd/graphdb/main.go` | Wire everything, remove dead code, add flags | 1-5 |

---

## 6. Verification Plan

### After Phase 1:
```bash
# Run tests
go test ./internal/rpg/...

# Build and run enrichment on the project itself
make build
.gemini/skills/graphdb/scripts/graphdb enrich-features -dir . -input graph.jsonl -output rpg.jsonl -mock-embedding

# Verify:
# - IMPLEMENTS edges: SourceID=function, TargetID=feature
# - Feature nodes have non-null embedding arrays
# - ALL Feature nodes (domains AND children) have name/description
# - All Feature nodes have scope_path set
```

### After Phase 2:
```bash
# Run with real LLM
.gemini/skills/graphdb/scripts/graphdb enrich-features -dir . -input graph.jsonl -output rpg.jsonl -project $PROJECT -token $TOKEN

# Verify function nodes have atomic_features property
```

### After Phase 3:
```bash
# Compare FileClusterer vs EmbeddingClusterer output
.gemini/skills/graphdb/scripts/graphdb enrich-features --cluster-mode=file ...
.gemini/skills/graphdb/scripts/graphdb enrich-features --cluster-mode=semantic ...

# Manual inspection: are semantic clusters more coherent than file-based?
```

### After Phase 5:
```bash
# Load into Neo4j and test queries
.gemini/skills/graphdb/scripts/graphdb query --type search-features --target "clustering" ...
.gemini/skills/graphdb/scripts/graphdb query --type explore-domain --target "domain-rpg"
```
