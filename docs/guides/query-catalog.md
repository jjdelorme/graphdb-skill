---
type: Guide
title: Query Engine Catalog & Capabilities Reference
description: Exhaustive operational catalog of all 17 graph query types supported by graphdb query, including arguments, Cypher mechanics, and JSON schemas.
tags: [guides, queries, catalog, cypher, blast-radius, seams, impact]
status: stable
generated: { by: "antigravity/documenter-agent", at: "2026-09-03T14:30:00Z" }
verified: { by: "human:jasondel@google.com", at: "2026-09-03T14:30:00Z" }
sources:
  - id: query-dispatcher
    resource: /cmd/graphdb/cmd_query.go
    title: Query CLI Handler & Flag Parser
    author: process:pipeline
    last_modified: 2026-09-03T14:00:00Z
  - id: neo4j-queries
    resource: /internal/query/neo4j.go
    title: Neo4j Cypher Implementation of Core Queries
    author: process:pipeline
    last_modified: 2026-09-03T14:00:00Z
  - id: topology-queries
    resource: /internal/query/neo4j_topology.go
    title: Structural Community & Dual-Lens Queries
    author: process:pipeline
    last_modified: 2026-09-03T14:00:00Z
---

# Query Engine Catalog & Capabilities Reference

## 1. Overview & Invocation Format

The GraphDB Skill exposes 17 specialized query capabilities tailored for architectural exploration, refactoring planning, and autonomous agent navigation:[^query-dispatcher]

```bash
graphdb query -type <type-name> [-target <target-id>] [-limit <n>] [-depth <d>] [-summary]
```

All queries output strictly formatted JSON to standard output, making them easy to consume in scripts, pipe to `jq`, or inject directly into LLM agent contexts.[^neo4j-queries]

---

## 2. Structural & Architectural Discovery Queries

### 2.1 `search-features`
* **Purpose:** Performs dense vector similarity search across `Feature` nodes using 768-dimensional embeddings to find relevant architectural features.
* **Arguments:** `-target <search-query-string>`, `[-limit <int>]`
* **Example:**
  ```bash
  graphdb query -type search-features -target "authorize credit card payment" -limit 3
  ```
* **Output Format:**
  ```json
  [
    {
      "id": "feature_billing_auth_01",
      "name": "Payment Authorization",
      "domain": "Billing & Payments",
      "score": 0.892,
      "description": "Handles tokenized credit card authorization through external gateways."
    }
  ]
  ```

---

### 2.2 `explore-domain`
* **Purpose:** Traverses the Intent Layer hierarchy starting from a top-level `Domain`, down through its `Feature` nodes, into underlying `Function` implementations.
* **Arguments:** `-target <domain-name-or-id>`
* **Example:**
  ```bash
  graphdb query -type explore-domain -target "Identity & Access"
  ```

---

### 2.3 `neighbors`
* **Purpose:** Returns 1st-degree incoming and outgoing dependencies for a specific code entity (e.g., who calls this function, and what does it call?).
* **Arguments:** `-target <node-id>`, `[-limit <int>]`, `[-summary]`
* **Example:**
  ```bash
  graphdb query -type neighbors -target "OrderProcessor::SubmitOrder" -limit 10
  ```

---

### 2.4 `impact`
* **Purpose:** Computes the transitive upstream caller blast radius. Identifies every function, class, and service that will be affected if the target function is modified or deleted.
* **Arguments:** `-target <node-id>`, `[-depth <int>]` (Default depth: `3`)
* **Example:**
  ```bash
  graphdb query -type impact -target "UserAuth::ValidateToken" -depth 4
  ```

---

### 2.5 `hybrid-context`
* **Purpose:** Fuses deterministic structural call graphs with semantic vector similarity, providing AI agents with the ideal 360-degree context window for a given task.
* **Arguments:** `-target <node-id>`
* **Example:**
  ```bash
  graphdb query -type hybrid-context -target "OrderService::Checkout"
  ```

---

## 3. Modernization & Feathers Suite Queries

### 3.1 `seams` (Pinch Points)
* **Purpose:** Identifies structural Pinch Points ($\text{Fan-In} \times \text{Volatile Fan-Out}$) where dependency inversion yields the highest unit-testing return.
* **Arguments:** `[-limit <int>]`
* **Example:**
  ```bash
  graphdb query -type seams -limit 5
  ```

---

