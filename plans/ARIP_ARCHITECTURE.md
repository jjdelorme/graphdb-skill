# Advanced Repository Intelligence Platform (ARIP)
## Architectural Blueprint & Decision Record

**Status:** Draft / Blueprint
**Date:** February 10, 2026
**Context:** Evolution of the Gemini CLI `graphdb` skill into a scalable, multi-tenant Code Intelligence Platform.

---

## 1. Executive Summary

This document outlines the architectural pivot from a local, single-user **GraphDB Skill** (Node.js + Neo4j, located in `.gemini/skills/graphdb/`) to a robust, multi-tenant **Repository Intelligence Platform** (Golang + Google Spanner).

The primary driver for this shift is the need to support **Large-Scale Disparate Repositories** (650k+ nodes per project) and **Multi-Developer Concurrency** without imposing significant operational overhead (Docker containers, JVM tuning) on the end user. By adopting the **Repository Planning Graph (RPG)** framework and wrapping it in a **Model Context Protocol (MCP)** server, we aim to provide AI agents with a "Dual-View" understanding of code: high-level *Intent* and low-level *Implementation*.

This architectural vision is grounded in the methodologies described in **"Closing the Loop: Universal Repository Representation with RPG-Encoder"** (`plans/RPG-Encoder.pdf`) and **"RPG: A Repository Planning Graph"** (`plans/RPG.pdf`).

---

## 2. The Conceptual Shift: Adopting RPG

### 2.1 The Problem: The "Reasoning Disconnect"
Current static analysis tools (including our V1 skill implementation in `.gemini/skills/graphdb/scripts/query_graph.js`) provide a **Dependency View**: they answer "Who calls function X?". They fail to answer "Where is the authentication logic?". This forces agents to grep blindly for features, leading to high token costs and poor context. This disconnect is identified as a primary bottleneck in repository understanding (*RPG-Encoder.pdf, Introduction*).

### 2.2 The Solution: The RPG Dual-View
We are adopting the **Repository Planning Graph** architecture (*RPG-Encoder.pdf, Figure 1 & Section 3.1*), which structures the graph into two distinct but linked layers:

1.  **The Functional Graph (Intent):**
    *   **Nodes:** `Feature` (Abstract capabilities, e.g., "Retry Logic", "User Auth").
    *   **Structure:** Hierarchical Tree (*RPG-Encoder.pdf, Section 3.1 "RPG Structure"*).
    *   **Source:** Generated via "Semantic Lifting" (LLM summarization of code) (*RPG-Encoder.pdf, Phase 1*).
    *   **Role:** Enables "Top-Down" navigation.

2.  **The Dependency Graph (Implementation):**
    *   **Nodes:** `File`, `Class`, `Function`.
    *   **Structure:** Network (*RPG.pdf, Section 3.1*).
    *   **Source:** Deterministic Static Analysis (Tree-sitter).
    *   **Role:** Enables "Bottom-Up" impact analysis.

---

## 3. System Architecture

### 3.1 The "Edge-to-Cloud" Hybrid Model
We reject the pure "SaaS" model (Cloud Run) in favor of a **Tethered Binary** model to preserve the "Real-Time" developer experience.

*   **The Edge (Local Machine):**
    *   **Component:** A single static **Golang Binary** (`graphdb`).
    *   **Role:** Ingestion Pump & Query Interface.
    *   **State:** Holds the "Delta" (Uncommitted changes) in RAM to ensure instant responsiveness to file edits.
    *   **Performance:** Uses local CPU for expensive Tree-sitter parsing, avoiding cloud compute costs and network latency for file uploads.

*   **The Backend (Google Cloud):**
    *   **Component:** **Google Spanner**.
    *   **Role:** The "CDN for Code Intelligence." Stores the massive, immutable "Base Layer" of the graph (e.g., the `main` branch).
    *   **Service:** **Vertex AI**. Used for generating vector embeddings.

### 3.2 Protocol: Model Context Protocol (MCP)
The system will be exposed not as a proprietary CLI tool, but as an **MCP Server**.
*   **Why:** Decouples the logic from the specific client (Gemini CLI, Claude Desktop, IDEs).
*   **Transport:** Stdio (Standard Input/Output) over the local binary. Zero network configuration required.

---

## 4. Technology Stack Decisions

### 4.1 Language: Node.js $\to$ Golang
**Decision:** Migrate entire codebase (currently in `.gemini/skills/graphdb/*`) to Go.

*   **Reason 1: Concurrency.** Ingestion of 650k+ nodes requires massive parallelism. Go's Goroutines + Channels are vastly superior to Node.js Event Loop for CPU-bound tasks (Parsing).
*   **Reason 2: Ecosystem.** Excellent bindings for `tree-sitter` (via CGO) and first-class Google Cloud SDKs.
*   **Reason 3: Deployment.** Shipping a single static binary (`.exe` / linux binary) is superior to requiring users to have specific Node.js versions and `node_gyp` environments (which frequently break on Windows).

### 4.2 Database: Neo4j $\to$ Google Spanner
**Decision:** Migrate storage to Spanner Graph.

*   **Reason 1: Multi-Tenancy.** We need to host disparate projects (Project A, Project B) on one instance. Spanner's **Schema Interleaving** (Parent/Child tables) allows strict physical separation of data by `ProjectID` while sharing the same infrastructure.
*   **Reason 2: Management.** Managing a Neo4j Cluster for high availability is a full-time job. Spanner is fully managed.
*   **Reason 3: The "Platform" Scale.** Neo4j Community Edition is limited to one node. Spanner scales horizontally.
*   **Reason 4: SQL + Graph.** The RPG architecture requires strict typing for Feature Nodes (SQL) combined with deep traversal for dependencies (Graph). Spanner Graph offers GQL (Graph Query Language) over relational tables.

