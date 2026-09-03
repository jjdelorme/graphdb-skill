---
type: Pipeline Phase
title: "Phase 3: Semantic RPG Intent Construction"
description: Complete specification of LLM atomic extraction, Vertex AI Batch API workflows, vector embedding generation, and K-Means++ domain clustering.
tags: [pipeline, phase3, rpg, extraction, embeddings, clustering, batch-api, vertex-ai]
status: stable
generated: { by: "antigravity/documenter-agent", at: "2026-09-03T14:30:00Z" }
verified: { by: "human:jasondel@google.com", at: "2026-09-03T14:30:00Z" }
sources:
  - id: cmd-enrich-features
    resource: /cmd/graphdb/cmd_enrich_features.go
    title: CLI Enrich Features Command Handler
    author: process:pipeline
    last_modified: 2026-09-03T14:00:00Z
  - id: batch-extractor
    resource: /internal/rpg/extractor_batch.go
    title: Vertex AI Batch API Extraction Engine
    author: process:pipeline
    last_modified: 2026-09-03T14:00:00Z
  - id: cluster-engine
    resource: /internal/rpg/cluster_semantic.go
    title: K-Means++ Semantic Clustering Pipeline
    author: process:pipeline
    last_modified: 2026-09-03T14:00:00Z
---

# Phase 3: Semantic RPG Intent Construction

## 1. Overview & Operational Contract

* **CLI Command:** `graphdb enrich-features [--batch] [--resume] [--gcs-bucket BUCKET]`[^cmd-enrich-features]
* **Inputs:** Neo4j database populated with physical `Function` nodes.
* **Outputs:** 
  1. Enriched `Function` nodes containing `atomic_features`, `is_volatile`, and 768d `embedding`.
  2. New `Domain` and `Feature` nodes linked via `PARENT_OF` and `IMPLEMENTS` relationships.
* **Dependencies:** Google Cloud Project credentials (`GOOGLE_CLOUD_PROJECT`), Vertex AI Gemini models (`gemini-embedding-001` and generative models).

Phase 3 is the semantic intelligence engine of the GraphDB Skill, lifting raw syntax into human- and agent-readable business concepts.[^cluster-engine]

---

## 2. Four Sub-Step Sequential Pipeline

Phase 3 executes four sub-steps sequentially. All sub-steps operate through database-backed resumable loops: they query for unprocessed nodes, process a chunk, write results back to Neo4j, and repeat.

```mermaid
flowchart TD
    subgraph S3A ["Sub-step 3a: Atomic Extraction"]
        A1["Query: Function.atomic_features IS NULL"] --> A2["Slice Code Lines from Disk"]
        A2 --> A3["Gemini: Extract Verb-Object & Volatility"]
        A3 --> A4["Update Function: atomic_features, is_volatile"]
    end

    subgraph S3B ["Sub-step 3b: Vector Embeddings"]
        B1["Query: Function.embedding IS NULL"] --> B2["Prioritize Text via NodeToText()"]
        B2 --> B3["Vertex AI: gemini-embedding-001 (768d)"]
        B3 --> B4["Write Function.embedding"]
    end

    subgraph S3C ["Sub-step 3c: Semantic Clustering"]
        C1["Purge Stale Domain/Feature Nodes"] --> C2["Calculate K = clamp(sqrt(files/5), 5, 50)"]
        C2 --> C3["K-Means++ Centroid Seeding"]
        C3 --> C4["Iterative Cosine Assignment Convergence"]
        C4 --> C5["LCA Filesystem Grounding"]
    end

    subgraph S3D ["Sub-step 3d: Generative Summarization"]
        D1["Query: Feature/Domain.description IS NULL"] --> D2["Gemini: Generate Architectural Role Summary"]
        D2 --> D3["Persist Domain / Feature Descriptions"]
    end

    S3A --> S3B
    S3B --> S3C
    S3C --> S3D

    classDef step fill:#e3f2fd,stroke:#1565c0,stroke-width:1px;
    class S3A,S3B,S3C,S3D step;
```

---

## 3. Asynchronous Split-Phase Mode (Vertex AI Batch API)

Processing tens of thousands of legacy functions through real-time REST APIs can lead to rate limits and long execution times. GraphDB provides a native **Split-Phase Batch Workflow** using Vertex AI Batch Prediction:[^batch-extractor]

