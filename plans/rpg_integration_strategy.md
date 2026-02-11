# RPG Integration Strategy: Bridging Intent and Implementation

**Status:** Revised
**Based on:** Research Papers `RPG.pdf`, `RPG-Encoder.pdf`
**Target:** GraphDB Skill (`.gemini/skills/graphdb`)

## 1. Executive Summary

The **Repository Planning Graph (RPG)** framework introduces a "missing link" in automated software engineering: a **Latent Functional Hierarchy** that bridges high-level user intent (Proposal) with low-level code implementation (Artifacts).

While our current **GraphDB Skill** excels at **Static Analysis** (Dependency Views) and **Vector Similarity** (Flat Semantic Views), it lacks this hierarchical "Intent View". Integrating RPG principles will transform the skill from a "Code Navigator" into a "Domain Expert," allowing agents to reason about *features* and *capabilities* before diving into files.

## 2. Analysis of RPG Framework

The RPG framework solves the "Reasoning Disconnect" by maintaining two synchronized views in a single graph:

### 2.1 The Dual-View Architecture
1.  **Functional View (The Intent Layer)**
    *   **Nodes:** `Feature` (Abstract capabilities, e.g., "User Authentication", "Retry Logic").
    *   **Structure:** Hierarchical (Tree). Root $\to$ Domain $\to$ Category $\to$ Feature.
    *   **Role:** Enables "Top-Down" navigation. Agents search this tree to find *where* a concept lives without grepping code.
2.  **Dependency View (The Implementation Layer)**
    *   **Nodes:** `File`, `Class`, `Function`.
    *   **Structure:** Network (Graph). Caller $\to$ Callee, Import, Inheritance.
    *   **Role:** Enables "Bottom-Up" execution tracing and impact analysis.

### 2.2 The Construction Pipeline ("Semantic Lifting")
RPG does not rely on simple vector embeddings of code. It uses a generative pipeline:
1.  **Atomic Feature Extraction:** An LLM scans code (e.g., `def login(...)`) and generates normalized feature descriptors (e.g., `["verify credentials", "issue token"]`).
2.  **Latent Architecture Recovery:** These descriptors are clustered to form the Functional Hierarchy, often revealing a logical structure that differs from the physical file structure.
3.  **Artifact Grounding:** Functional nodes are linked to physical code nodes via `IMPLEMENTS` edges.

### 2.3 Incremental Evolution
RPG handles "Stale Graphs" via **Differential Updates**:
*   **Semantic Drift:** When code changes, the system compares the *new* feature descriptors with the *old* ones.
*   **Semantic Routing:** If a function's intent changes (e.g., a utility becomes a core business rule), it is re-routed to a new parent in the Feature Hierarchy.

## 3. Gap Analysis: GraphDB vs. RPG

| Feature | Current GraphDB Skill | RPG Paradigm | The Gap |
| :--- | :--- | :--- | :--- |
| **Primary Unit** | `Function` (Code) | `Feature` (Intent) + `Function` (Code) | We lack the abstraction layer. We search raw code vectors. |
| **Search Strategy** | **Flat Vector Search** (KNN on all 75k functions). | **Hierarchical Search** (Traverse Feature Tree). | Flat search is noisy at scale. Hierarchical search is precise and context-aware. |
| **Grouping** | Ephemeral Clustering (`suggest-seams`). | Persistent Functional Hierarchy. | Our clusters vanish after the query. RPG's clusters are the map. |
| **Maintenance** | Re-parse on change. | Semantic Drift Detection. | We detect *text* changes; RPG detects *meaning* changes. |

## 4. Implementation Roadmap

To upgrade GraphDB to an RPG-class system, we propose a three-phase integration plan.

### Phase 1: The Semantic Schema (Graph Model)
**Goal:** Introduce the "Intent Layer" to the Neo4j database.

*   **New Node Label:** `(:Feature { name: string, description: string, embedding: vector, scope_path: string })`
*   **New Relationships:**
    *   `(:Feature)-[:PARENT_OF]->(:Feature)` (The Hierarchy).
    *   `(:Function)-[:IMPLEMENTS]->(:Feature)` (The Grounding).
*   **Migration:** No data loss. This layer sits *above* existing `Function` nodes.

### Phase 2: "Semantic Lifting" Pipeline
**Goal:** Populate the Feature nodes using LLMs.

*   **New Tool:** `extraction/enrich_features.js`
    *   **Input:** Batches of `Function` nodes (source code).
    *   **Process:**
        1.  **Prompt:** "Extract atomic functional features from this code (Verb-Object)."
        2.  **Clustering:** Use `ClusterService` (K-Means) on feature text embeddings to form groups.
        3.  **Summarization:** Generate names for clusters (e.g., "Data Validation").
    *   **Output:** Cypher statements to create `Feature` nodes and link them.

### Phase 3: Agentic Navigation Tools
**Goal:** Empower the Agent to use the map.

*   **Tool Upgrade:** `query_graph.js`
    *   **New Query:** `search-features`
        *   *Input:* "Authentication"
        *   *Logic:* Vector search on `Feature` nodes $\to$ Return linked `File`/`Function` nodes.
    *   **New Query:** `explore-domain`
        *   *Input:* `FeatureID`
        *   *Logic:* Return child features and sibling features (Context).

### Phase 4: Git-Driven Semantic Sync
**Goal:** Keep the map accurate.

*   **Enhance:** `scripts/sync_graph.js`
    *   When a file is modified:
        1.  Re-extract features for changed functions (generate new description).
        2.  **Alignment Check:** Check alignment with current Parent Feature (LLM verify).
        3.  **Top-Down Semantic Routing:** If misaligned, trigger Top-Down Routing (Root -> Domain -> Feature) to find a new home.

## 5. Strategic Benefits for Modernization

1.  **Safe Refactoring:** We can identify "Split Brain" features (logic scattered across modules) by seeing multiple sub-trees linking to the same disjoint files.
2.  **Onboarding:** Agents can explain *what* the system does (Functional View) without reading *how* (Implementation View).
3.  **Decomposition:** The Feature Hierarchy provides natural boundaries for microservice extraction (e.g., "Extract the whole 'Billing' feature branch").

## 6. Upgrade Strategy: Converting Legacy Graphs

For large-scale repositories where a complete re-index is prohibitively expensive (e.g., 50k+ functions already embedded), we propose a **Bottom-Up Upgrade Strategy**. This method leverages existing vectors to bootstrap the RPG structure without re-reading source code.

### 6.1 Hybrid Construction (Intent-First)
We employ a "Hybrid Construction" approach to ensure domain coherence.

1.  **Step 1: Domain Discovery**
    *   Feed file tree summary (depth 2-3) to LLM -> Generate 5-10 Top-Level Domains.
2.  **Step 2: Constrained Clustering**
    *   Run K-Means restricted within these Domains (files in `src/auth` cluster together first).
3.  **Step 3: Refined Leaf Definition**
    *   Explicitly define clusters as "Topic Nodes".
    *   Add a step where LLM examines a cluster and splits it if it's too broad (Topic vs Atomic Feature).

### 6.2 The Hybrid Refinement Loop
Once the initial Hybrid RPG is built:

*   **Lazy Refinement:** When an agent interacts with a Feature Node and finds it vague or incoherent, it flags it. The system then performs true "Semantic Lifting" (reading source code) *only* on that subtree to repair it.
*   **Incremental Pay-as-you-go:** New code added via `sync_graph.js` uses the full high-fidelity RPG pipeline, gradually increasing the graph's quality over time.

This approach allows upgrading a massive legacy graph in **minutes** (clustering) + **low tokens** (labeling) rather than days of re-processing.
