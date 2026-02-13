# Architectural Evaluation: Query Interface Expansion

**Objective:** Evaluate architectural options for expanding the `graphdb` query interface to support ad-hoc investigation, considering the future migration to Google Cloud Spanner (ARIP).

**Context:** The current `GraphProvider` is a collection of task-specific methods (`GetImpact`, `GetSeams`) backed by hardcoded Cypher queries. The roadmap explicitly calls for a migration to Google Spanner Graph (GQL), making "Abstraction Safety" a critical constraint.

## Option 1: Parametric Traversal
*Expanding `GetNeighbors` to accept generic `edge_type` and `node_label` arguments.*

### Assessment
*   **Flexibility:** **Medium**.
    *   ✅ Effective for structural questions ("What calls X?", "What does X inherit from?").
    *   ❌ Weak for attribute-heavy questions ("Find all public methods with complexity > 10"). It relies on the consumer knowing the exact schema (Edge Types: `CALLS`, `USES_GLOBAL`, etc.).
*   **Abstraction Safety:** **High**.
    *   Concepts like `StartNode`, `EdgeType`, `Direction`, and `MaxDepth` are universal across Graph DBs (Neo4j, Spanner, Neptune).
    *   Implementing `Traverse(id, "CALLS", Outgoing, 2)` is trivial in both Cypher and GQL.
*   **Agent Usability:** **Medium**.
    *   LLMs can easily fill in string arguments, but they must be "taught" the schema (valid edge names) via the system prompt or tool definition.

## Option 2: GraphQL-like Filter
*A structured JSON input for filtering nodes/edges (e.g., `{ "label": "Function", "where": { "complexity": { "gt": 10 } } }`).*

### Assessment
*   **Flexibility:** **Very High**.
    *   Allows asking arbitrary questions about node attributes and relationships without code changes.
    *   Can replace multiple specific methods (`FindNode`, `SearchSimilar`) with one generic query.
*   **Abstraction Safety:** **Low (High Risk)**.
    *   Requires building a **Query Transpiler** (JSON -> Cypher AND JSON -> GQL).
    *   Leaky Abstraction Risk: Regex syntax, fuzzy matching, and null handling differ significantly between Neo4j and Spanner. Writing a robust cross-database transpiler is a significant engineering effort that distracts from the core value.
*   **Agent Usability:** **High**.
    *   LLMs are excellent at generating structured JSON constraints.

## Option 3: Task-Based Expansion (Status Quo)
*Continuing to add specific semantic methods like `GetFields`, `GetClasses`, `GetAuthChecks`.*

### Assessment
*   **Flexibility:** **Low**.
    *   Every new question requires:
        1.  Updating the `GraphProvider` interface (Go).
        2.  Implementing the Cypher query (Neo4j).
        3.  Implementing the GQL query (Spanner).
        4.  Recompiling the Agent.
*   **Abstraction Safety:** **Maximum**.
    *   The interface speaks in "Business Domain" terms (`GetImpact`), not "Database" terms.
    *   Allows optimizing the implementation for each backend independently (e.g., using recursive CTEs in SQL/Spanner vs. `MATCH *` in Cypher) without changing the contract.
*   **Agent Usability:** **High (but brittle)**.
    *   Extremely easy to use *if* the specific tool exists.
    *   Frustrating for the Agent if the tool is missing ("I know the data is there, but I can't ask for it").

## Recommendation for Architect

**Admit a Hybrid Approach: "Semantic Discovery + Parametric Traversal"**

1.  **Keep the Task-Based Interface for Complex/Common Queries:**
    *   Retain `GetImpact`, `GetSeams`, and `ExploreDomain`. These involve complex logic (transitive closures, risk scoring) that is best optimized inside the database adapter.

2.  **Implement Option 1 (Parametric Traversal) for "Graph Walking":**
    *   Implement the `Traverse` method immediately. This covers 80% of "ad-hoc" investigation needs (tracing dependencies) without requiring a new deployment for every edge type.
    *   *Signature:* `Traverse(start_id, edge_type_filter, direction, max_depth)`

3.  **Reject Option 2 (GraphQL-like Filter) for now:**
    *   The engineering cost of building a "Spanner-safe" query builder is too high for the current phase.
    *   *Alternative:* Expose a generic `FindNodes(label, property_map)` for simple lookups, but avoid complex operators (OR, nested ANDs) until absolutely necessary.

**Migration Strategy:**
*   Ensure `Traverse` maps cleanly to Spanner's `GRAPH_PATH` or Recursive CTEs.
*   Avoid leaking Neo4j-specific features (like APOC) in the `Traverse` implementation.
