---
type: Deep Dive
title: "Monolith Recovery: Modernizing 20-Year-Old Spaghetti Code"
description: Architectural playbooks and graph-driven recipes for decomposing God classes, neutralizing global state, and breaking cyclic dependencies using the Strangler Fig pattern.
tags: [deep-dives, monolith, legacy-code, god-class, global-state, strangler-fig, refactoring]
status: stable
generated: { by: "antigravity/documenter-agent", at: "2026-09-03T14:30:00Z" }
verified: { by: "human:jasondel@google.com", at: "2026-09-03T14:30:00Z" }
sources:
  - id: feathers-suite
    resource: /architecture/feathers-modernization.md
    title: The Feathers Modernization & Risk Suite
    author: process:pipeline
    last_modified: 2026-09-03T14:00:00Z
  - id: seam-queries
    resource: /guides/query-catalog.md
    title: Query Engine Catalog & Capabilities Reference
    author: process:pipeline
    last_modified: 2026-09-03T14:00:00Z
---

# Monolith Recovery: Modernizing 20-Year-Old Spaghetti Code

## 1. The Anatomy of Legacy Degeneration

Enterprise systems surviving over decades inevitably accumulate severe architectural debt:[^feathers-suite]
1. **The God Class / Omniscient Module:** Single files spanning 5,000–15,000 lines handling UI events, SQL queries, business math, and session caching.
2. **Global Mutable State:** Global structs, singletons, or static variables mutated across hundreds of functions without synchronization.
3. **Cyclic Entanglements:** Subsystem A calls Subsystem B, which in turn calls Subsystem A, making isolated testing mathematically impossible.

This guide details how engineers and AI agents use GraphDB Skill capabilities to systematically untangle these pathologies.[^seam-queries]

---

## 2. Playbook 1: Decomposing the God Class

```mermaid
flowchart TD
    A["5,000-Line God Class\n(e.g., OrderManager.cpp)"] --> B["Step 1: Run Semantic Seam Query\ngraphdb query -type semantic-seams -target OrderManager"]
    B --> C["Step 2: Inspect Low-Similarity Pairs\nFunctions with Cosine Similarity < 0.5"]
    C --> D["Step 3: Discover Latent Sub-Domains\nCluster A: Tax Calculation\nCluster B: Inventory Reservation\nCluster C: Email Dispatch"]
    D --> E["Step 4: Extract Distinct Classes\nOrderTaxService, InventoryClient, OrderNotifier"]
    E --> F["Step 5: Verify Blast Radius with What-If\ngraphdb query -type what-if -target OrderManager"]

    classDef proc fill:#e3f2fd,stroke:#1565c0,stroke-width:1px;
    classDef bad fill:#ffebee,stroke:#c62828,stroke-width:2px;
    classDef good fill:#e8f5e9,stroke:#2e7d32,stroke-width:2px;

    class A bad;
    class B,C,D,E proc;
    class F good;
```

### Execution Recipe
1. **Identify SRP Violations:** Run `graphdb query -type semantic-seams -target <ClassID>`. The engine compares vector embeddings between all member functions, flagging pairs with near-zero conceptual alignment.
2. **Group by Semantic Affinity:** Functions naturally cluster around distinct business entities (e.g., tax calculation vs. notification sending).
3. **Extract Cohesive Services:** Move clustered functions into dedicated classes with well-defined single responsibilities.

---

## 3. Playbook 2: Taming Shared Mutable Global State

Global state is the primary cause of non-linear regressions in legacy code:

```mermaid
graph LR
    subgraph Danger ["Uncontrolled Global Mutation"]
        FnA["ProcessPayment()"] -->|Mutates| G[("g_AppState Global")]
        FnB["GenerateInvoice()"] -->|Reads| G
        FnC["PrintReceipt()"] -->|Mutates| G
    end

    subgraph Solution ["Dependency Inversion Seam"]
        FnA2["ProcessPayment()"] --> Svc["IAppStateService (Interface)"]
        FnB2["GenerateInvoice()"] --> Svc
        FnC2["PrintReceipt()"] --> Svc
        Svc --> Impl["ThreadSafeAppState (Injected Singleton)"]
    end

    classDef bad fill:#ffebee,stroke:#c62828,stroke-width:2px;
    classDef good fill:#e8f5e9,stroke:#2e7d32,stroke-width:2px;

    class FnA,FnB,FnC,G bad;
    class FnA2,FnB2,FnC2,Svc,Impl good;
```

### Execution Recipe
1. **Locate All Access Points:** Run `graphdb query -type globals -target <GlobalID>`. This returns every function reading or modifying the global state across the entire codebase.
2. **Separate Readers from Writers:** Categorize callers into read-only consumers versus mutating producers.
3. **Wrap in Interface Seam:** Encapsulate the global variable inside a managed service interface (`IAppStateService`). Inject this interface via constructor parameters, instantly enabling unit test mocks.

---

## 4. Playbook 3: The Strangler Fig Modernization Cycle

The **Strangler Fig Pattern** incrementally replaces parts of a legacy system until the old implementation is completely decommissioned.

```mermaid
sequenceDiagram
    autonumber
    actor Dev as Modernization Engineer / Agent
    participant GDB as GraphDB Skill
    participant Code as Legacy Monolith
    participant NewSvc as New Cloud Microservice

    Dev->>GDB: graphdb query -type dual-lens-seams -limit 1
    GDB-->>Dev: Seam: OrderService::AuthorizePayment (Cut-Edges: 2, PinchScore: 84)
    Dev->>GDB: graphdb query -type what-if -target OrderService::AuthorizePayment
    GDB-->>Dev: Impact: 28 upstream callers, 0 cyclic side-effects
    Dev->>Code: Extract IPaymentGateway interface at cut-edge
    Dev->>NewSvc: Implement IPaymentGateway calling modern Payment API
    Dev->>Code: Inject IPaymentGateway into OrderService
    Dev->>GDB: graphdb query -type test-context -target OrderService
    GDB-->>Dev: Tests to run: [OrderTest_Auth, OrderTest_Refund]
    Dev->>Code: Run unit tests with mock gateway -> PASS
```

1. **Find Highest-ROI Seam:** Query `dual-lens-seams` to identify boundaries with maximum caller protection and minimal cut-edges ($\le 4$).
2. **Simulate Blast Radius:** Execute `what-if` on the candidate seam function to verify that upstream callers are cleanly bounded.
3. **Introduce Interface:** Extract an interface at the exact cut-edge functions.
4. **Implement Modern Service:** Deploy the new implementation (e.g., modern microservice or cloud function) fulfilling the interface contract.
5. **Verify with Linked Tests:** Run `test-context` to identify and execute the exact test suite verifying the boundary.

[^feathers-suite]: [`feathers-modernization.md`](file:///architecture/feathers-modernization.md)
[^seam-queries]: [`query-catalog.md`](file:///guides/query-catalog.md)