*   **Rejected Alternative: Firebase.**
    *   *Reason:* Graph traversals (Depth 2+) require N+1 read operations (Client-Side Joins). Prohibitively expensive and slow for code analysis.

---

## 5. The Concurrency Challenge: "Git-Aligned" State

**Scenario:** Developer A creates the graph. Developer B checks out the repo and modifies code locally. How do we prevent B's changes from corrupting A's view?

**Decision:** We treat the Graph like Git—**Layered Versioning**, drawing inspiration from the Differential Evolution strategy (*RPG-Encoder.pdf, Section 3.2 "RPG Evolution"*).

### 5.1 The "Base Layer" (Cloud)
*   **Source:** The canonical `main` branch.
*   **Storage:** Spanner.
*   **Update Frequency:** On CI/CD Merge (Slow, robust).
*   **Access:** Read-Only for all developers.

### 5.2 The "Delta Layer" (Edge)
*   **Source:** The developer's local uncommitted changes.
*   **Storage:** **Local RAM** (inside the Go binary).
*   **Update Frequency:** On File Save (Instant).
*   **Access:** Private to the specific developer.

### 5.3 The "Overlay" Resolution
When the Agent queries the graph:
1.  The Go Binary checks its **RAM Overlay**.
    *   *"Do I have a dirty copy of `AuthService.go`?"*
    *   **Yes:** Parse and answer from RAM.
    *   **No:** Fallback to Spanner (`main` version).
2.  **Result:** Zero corruption. Developer A sees their rename; Developer B sees their logic change.

---

## 6. Implementation Strategy

### Phase 1: The Go "Ingestor"
*   Build a standalone Go CLI that accepts a directory path (replacing the current logic in `.gemini/skills/graphdb/extraction/`).
*   Uses `worker pools` to walk the file tree.
*   Uses `go-tree-sitter` to generate nodes.
*   Uses `Vertex AI` to embed functions.
*   Outputs a JSONL dump (intermediate format).

### Phase 2: The Spanner Backend
*   Define the GQL Schema (Nodes, Edges, Projects).
*   Write the `Bulk Loader` to push JSONL to Spanner.

### Phase 3: The MCP Server
*   Implement the MCP Protocol (Stdio).
*   Implement the "Overlay Logic" (RAM vs Spanner).
*   Expose Tools: `search_features`, `traverse_deps`, `read_file`.

### Phase 4: Cross-Platform Build
*   Use **Zig** as the C-Cross-Compiler to build Windows/Linux binaries (handling the `tree-sitter` CGO dependency) from a single CI pipeline.

---

## 7. Risks & Mitigations

| Risk | Mitigation |
| :--- | :--- |
| **Latency:** Spanner queries (50ms) are slower than Localhost Neo4j (5ms). | **Parallelism.** Go can fire 50 Spanner queries concurrently. For an Agent (Chat UX), 200ms total latency is imperceptible. |
| **Complexity:** CGO (C + Go) build chains are fragile. | **Docker/Zig.** Use a standardized build container with Zig to ensure reproducible builds across OSs. |
| **Cost:** Spanner is expensive. | **Use Granular Instances.** Or, for development, use the Spanner Emulator (Local) and switch to Cloud for Prod. |
| **Staleness:** Cloud graph lags behind `main`. | **Lazy Re-indexing.** The Local Overlay covers the gap. We don't need "Real-time Cloud Sync," we just need "Real-time Local Sync." |

## 8. Conclusion

This architecture transforms the tool from a "Script" into a "Platform." By leveraging **Go** for high-performance edge processing and **Spanner** for managed multi-tenant storage, we create a system that can scale to enterprise-level repositories while maintaining the "snappy" feel of a local tool via the **RAM Overlay** strategy.

---

## 9. Primary Research Sources

The following research papers provided the foundational concepts for this architecture:

### 9.1 RPG-Encoder: Closing the Loop (`plans/RPG-Encoder.pdf`)
*   **Dual-View Graph (Functional + Dependency):**
    *   *Reference:* Figure 1 (Page 2) & Section 3.1 "RPG Structure" (Page 3).
    *   *Application:* Basis for our separation of Intent (Feature) and Implementation (Code).
*   **Semantic Lifting:**
    *   *Reference:* Section 3.1 "Phase 1: Semantic Lifting" (Page 4).
    *   *Application:* Algorithm for extracting high-level features from raw code.
*   **Latent Architecture Recovery:**
    *   *Reference:* Section 3.1 "Phase 2: Semantic Structure Reorganization" (Page 4) & Appendix A.1.2 (Page 17).
    *   *Application:* Grouping features into a hierarchy that may differ from the file system.
*   **Artifact Grounding:**
    *   *Reference:* Section 3.1 "Phase 3: Artifact Grounding" (Page 5) & Appendix A.1.3 (Page 19).
    *   *Application:* Linking the abstract Feature Tree to physical files/directories.
*   **Incremental Evolution (Differential Updates):**
    *   *Reference:* Section 3.2 "RPG Evolution" (Page 5) & Appendix A.2 (Page 21).
    *   *Application:* Basis for our "Git-Aligned" State strategy, handling drift via re-routing.

### 9.2 RPG: Repository Planning Graph (`plans/RPG.pdf`)
*   **Repository Planning Graph Structure:**
    *   *Reference:* Section 3.1 (Page 3).
    *   *Application:* Defines the node/edge schema for bridging high-level intent with low-level code.
*   **Graph-Guided Localization:**
    *   *Reference:* Section 7.3 (Page 9).
    *   *Application:* Validates the efficiency of graph-guided search over simple retrieval.