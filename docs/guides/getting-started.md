---
type: Guide
title: Getting Started with GraphDB Skill
description: Prerequisites, installation, environment configuration, Neo4j setup, and building the cross-platform CLI binaries.
tags: [guides, getting-started, setup, installation, configuration, neo4j]
status: stable
generated: { by: "antigravity/documenter-agent", at: "2026-09-03T14:30:00Z" }
verified: { by: "human:jasondel@google.com", at: "2026-09-03T14:30:00Z" }
sources:
  - id: gemini-readme
    resource: /GEMINI.md
    title: GraphDB Skill Environment & Build Guidelines
    author: human:jasondel@google.com
    last_modified: 2026-09-01T00:00:00Z
  - id: root-makefile
    resource: /Makefile
    title: Cross-Platform Makefile with Zig CGO Toolchain
    author: process:pipeline
    last_modified: 2026-09-03T14:00:00Z
---

# Getting Started with GraphDB Skill

## 1. System Prerequisites

Before configuring and building the GraphDB Skill, ensure your development environment satisfies the following requirements:[^gemini-readme]

* **Go Toolchain:** Go 1.22+ installed and available on `PATH`.
* **Node.js:** v20+ (Required for Gemini CLI skill integration).
* **Neo4j Database:** Neo4j Community Edition 5.11+ (Local or Docker) with Vector Index support.
* **Zig Compiler (One-time Setup):** Required as the cross-compilation C/C++ compiler for CGO Tree-sitter binaries on Windows from Linux.
* **Google Cloud Project:** For Vertex AI vector embeddings (`gemini-embedding-001`) and generative summaries.

---

## 2. Setting Up Zig for Cross-Compilation

Because the CLI contains CGO dependencies (`tree-sitter` grammars), building for both Linux and Windows from a Linux host requires Zig:[^root-makefile]

```bash
mkdir -p .gemini/tools && cd .gemini/tools
curl -L -O https://ziglang.org/download/0.13.0/zig-linux-x86_64-0.13.0.tar.xz
tar -xf zig-linux-x86_64-0.13.0.tar.xz
cd ../..
```

The repository `Makefile` automatically detects this local Zig installation at `.gemini/tools/zig-linux-x86_64-0.13.0/zig`.

---

## 3. Environment Configuration (`.env`)

Create a `.env` file in the project root containing your Neo4j and Vertex AI configuration:

```bash
# Neo4j Database Configuration
NEO4J_URI=bolt://localhost:7687
NEO4J_USER=neo4j
NEO4J_PASSWORD=my_secret_password

# Google Cloud & Vertex AI Configuration
GOOGLE_CLOUD_PROJECT=my-gcp-project-id
GOOGLE_CLOUD_LOCATION=us-central1

# Model Standardization (Required: 768 dimensions)
GEMINI_EMBEDDING_MODEL=gemini-embedding-001
GEMINI_GENERATIVE_MODEL=gemini-3.1-flash-lite

# Optional: GCS Bucket for Asynchronous Batch API (10k+ functions)
GEMINI_BATCH_GCS_BUCKET=gs://my-batch-staging-bucket
```

---

## 4. Building the Unified CLI

**CRITICAL RULE:** Never build directly with `go build`. Always use the Makefile targets from the project root. The Makefile automatically configures Zig, `CGO_ENABLED=1`, and target flags for both Linux and Windows simultaneously.

```bash
make build-all
```

### Build Artifacts
Upon completion, the binaries are placed directly into the skill directory:
* **Linux:** `.gemini/skills/graphdb/scripts/graphdb`
* **Windows:** `.gemini/skills/graphdb/scripts/graphdb-win.exe`

---

## 5. Running the Test Suite

Always execute tests using the Makefile to ensure environment variables and CGO flags are properly injected:

```bash
make test
```

---

## 6. End-to-End Quickstart on a Codebase

To analyze a repository from end to end:

```bash
# 1. Full automated build of all phases
.gemini/skills/graphdb/scripts/graphdb build-all -dir /path/to/target/monolith

# 2. Start the interactive Web Visualizer on port 8080
.gemini/skills/graphdb/scripts/graphdb serve -port 8080

# 3. Query high-ROI refactoring seams from the terminal
.gemini/skills/graphdb/scripts/graphdb query -type dual-lens-seams -limit 5
```

[^gemini-readme]: [`GEMINI.md`](file:///home/jasondel/dev/graphdb-skill/GEMINI.md)
[^root-makefile]: [`Makefile`](file:///home/jasondel/dev/graphdb-skill/Makefile)
