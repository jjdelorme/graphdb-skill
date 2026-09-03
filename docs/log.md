# Bundle Update Log

Chronological audit log of all modifications, additions, and updates to the GraphDB Skill Open Knowledge Catalog.

## 2026-09-03T14:35:00Z antigravity/documenter-agent

### Added
- `docs/index.md` - Master OKF v0.2 root catalog index with progressive disclosure hierarchy.
- `docs/architecture/hybrid-overview.md` - Complete architectural overview of the Hybrid Triad (CPG + RPG + Leiden).
- `docs/architecture/physical-cpg.md` - CPG schema, node labels, edge semantics, and Tree-sitter parsers.
- `docs/architecture/intent-layer-rpg.md` - Semantic Intent Layer, atomic verb-object extraction, and K-Means++ clustering.
- `docs/architecture/dual-lens-topology.md` - Dual-Lens CPM Leiden engine, Two-Tier Hub Suppression, and divergence detection.
- `docs/architecture/feathers-modernization.md` - Feathers Modernization Suite (Pinch Points, Contamination, SRP Seams, What-If).
- `docs/pipeline/phase1-ingestion.md` - Phase 1 Tree-sitter AST parsing, worker pools, and streaming JSONL emission.
- `docs/pipeline/phase2-persistence.md` - Phase 2 Neo4j loading, UNWIND batches, constraints, and vector indexes.
- `docs/pipeline/phase3-rpg-enrichment.md` - Phase 3 LLM extraction, Vertex AI Batch API, and domain clustering.
- `docs/pipeline/phase4-contamination.md` - Phase 4 Volatility flood-fill, distance decay, and risk scoring.
- `docs/pipeline/phase5-temporal-tests.md` - Phase 5 Git commit churn, co-change coupling, and test suite linkage.
- `docs/pipeline/phase6-topology-enrichment.md` - Phase 6 CPM Leiden pipeline execution, hub quarantine, and seam ranking.
- `docs/algorithms/cpm-leiden.md` - Constant Potts Model (CPM) quality function and adaptive gamma bisection search.
- `docs/algorithms/two-tier-hub-suppression.md` - Inverse-degree logarithmic damping and top 1% quarantine to prevent hairball collapse.
- `docs/algorithms/boundary-participation-ratio.md` - Boundary Participation Ratio (BPR) for shared infrastructure isolation.
- `docs/algorithms/actionable-seam-ranking.md` - Feathers Seam Score handshake formula and ROI prioritization.
- `docs/algorithms/kmeans-vector-clustering.md` - K-Means++ centroid initialization on 768d embeddings.
- `docs/guides/getting-started.md` - Setup, prerequisites, Neo4j configuration, and Makefile build instructions.
- `docs/guides/cli-reference.md` - Complete CLI reference for all commands, flags, and default options.
- `docs/guides/query-catalog.md` - Exhaustive catalog of all 17 graph query types with JSON outputs.
- `docs/guides/web-visualizer.md` - Interactive D3 force-directed visualizer, X-Ray overlay, and contamination heatmaps.
- `docs/guides/agent-orchestration.md` - Multi-agent swarm orchestration (Scout, Architect) and conflict-free community partitioning.
- `docs/computations/actionable-seam-score.md` - Attested computation defining the sanctioned Feathers Seam Score Cypher query.
- `docs/computations/contamination-volatility.md` - Attested computation defining the sanctioned Volatility Flood-Fill Cypher query.
- `docs/deep-dives/graphify-comparison.md` - Technical comparison between Microsoft Graphify (v8) and GraphDB Skill.
- `docs/deep-dives/legacy-monolith-recovery.md` - Modernization playbooks for God classes, global state, and Strangler Fig refactoring.
- `docs/deep-dives/vertex-batch-processing.md` - Scaling to 100k+ functions via Vertex AI Gemini Batch API.
