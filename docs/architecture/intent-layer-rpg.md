---
type: Architecture Layer
title: Semantic Intent Layer (Repository Planning Graph)
description: Theoretical foundation, atomic verb-object decomposition, vector embedding pipeline, and K-Means++ clustering for the Repository Planning Graph (RPG).
tags: [architecture, rpg, intent-layer, embeddings, k-means, clustering, gemini]
status: stable
generated: { by: "antigravity/documenter-agent", at: "2026-09-03T14:30:00Z" }
verified: { by: "human:jasondel@google.com", at: "2026-09-03T14:30:00Z" }
sources:
  - id: rpg-extractor
    resource: /internal/rpg/extractor.go
    title: LLM Atomic Feature Extractor
    author: process:pipeline
    last_modified: 2026-09-03T14:00:00Z
  - id: rpg-cluster
    resource: /internal/rpg/cluster_semantic.go
    title: Semantic K-Means++ Clustering Engine
    author: process:pipeline
    last_modified: 2026-09-03T14:00:00Z
  - id: rpg-embedder
    resource: /internal/rpg/embedder.go
    title: Vertex AI Vector Embedding Generator
    author: process:pipeline
    last_modified: 2026-09-03T14:00:00Z
---

# Semantic Intent Layer (Repository Planning Graph)

## 1. Motivation: The "Why" vs. The "How"

In large legacy codebases, the physical organization of code (files, classes, folders) rarely reflects the actual business domains or developer intent.[^rpg-extractor] Over decades of maintenance:
* Single files grow into 5,000-line "God classes" spanning payment processing, reporting, and database queries.
* Feature logic becomes scattered across dozens of unrelated directories.
* Names of functions (e.g., `ProcessData2()`, `HandleInput()`) carry almost zero domain semantics.

To address this, the GraphDB Skill implements the **Repository Planning Graph (RPG)** framework.[^rpg-cluster] The RPG creates an **Intent Layer** that abstracts above physical files to group code into coherent business domains and features based on semantic meaning:

```mermaid
graph TD
    %% --- RPG Nodes ---
    subgraph Intent_Layer ["RPG Intent Layer (Business 'Why')"]
        D1["Domain: Billing & Payments"]
        F1["Feature: Credit Card Authorization"]
        F2["Feature: Subscription Invoicing"]
        
        D2["Domain: Identity & Access"]
        F3["Feature: JWT Token Lifecycle"]
    end

    %% --- Physical Nodes ---
    subgraph Physical_Layer ["CPG Physical Layer (Code 'How')"]
        Fn1["ValidateCard() in /legacy/payment.cpp"]
        Fn2["ChargeStripe() in /api/stripe.cpp"]
        Fn3["GenerateInvoice() in /cron/billing.cs"]
        Fn4["SignToken() in /auth/jwt.py"]
    end

    %% --- Intent Edges ---
    D1 -- "PARENT_OF" --> F1
    D1 -- "PARENT_OF" --> F2
    D2 -- "PARENT_OF" --> F3

    %% --- Bridge Edges ---
    Fn1 -- "IMPLEMENTS" --> F1
    Fn2 -- "IMPLEMENTS" --> F1
    Fn3 -- "IMPLEMENTS" --> F2
    Fn4 -- "IMPLEMENTS" --> F3

    %% --- Styling ---
    classDef rpg fill:#f3e5f5,stroke:#7b1fa2,stroke-width:2px;
    classDef code fill:#e8f5e9,stroke:#2e7d32,stroke-width:2px;

    class D1,D2,F1,F2,F3 rpg;
    class Fn1,Fn2,Fn3,Fn4 code;
```

---

## 2. Four-Stage Construction Pipeline

The Intent Layer is constructed in Phase 3 of the build workflow through four sequential, resumable sub-steps:

```mermaid
flowchart TD
    A[("Neo4j CPG (Functions)")] -->|Sub-step 3a| B["Atomic Feature Extraction\n(LLM: Verb-Object Descriptors + Volatility)"]
    B -->|Sub-step 3b| C["Dense Vector Embedding\n(gemini-embedding-001, 768d)"]
    C -->|Sub-step 3c| D["K-Means++ Semantic Clustering\n(Centroid Seeding & Cosine Convergence)"]
    D -->|Sub-step 3d| E["Generative Naming & Summarization\n(Domain / Feature Topology)"]
    E --> F[("Neo4j Enriched Graph")]

    classDef proc fill:#e3f2fd,stroke:#1565c0,stroke-width:1px;
    classDef store fill:#f3e5f5,stroke:#7b1fa2,stroke-width:2px;
    class B,C,D,E proc;
    class A,F store;
```

---

## 3. Sub-step 3a: Atomic Feature Extraction

