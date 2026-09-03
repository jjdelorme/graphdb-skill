# Ingestion & Modernization Pipeline

The complete six-phase data lifecycle transitioning raw polyglot source code into a queryable knowledge graph.

## Pipeline Phases

* [Phase 1: Deterministic Structural Ingestion](phase1-ingestion.md) - Offline Tree-sitter AST parsing, concurrent worker pools, test tagging, and streaming JSONL emission.
* [Phase 2: Graph Loading & Index Persistence](phase2-persistence.md) - High-throughput ingestion of JSONL streams into Neo4j 5.x using transactional UNWIND batches and vector indexes.
* [Phase 3: Semantic RPG Intent Construction](phase3-rpg-enrichment.md) - LLM atomic extraction, Vertex AI Batch API workflows, vector embedding generation, and K-Means++ clustering.
* [Phase 4: Contamination & Risk Propagation](phase4-contamination.md) - Upward volatility flood-fill, distance decay calculation, and Pinch Point identification.
* [Phase 5: Temporal History & Test Linkage](phase5-temporal-tests.md) - Git commit log churn, co-change coupling, and convention-based unit test linkage.
* [Phase 6: Dual-Lens Topological Partitioning](phase6-topology-enrichment.md) - CPM Leiden execution, Two-Tier Hub Suppression, BPR classification, and Dual-Lens seam ranking.