### 3.2 `semantic-seams` (SRP Divergence)
* **Purpose:** Flags Single Responsibility Principle (SRP) violations within individual classes or files where member functions exhibit semantic cosine similarity $< 0.5$.
* **Arguments:** `[-target <file-or-class-id>]`, `[-limit <int>]`
* **Example:**
  ```bash
  graphdb query -type semantic-seams -limit 10
  ```

---

### 3.3 `globals`
* **Purpose:** Identifies all functions reading or mutating global state (`USES_GLOBAL` edges), uncovering hidden side-channel coupling.
* **Arguments:** `[-target <global-variable-id>]`, `[-limit <int>]`
* **Example:**
  ```bash
  graphdb query -type globals -limit 20
  ```

---

### 3.4 `what-if`
* **Purpose:** Simulates the structural blast radius of removing or replacing a node, reporting severed downstream edges, broken upstream callers, and contamination reduction.
* **Arguments:** `-target <node-id>`
* **Example:**
  ```bash
  graphdb query -type what-if -target "LegacyDatabaseDriver::ExecuteRawSql"
  ```

---

## 4. Dual-Lens Topology & Leiden Queries

### 4.1 `communities`
* **Purpose:** Lists structural communities discovered by the CPM Leiden algorithm, including member counts, internal edge density, and top functions.[^topology-queries]
* **Arguments:** `[-limit <int>]`
* **Example:**
  ```bash
  graphdb query -type communities -limit 10
  ```

---

### 4.2 `dual-lens-seams`
* **Purpose:** The premier refactoring query. Surfaces candidate seams ranked by Actionable Seam Score:
  $$\text{Score} = \frac{\text{Internal Fan-In} \times \text{Volatile Fan-Out}}{\text{Cut-Edges} + 1}$$
* **Arguments:** `[-limit <int>]`
* **Example:**
  ```bash
  graphdb query -type dual-lens-seams -limit 5
  ```
* **Output Format:**
  ```json
  [
    {
      "seam_id": "seam_orders_to_billing",
      "community_from": "comm_orders",
      "community_to": "comm_billing",
      "actionable_score": 84.0,
      "internal_fan_in": 28,
      "volatile_fan_out": 6,
      "cut_edges": 1,
      "functions": ["OrderManager::AuthorizePayment"]
    }
  ]
  ```

---

### 4.3 `divergence`
* **Purpose:** Compares physical CPM Leiden communities with semantic RPG domains to flag **Leaky Boundaries** and **Fragmented Features**.
* **Arguments:** `[-limit <int>]`
* **Example:**
  ```bash
  graphdb query -type divergence -limit 10
  ```

---

### 4.4 `topology-graph`
* **Purpose:** Generates a complete D3-compatible node-link JSON payload with convex hull community metadata used by the Web Visualizer.
* **Arguments:** `[-limit <int>]`
* **Example:**
  ```bash
  graphdb query -type topology-graph > web_graph.json
  ```

---

## 5. Source & Code Inspection Queries

### 5.1 `fetch-source`
* **Purpose:** Slices and returns the exact source code lines for a given node directly from the local disk using stored line numbers.
* **Arguments:** `-target <function-or-class-id>`
* **Example:**
  ```bash
  graphdb query -type fetch-source -target "OrderProcessor::SubmitOrder"
  ```

---

### 5.2 `locate-usage`
* **Purpose:** Finds all call sites and references to a specific function or variable across the entire repository.
* **Arguments:** `-target <symbol-name>`
* **Example:**
  ```bash
  graphdb query -type locate-usage -target "g_SessionToken"
  ```

---

### 5.3 `test-context`
* **Purpose:** Returns all automated unit and integration tests that verify the target function via `[:TESTS]` relationships.
* **Arguments:** `-target <function-id>`
* **Example:**
  ```bash
  graphdb query -type test-context -target "BillingEngine::CalculateTax"
  ```

---

### 5.4 `search-similar`
* **Purpose:** Uses vector cosine similarity to find semantically equivalent functions across the codebase, identifying copy-pasted or redundant logic.
* **Arguments:** `-target <function-id>`, `[-limit <int>]`
* **Example:**
  ```bash
  graphdb query -type search-similar -target "LegacyAuth::ValidateCookie" -limit 5
  ```

[^query-dispatcher]: [`cmd_query.go`](file:///home/jasondel/dev/graphdb-skill/cmd/graphdb/cmd_query.go)
[^neo4j-queries]: [`neo4j.go`](file:///home/jasondel/dev/graphdb-skill/internal/query/neo4j.go)
[^topology-queries]: [`neo4j_topology.go`](file:///home/jasondel/dev/graphdb-skill/internal/query/neo4j_topology.go)