```mermaid
sequenceDiagram
    autonumber
    actor Dev as Developer / CI
    participant CLI as GraphDB CLI
    participant GCS as Google Cloud Storage
    participant VAI as Vertex AI Batch API
    participant N4J as Neo4j 5.x Database

    Dev->>CLI: graphdb build-all --batch --gcs-bucket gs://my-bucket
    CLI->>N4J: Ingest & Import Physical CPG
    CLI->>N4J: Scan functions lacking atomic_features
    CLI->>GCS: Upload batch_requests.jsonl
    CLI->>VAI: Submit BatchJob (gemini-3.1-flash-lite)
    CLI->>N4J: Record BatchJob node (ID, status: PENDING)
    CLI-->>Dev: Build paused safely. Check back later.

    Note over VAI: Asynchronous cloud execution (No rate-limit stalls)

    Dev->>CLI: graphdb build-all --resume
    CLI->>N4J: Read active BatchJob node
    CLI->>VAI: Poll BatchJob status
    VAI-->>CLI: Status: SUCCEEDED
    CLI->>GCS: Download batch_responses.jsonl
    CLI->>N4J: Parse JSONL via ParseLLMJSON and commit atomic_features
    CLI->>CLI: Run Embeddings, Clustering, History, Contamination
    CLI-->>Dev: Graph build complete!
```

### Key Technical Hardening in Batch Parsing
* **Markdown Container Stripping:** Responses from generative models often include markdown code fences (````json ... ````). The `ParseLLMJSON()` helper cleans these fences to prevent unmarshalling failures.
* **Dual Schema Acceptance:** Supports both the current structured object format (`{"descriptors": [...], "is_volatile": true}`) and legacy descriptor arrays for backwards compatibility.
* **UTC Timestamp Normalization:** Converts `time.Now()` to explicit UTC (`time.Now().UTC()`) before writing to Neo4j to avoid `tz_id` serialization errors.

---

## 4. Sub-step Details

### Sub-step 3a: Atomic Extraction
* Slices exact function code from disk using the stored `file`, `start_line`, and `end_line` properties.
* Instructs the LLM to output 1–5 normalized verb-object descriptors (e.g., `authenticate-user`, `charge-credit-card`) and an `is_volatile` boolean.

### Sub-step 3b: Vector Embeddings
* Converts each function to text via `NodeToText()`, giving strict priority to `atomic_features` over raw function names.
* Generates 768-dimensional float32 vectors using `gemini-embedding-001` and commits them to Neo4j.

### Sub-step 3c: K-Means++ Clustering
* Cleans up existing `Feature` and `Domain` topology to guarantee idempotent builds.
* Computes optimal cluster count $K = \text{clamp}(\lfloor \sqrt{\text{fileCount}/5} \rfloor, 5, 50)$.
* Uses K-Means++ initialization to select maximally distant seeds across latent space.
* Assigns functions via Cosine Distance until centroid movement falls below convergence thresholds.
* Resolves directory locality using Lowest Common Ancestor (LCA) algorithms.

### Sub-step 3d: Summarization
* Queries for `Domain` and `Feature` nodes lacking descriptions.
* Generates concise, architectural summaries describing the purpose and boundaries of each logical group.

---

## 5. CLI Flags Reference

| Flag | Default | Description |
| :--- | :--- | :--- |
| `--batch` | `false` | Enable asynchronous Vertex AI Batch API mode. |
| `--resume` | `false` | Check status of active Batch API job and resume pipeline. |
| `--gcs-bucket` | `""` | GCS bucket for staging batch JSONL files (falls back to `GEMINI_BATCH_GCS_BUCKET` in `.env`). |
| `--concurrency`| `5` | Number of concurrent inline LLM requests when not using batch mode. |

[^cmd-enrich-features]: [`cmd_enrich_features.go`](file:///home/jasondel/dev/graphdb-skill/cmd/graphdb/cmd_enrich_features.go)
[^batch-extractor]: [`extractor_batch.go`](file:///home/jasondel/dev/graphdb-skill/internal/rpg/extractor_batch.go)
[^cluster-engine]: [`cluster_semantic.go`](file:///home/jasondel/dev/graphdb-skill/internal/rpg/cluster_semantic.go)
