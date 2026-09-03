---
type: Guide
title: Command-Line Interface (CLI) Complete Reference
description: Comprehensive operational reference for all graphdb CLI commands, flags, default arguments, and environment configurations.
tags: [guides, cli, reference, commands, flags]
status: stable
generated: { by: "antigravity/documenter-agent", at: "2026-09-03T14:30:00Z" }
verified: { by: "human:jasondel@google.com", at: "2026-09-03T14:30:00Z" }
sources:
  - id: main-cli
    resource: /cmd/graphdb/main.go
    title: CLI Root Dispatcher
    author: process:pipeline
    last_modified: 2026-09-03T14:00:00Z
  - id: query-cli
    resource: /cmd/graphdb/cmd_query.go
    title: CLI Unified Query Dispatcher
    author: process:pipeline
    last_modified: 2026-09-03T14:00:00Z
---

# Command-Line Interface (CLI) Complete Reference

## 1. Overview & Syntax

The `graphdb` executable provides a unified command-line interface for the entire modernization lifecycle:[^main-cli]

```bash
graphdb <command> [flags]
```

### Global Environment Overrides
All commands respect variables defined in `.env` or injected via the execution shell:
* `NEO4J_URI`: Bolt address (Default: `bolt://localhost:7687`)
* `NEO4J_USER`: Authentication user (Default: `neo4j`)
* `NEO4J_PASSWORD`: Authentication secret
* `GOOGLE_CLOUD_PROJECT`: GCP Project ID for Vertex AI
* `GOOGLE_CLOUD_LOCATION`: Vertex AI Region (e.g., `us-central1` or global)

---

## 2. Command Reference

### 2.1 `build-all`
Orchestrates the entire multi-phase ingestion and enrichment pipeline in a single automated pass.
```bash
graphdb build-all -dir <path> [flags]
```
* `-dir <string>`: Path to target repository root. (Default: `.`)
* `-clean`: Purge all existing database contents, constraints, and indexes before rebuilding.
* `--batch`: Submit asynchronous Vertex AI Batch prediction job for Phase 3 and pause.
* `--resume`: Check active Batch prediction job status and resume remaining pipeline phases.
* `--gcs-bucket <string>`: Cloud Storage bucket URI for batch staging files.

---

### 2.2 `ingest`
Parses polyglot source code files into language-agnostic JSONL node and edge streams using Tree-sitter.
```bash
graphdb ingest -dir <path> [-output <dir>] [-workers <int>] [-since-commit <sha>]
```
* `-dir <string>`: Target source code directory. (Default: `.`)
* `-output <string>`: Directory to write `nodes.jsonl` and `edges.jsonl`. (Default: `./graph_data`)
* `-workers <int>`: Concurrency level. (Default: `runtime.NumCPU()`)
* `-since-commit <string>`: Incremental mode: only parse files modified since Git SHA.

---

### 2.3 `import`
Streams JSONL entity records into Neo4j using transactional Cypher `UNWIND` batches.
```bash
graphdb import -nodes <file> -edges <file> [-batch-size <int>] [-clean]
```
* `-nodes <string>`: Path to `nodes.jsonl`. (Default: `graph_data/nodes.jsonl`)
* `-edges <string>`: Path to `edges.jsonl`. (Default: `graph_data/edges.jsonl`)
* `-batch-size <int>`: Records committed per Cypher transaction. (Default: `1000`)
* `-clean`: Purge database before loading. (Default: `false`)

---

### 2.4 `enrich-features`
Executes Phase 3 semantic extraction, vector embeddings, K-Means++ clustering, and LLM summarization.
```bash
graphdb enrich-features [--batch] [--resume] [--gcs-bucket <uri>] [--concurrency <int>]
```
* `--batch`: Run extraction asynchronously via Vertex AI Batch Prediction API.
* `--resume`: Resume pipeline after Batch API job finishes.
* `--gcs-bucket <string>`: Staging bucket for JSONL requests/responses.
* `--concurrency <int>`: Concurrent inline LLM extraction workers when not using batch mode. (Default: `5`)

---

### 2.5 `enrich-contamination`
Executes Phase 4 upward volatility flood-fill, distance decay scoring, and composite risk normalization.
```bash
graphdb enrich-contamination
```
* *No flags required.* Completes deterministically in seconds.

---

### 2.6 `enrich-history`
Parses local Git commit logs to calculate file churn frequencies and co-change coupling.
```bash
graphdb enrich-history -dir <path>
```
* `-dir <string>`: Path to local Git repository root. (Default: `.`)

---

### 2.7 `enrich-tests`
Discovers automated unit tests and creates `[:TESTS]` relationships to production target functions.
```bash
graphdb enrich-tests
```
* *No flags required.* Completes deterministically.

---

### 2.8 `enrich-topology`
Executes Phase 6 Constant Potts Model (CPM) Leiden community detection and Dual-Lens seam scoring.
```bash
graphdb enrich-topology [-gamma <float>] [-min-size <int>] [-max-size <int>] [-seed <int>] [-suppress-hubs]
```
* `-gamma <float>`: Explicit CPM resolution density. If omitted or `0.0`, runs automated bisection search.
* `-min-size <int>`: Target minimum community size during bisection search. (Default: `30`)
* `-max-size <int>`: Target maximum community size during bisection search. (Default: `250`)
* `-seed <int>`: Random seed for deterministic reproducibility. (Default: `42`)
* `-suppress-hubs`: Enable inverse-degree logarithmic damping and top 1% quarantine. (Default: `true`)
* `--offline`: Enforce zero-token local execution. (Default: `true`)

---

### 2.9 `query`
The primary interface for developers and AI agents to interrogate the modernization graph.[^query-cli]
```bash
graphdb query -type <query-name> [-target <id>] [-limit <int>] [-depth <int>] [-summary]
```
* `-type <string>`: One of the 17 query capabilities (e.g., `dual-lens-seams`, `impact`, `what-if`, `search-features`).
* `-target <string>`: Target node ID, function name, or search query string.
* `-limit <int>`: Maximum number of results to return.
* `-depth <int>`: Maximum traversal depth for transitive graph queries. (Default: `3`)
* `-summary`: Return concise token-efficient summaries rather than complete JSON trees.

---

### 2.10 `serve`
Launches the embedded HTTP server hosting the D3 Force-Directed Web Visualizer.
```bash
graphdb serve [-port <int>] [-static-dir <path>]
```
* `-port <int>`: HTTP listening port. (Default: `8080`)
* `-static-dir <string>`: Custom directory for static HTML/JS web assets.

[^main-cli]: [`main.go`](file:///home/jasondel/dev/graphdb-skill/cmd/graphdb/main.go)
[^query-cli]: [`cmd_query.go`](file:///home/jasondel/dev/graphdb-skill/cmd/graphdb/cmd_query.go)
