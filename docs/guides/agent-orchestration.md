---
type: Guide
title: Multi-Agent Swarm Orchestration & Personas
description: Coordinating autonomous Gemini CLI and Antigravity agents (Scout, Architect) using graph queries and conflict-free community partitioning.
tags: [guides, agents, swarms, scout, architect, antigravity, gemini-cli]
status: stable
generated: { by: "antigravity/documenter-agent", at: "2026-09-03T14:30:00Z" }
verified: { by: "human:jasondel@google.com", at: "2026-09-03T14:30:00Z" }
sources:
  - id: scout-agent
    resource: /.gemini/agents/scout.md
    title: Scout Deep Research Agent Persona
    author: human:jasondel@google.com
    last_modified: 2026-09-01T00:00:00Z
  - id: skill-def
    resource: /.gemini/skills/graphdb/SKILL.md
    title: GraphDB Skill Definition for Gemini CLI
    author: process:pipeline
    last_modified: 2026-09-03T14:00:00Z
---

# Multi-Agent Swarm Orchestration & Personas

## 1. Overview & Token Economy

In standard LLM development, agents burn hundreds of thousands of tokens grepping files and reading entire source directories into context.[^scout-agent] 

The **GraphDB Skill** transforms agents from naive text-searchers into graph-aware navigators:[^skill-def]
* **Precise Context Slicing:** An agent queries `graphdb query -type hybrid-context -target <symbol>` to receive exactly the callers, callees, and semantic features relevant to the task in under 2,000 tokens.
* **Autonomous Exploration:** Agents traverse the architecture using domain trees (`explore-domain`) without touching raw files until implementation begins.

---

## 2. Agent Personas in the Ecosystem

```mermaid
flowchart TD
    User["Developer Request"] --> Orch["Gemini CLI / Antigravity Orchestrator"]

    subgraph Agents ["Specialized Swarm Agents"]
        Scout["Scout Agent\n(Deep Codebase Researcher)"]
        Architect["Architect Agent\n(Migration & Seam Planner)"]
        Engineer["Engineer Agent\n(TDD Implementation Builder)"]
        Auditor["Auditor Agent\n(Test & Consistency Gatekeeper)"]
    end

    Orch --> Scout
    Orch --> Architect
    Scout <-->|Graph Queries| N4J[("GraphDB Neo4j Knowledge Base")]
    Architect <-->|Seam & What-If Queries| N4J
    Architect --> Engineer
    Engineer --> Auditor

    classDef agent fill:#e8f5e9,stroke:#2e7d32,stroke-width:2px;
    classDef store fill:#f3e5f5,stroke:#7b1fa2,stroke-width:2px;
    class Scout,Architect,Engineer,Auditor agent;
    class N4J store;
```

### 2.1 The Scout Agent (`scout.md`)
* **Role:** Architectural cartographer and dependency researcher.
* **Primary Queries:** `search-features`, `explore-domain`, `neighbors`, `locate-usage`, `test-context`.
* **Behavior:** When asked *"How does the system calculate taxes for international orders?"*, Scout queries `search-features`, locates the relevant `Feature` node, retrieves its implementing functions, traces their downstream database calls, and returns an exact call graph to the developer.

### 2.2 The Architect Agent (`architect.md`)
* **Role:** Modernization strategist and refactoring planner.
* **Primary Queries:** `dual-lens-seams`, `seams`, `divergence`, `what-if`.
* **Behavior:** When asked *"How can we decouple the billing engine from legacy SQL singletons?"*, Architect queries `dual-lens-seams`, discovers high-ROI interface seams ($\le 4$ cut-edges), runs `what-if` to verify blast-radius containment, and generates a concrete step-by-step refactoring plan.

---

## 3. Conflict-Free Swarm Work Partitioning

When multiple autonomous coding agents work concurrently on a legacy monolith, they frequently collide—editing the same files or creating merge conflicts.

### Partitioning Along Leiden Communities
The engine uses **CPM Structural Communities** to partition swarm assignments:

```mermaid
graph LR
    subgraph Swarm ["Autonomous Multi-Agent Swarm"]
        A1["Agent 1 (Assigned to Comm-01)"]
        A2["Agent 2 (Assigned to Comm-02)"]
    end

    subgraph Comm1 ["Community 01 (Orders)"]
        O1["OrderService.cpp"]
        O2["OrderModel.cpp"]
    end

    subgraph Comm2 ["Community 02 (Inventory)"]
        I1["WarehouseManager.cs"]
        I2["StockChecker.cs"]
    end

    SB["Shared Boundary: IDbContext\n(LOCKED CONTRACT)"]

    A1 --> Comm1
    A2 --> Comm2
    Comm1 -.-> SB
    Comm2 -.-> SB

    classDef a fill:#e8f5e9,stroke:#2e7d32,stroke-width:2px;
    classDef c fill:#e1f5fe,stroke:#0288d1,stroke-width:2px;
    classDef l fill:#ffebee,stroke:#c62828,stroke-width:2px;

    class A1,A2 a;
    class O1,O2,I1,I2 c;
    class SB l;
```

1. **Independent Execution:** Agent 1 modifies files in Community 01, while Agent 2 modifies files in Community 02. Because cut-edges between communities are minimized by the CPM algorithm, collisions are virtually zero.
2. **Locked Boundary Contracts:** Any node classified as `:SharedBoundary` is locked. Neither agent is permitted to alter its signature unilaterally; modifications require an explicit interface proposal.

[^scout-agent]: [`.gemini/agents/scout.md`](file:///home/jasondel/dev/graphdb-skill/.gemini/agents/scout.md)
[^skill-def]: [`.gemini/skills/graphdb/SKILL.md`](file:///home/jasondel/dev/graphdb-skill/.gemini/skills/graphdb/SKILL.md)