Rather than clustering raw function names or comments, the engine sends the actual implementation of each function to a generative model ([`internal/rpg/extractor.go`](file:///home/jasondel/dev/graphdb-skill/internal/rpg/extractor.go)).

### 3.1 Prompt Contract & Output Schema
The LLM evaluates the sliced function code and returns a structured JSON payload:
```json
{
  "descriptors": [
    "credit-card payment authorization",
    "fraud risk assessment",
    "stripe gateway integration"
  ],
  "is_volatile": true
}
```

### 3.2 Key Design Principles
1. **Verb-Object Decomposition:** The prompt strictly enforces a domain-first (Noun-Verb or Verb-Object) format. For example, `credit-card validation` rather than generic `validate`. This ensures functions cluster by the business entity they manipulate rather than generic programming activities.
2. **Multi-Descriptor Preservation for Legacy God Functions:** Legacy methods frequently perform multiple operations. The extractor outputs 1 to 5 descriptors per function. When vectorized, a God function sits at the intersection of those concepts in latent space rather than being artificially forced into a single bucket.
3. **External Volatility Seeding:** The LLM inspects whether the function performs database I/O, network calls, filesystem operations, or UI rendering, tagging it with `is_volatile: true`. This boolean flag seeds the downstream Phase 4 Contamination Analysis.
4. **Context Window Protection:** Functions exceeding 4,000 characters are safely sliced to prevent LLM context exhaustion.

---

## 4. Sub-step 3b: Dense Vector Embedding

Once atomic descriptors exist, the engine vectorizes each function into latent space ([`internal/rpg/embedder.go`](file:///home/jasondel/dev/graphdb-skill/internal/rpg/embedder.go)).[^rpg-embedder]

### 4.1 Embedding Standardization
* **Model:** `gemini-embedding-001` via Vertex AI.
* **Dimensions:** Standardized at **768 dimensions** across the ecosystem.
* **Index:** Cosine similarity vector index registered in Neo4j (`Function.embedding`).

### 4.2 Why Text Prioritization (`NodeToText`) Matters
Bare function names like `handleReq()` produce low-information embeddings that cluster with other generically named functions. The `NodeToText()` mapper prioritizes:
1. `atomic_features` (LLM-extracted verb-object descriptors)
2. Normalized function signature and parameters
3. Fallback to raw function name only if unextracted

---

## 5. Sub-step 3c: K-Means++ Semantic Clustering

The engine discovers natural architectural groupings by clustering function vectors ([`internal/rpg/cluster_semantic.go`](file:///home/jasondel/dev/graphdb-skill/internal/rpg/cluster_semantic.go)).

### 5.1 Dynamic Domain Sizing
The optimal number of top-level domains $K$ scales dynamically based on the total number of files in the codebase:
$$K = \text{clamp}\left(\left\lfloor \sqrt{\frac{\text{fileCount}}{5}} \right\rfloor, 5, 50\right)$$
This formula prevents small projects from fragmenting into dozens of tiny domains while ensuring million-line monoliths receive sufficient architectural separation.

### 5.2 K-Means++ Centroid Seeding
Standard K-Means is notorious for poor convergence when centroids initialize near one another. K-Means++ guarantees initial centroids are chosen with a probability proportional to their squared distance from existing centers:
$$P(x) = \frac{D(x)^2}{\sum_{x' \in X} D(x')^2}$$
This forces initial seeds to spread across distinct semantic territories (e.g., networking, database access, user authentication, UI dispatch).

### 5.3 File-Grounded Lowest Common Ancestor (LCA)
Pure vector clustering can occasionally group syntactically similar but physically disparate functions. The engine grounds clusters in the physical directory hierarchy using Lowest Common Ancestor (LCA) path resolution, ensuring discovered features maintain actionable file locality.

---

## 6. Sub-step 3d: Generative Naming & Summarization

Once vector clusters converge, the functions belonging to each cluster are passed to an LLM:
1. **Domain Naming:** The model assigns human-readable titles (e.g., `Authentication & Credential Management`, `Billing & Invoicing Engine`).
2. **Architectural Summarization:** Generates paragraph-length summaries stored on `Domain.description` and `Feature.description` explaining the subsystem's architectural role, dependencies, and business responsibilities.
3. **Fail-Fast Safety:** If the naming model encounters rate limits or errors, the pipeline halts immediately with an explicit error rather than injecting placeholder names like `Domain-UUID-1234`.

[^rpg-extractor]: [`extractor.go`](file:///home/jasondel/dev/graphdb-skill/internal/rpg/extractor.go)
[^rpg-cluster]: [`cluster_semantic.go`](file:///home/jasondel/dev/graphdb-skill/internal/rpg/cluster_semantic.go)
[^rpg-embedder]: [`embedder.go`](file:///home/jasondel/dev/graphdb-skill/internal/rpg/embedder.go)
