# ARIP Master Roadmap: Repository Intelligence Platform

**Status:** Active
**Vision:** Evolve the `graphdb` skill from a local Node.js script into a scalable, multi-tenant Go + Spanner platform (ARIP).

## 🌍 Strategic Campaigns

### Campaign 1: The Go Ingestor (Language Parity)
**Goal:** Replace the Node.js extraction logic with a high-performance Go binary capable of parallel parsing and embedding, with **strict parity for existing languages**.
**Status:** Planned
**Key Deliverables:**
- [ ] Standalone Go CLI (`graphdb`).
- [ ] Parallel file walker with Worker Pools.
- [ ] **Language Parity:** C#, C/C++, VB.NET, SQL, TypeScript.
- [ ] Tree-sitter integration via CGO.
- [ ] **Vertex AI Integration:** Embedding generation parity with `enrich_vectors.js`.
- [ ] **Data Parity:** JSONL output strictly matches existing schema (Nodes/Edges).
- [ ] Standardized JSONL output format via `Storage/Emitter` interface.

### Campaign 2: The Graph Query Engine (Full Query Parity)
**Goal:** Implement the "Read" side of the platform in Go, mirroring the "Write" side (Ingestor). This enables the Go binary to answer queries directly, preparing for the Spanner migration.
**Status:** Planned
**Key Deliverables:**
- [ ] `GraphProvider` Interface (FindNode, Traverse, SearchFeatures).
- [ ] `Neo4jProvider` implementation (connects to local Neo4j).
- [ ] **Full Query Parity:** Port all critical query types: `hybrid-context`, `test-context`, `impact`, `globals`, `suggest-seams`.
- [ ] Cypher Query Builder/Manager in Go.

### Campaign 3: Gemini CLI Skill Integration (The Agent Bridge)
**Goal:** Wrap the Go Binary in a Gemini CLI Skill to allow agents to invoke it directly for **both ingestion and querying**.
**Status:** Pending
**Key Deliverables:**
- [ ] Update existing JS skill (`.gemini/skills/graphdb`) to spawn the Go binary.
- [ ] **Unified Interface:** Skill delegates `extract` and `query` commands to the Go binary.
- [ ] Expose CLI flags (path, depth, output format) to the agent via tool definitions.
- [ ] Implement robust stdout/stderr capture and error handling for the agent.

### Campaign 4: The Spanner Backend (Storage Swap)
**Goal:** Establish the multi-tenant, immutable storage layer using Google Spanner Graph by swapping the storage implementation.
**Status:** Pending
**Key Deliverables:**
- [ ] Spanner Graph Schema (GQL) for RPG structure.
- [ ] **Storage Adapter:** Implement `SpannerEmitter` to replace `JSONLEmitter`.
- [ ] **Graph Provider:** Implement `SpannerProvider` to replace `Neo4jProvider`.
- [ ] Bulk Loader (JSONL -> Spanner).
- [ ] Multi-tenancy implementation (Schema Interleaving).

### Campaign 5: Cross-Platform Distribution (The Release)
**Goal:** Ship a single, zero-dependency binary for all major OSs.
**Status:** Pending
**Key Deliverables:**
- [ ] Zig-based Cross-Compilation pipeline.
- [ ] GitHub Actions release workflow.
- [ ] Automated integration tests.

### Campaign 6: The MCP Server (The Interface)
**Goal:** Expose the platform to Agents via the Model Context Protocol (MCP), enabling "Dual-View" reasoning. **(Scheduled Last)**
**Status:** Pending
**Key Deliverables:**
- [ ] MCP Protocol implementation (Stdio transport).
- [ ] "RAM Overlay" logic (Local Diff vs. Cloud Base).
- [ ] Tool implementations (`search_features`, `traverse_deps`).
