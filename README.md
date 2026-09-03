# GraphDB Skill Ecosystem

[![Go Report Card](https://goreportcard.com/badge/github.com/jjdelorme/graphdb-skill)](https://goreportcard.com/report/github.com/jjdelorme/graphdb-skill)
[![Neo4j](https://img.shields.io/badge/Neo4j-5.x%20Vector-blue.svg)](https://neo4j.com/)
[![Gemini](https://img.shields.io/badge/Model-gemini--embedding--001-orange.svg)](https://cloud.google.com/vertex-ai)
[![OKF](https://img.shields.io/badge/Docs-OKF%20v0.2-green.svg)](docs/index.md)

The **GraphDB Skill** is an enterprise-grade legacy modernization and architectural reasoning subsystem designed for the **Gemini CLI** and **Antigravity Multi-Agent Swarms**.

By transforming multi-million-line polyglot repositories (C++, C#, Java, TypeScript, Python, ASP Classic, VB.NET, and SQL) into a **Hybrid Modernization Triad**, GraphDB gives AI coding agents unprecedented spatial and architectural awareness:

1. **Lens 1: Physical Reality (CPG):** Offline, deterministic AST extraction via Tree-sitter parsers into Neo4j 5.x.
2. **Lens 2: Business Intent (RPG):** Semantic verb-object intent extraction via Gemini models and 768d vector embeddings (`gemini-embedding-001`) clustered via K-Means++.
3. **Lens 3: Dual-Lens Topology (Leiden):** Resolution-free Constant Potts Model (CPM) Leiden community detection with Two-Tier Hub Suppression, uncovering high-ROI refactoring seams and boundary divergence.

---

## 📚 Open Knowledge Catalog (OKF Documentation)

Comprehensive, definitive documentation structured according to the **Open Knowledge Format (OKF v0.2)** is available in the [`docs/`](docs/index.md) catalog:

| Section | Focus Areas & Concepts |
| :--- | :--- |
| **[Architecture](docs/architecture/index.md)** | [Hybrid Triad Overview](docs/architecture/hybrid-overview.md) • [Physical CPG Schema](docs/architecture/physical-cpg.md) • [Semantic Intent Layer (RPG)](docs/architecture/intent-layer-rpg.md) • [Dual-Lens Leiden Engine](docs/architecture/dual-lens-topology.md) • [Feathers Modernization Suite](docs/architecture/feathers-modernization.md) |
| **[Pipeline](docs/pipeline/index.md)** | [Phase 1: Ingest](docs/pipeline/phase1-ingestion.md) • [Phase 2: Import](docs/pipeline/phase2-persistence.md) • [Phase 3: RPG](docs/pipeline/phase3-rpg-enrichment.md) • [Phase 4: Contamination](docs/pipeline/phase4-contamination.md) • [Phase 5: Temporal/Tests](docs/pipeline/phase5-temporal-tests.md) • [Phase 6: Topology](docs/pipeline/phase6-topology-enrichment.md) |
| **[Algorithms](docs/algorithms/index.md)** | [Constant Potts Model (CPM)](docs/algorithms/cpm-leiden.md) • [Two-Tier Hub Suppression](docs/algorithms/two-tier-hub-suppression.md) • [Boundary Participation Ratio (BPR)](docs/algorithms/boundary-participation-ratio.md) • [Actionable Seam Ranking](docs/algorithms/actionable-seam-ranking.md) • [K-Means++ Vector Clustering](docs/algorithms/kmeans-vector-clustering.md) |
| **[Guides](docs/guides/index.md)** | [Getting Started](docs/guides/getting-started.md) • [CLI Complete Reference](docs/guides/cli-reference.md) • [17 Query Capabilities](docs/guides/query-catalog.md) • [Interactive Web Visualizer](docs/guides/web-visualizer.md) • [Multi-Agent Swarm Orchestration](docs/guides/agent-orchestration.md) |
| **[Computations](docs/computations/index.md)** | [Actionable Seam Score Computation](docs/computations/actionable-seam-score.md) • [Volatility Flood-Fill Computation](docs/computations/contamination-volatility.md) |
| **[Deep Dives](docs/deep-dives/index.md)** | [Technical Comparison: Graphify vs GraphDB](docs/deep-dives/graphify-comparison.md) • [Legacy Monolith Recovery Playbook](docs/deep-dives/legacy-monolith-recovery.md) • [100k+ Functions with Vertex AI Batch API](docs/deep-dives/vertex-batch-processing.md) |

---

## 📦 Quick Installation

You do not need to build from source to use the skill in your project. Pre-compiled binaries and agent personas are available on our [GitHub Releases](https://github.com/jjdelorme/graphdb-skill/releases).

### Linux / macOS
```bash
curl -sL https://github.com/jjdelorme/graphdb-skill/releases/latest/download/graphdb-bundle-linux.tar.gz | tar -xzv
```

### Windows (PowerShell)
```powershell
curl.exe -sL https://github.com/jjdelorme/graphdb-skill/releases/latest/download/graphdb-bundle-windows.tar.gz -o bundle.tar.gz; tar.exe -xzvf bundle.tar.gz; del bundle.tar.gz
```

---

## ⚙️ Configuration (`.env`)

Create a `.env` file in the project root:

```ini
# Neo4j Database
NEO4J_URI=bolt://localhost:7687
NEO4J_USER=neo4j
NEO4J_PASSWORD=my_secure_password

# Google Cloud & Vertex AI (Embeddings & Summaries)
GOOGLE_CLOUD_PROJECT=my-gcp-project-id
GOOGLE_CLOUD_LOCATION=us-central1
GEMINI_EMBEDDING_MODEL=gemini-embedding-001
GEMINI_GENERATIVE_MODEL=gemini-3.1-flash-lite

# Optional: GCS Staging Bucket for Batch API (10k+ functions)
GEMINI_BATCH_GCS_BUCKET=gs://my-batch-staging-bucket
```

---

## 🚀 Building & Running

### Compiling Binaries
Always build using the project Makefile (Linux & Windows cross-compilation via Zig):
```bash
make build-all
```

### Ingesting & Modernizing a Repository
```bash
# 1. Run all 6 pipeline phases end-to-end
.gemini/skills/graphdb/scripts/graphdb build-all -dir /path/to/target/monolith

# 2. Launch the embedded interactive D3 Visualizer
.gemini/skills/graphdb/scripts/graphdb serve -port 8080

# 3. Query actionable refactoring seams from the terminal
.gemini/skills/graphdb/scripts/graphdb query -type dual-lens-seams -limit 5
```

---

## 🤖 Multi-Agent Swarms (Scout & Architect)

The GraphDB Skill natively integrates with autonomous AI agents:
* **The Scout Agent ([`.gemini/agents/scout.md`](.gemini/agents/scout.md)):** High-speed codebase explorer using semantic feature queries and blast-radius lookups without burning context tokens.
* **The Architect Agent:** Automated refactoring planner that discovers low-cut-edge seams and safely partitions work across multi-agent swarms along CPM community boundaries.

For detailed swarm setup, see the [Agent Orchestration Guide](docs/guides/agent-orchestration.md).

---

## 📜 License & Governance

Distributed under the MIT License. See [Changelog](CHANGELOG.md) and [OKF Bundle Log](docs/log.md) for version history.
