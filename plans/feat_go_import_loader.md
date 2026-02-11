# Feature Implementation Plan: Go Import Loader

## 📋 Todo Checklist
- [ ] **Phase 1: Core Logic (The Loader Package)**
    - [ ] Define `Loader` struct and interface in `internal/loader`.
    - [ ] Implement batching and grouping logic (parity with JS).
    - [ ] Implement `git` commit hash extraction.
- [ ] **Phase 2: Neo4j Integration**
    - [ ] Implement Neo4j connection and Cypher execution.
    - [ ] Implement `Clean` (DB wipe) and `Constraints` creation.
    - [ ] Implement `UpdateGraphState`.
- [ ] **Phase 3: CLI Integration**
    - [ ] Add `import` command to `cmd/graphdb/main.go`.
    - [ ] Wire up flags (`-nodes`, `-edges`, `-clean`, `-incremental`).
- [ ] **Phase 4: Verification**
    - [ ] Verify against local Neo4j instance (manual/scripted).
    - [ ] Verify `GraphState` update.

## 🔍 Analysis & Investigation

### Current State (Node.js)
The existing `import_to_neo4j.js` script handles:
1.  **Connection:** Connects via `neo4j-driver`.
2.  **Cleaning:** Optionally wipes the DB (`MATCH (n) DETACH DELETE n`).
3.  **Constraints:** Creates uniqueness constraints on `Entity(id)`.
4.  **Batching:** Reads JSONL, buffers 2000 lines.
5.  **Grouping:** Inside a batch, groups items by `type` (Nodes) or relationship label (Edges) to optimize Cypher.
6.  **Execution:** Uses `UNWIND $batch` to bulk insert.
7.  **State:** Records the current git commit hash.

### Target State (Go)
We need a new package `internal/loader` that replicates this exactly.
*   **Performance:** Go's `bufio.Scanner` is faster than Node streams.
*   **Concurrency:** We can keep it simple (sequential batches) or parallelize. Sequential is safer for database load and easier to debug, matching JS behavior.
*   **Dependencies:** `github.com/neo4j/neo4j-go-driver/v5/neo4j`.

### Architecture
*   **`internal/loader`**:
    *   `type Loader struct`: Holds the Neo4j driver.
    *   `func NewLoader(cfg config.Config) (*Loader, error)`
    *   `func (l *Loader) Import(ctx context.Context, nodesPath, edgesPath string, opts ImportOptions) error`
*   **`cmd/graphdb/main.go`**:
    *   New subcommand `import`.
    *   Flags: `-nodes`, `-edges`, `-clean`, `-incremental`.

## 📝 Implementation Plan

### Prerequisites
*   Ensure `go.mod` has `github.com/neo4j/neo4j-go-driver/v5`. (Already present).

### Step-by-Step Implementation

#### Phase 1: The Loader Package (`internal/loader`)

1.  **Step 1.A (The Interface):** Define the `Loader` structure.
    *   *Action:* Create `internal/loader/loader.go`.
    *   *Content:* Define `Loader` struct, `ImportOptions` struct.
    *   *Goal:* Establish the API surface.

2.  **Step 1.B (Batch Processing Logic):** Implement the reading and grouping logic.
    *   *Action:* Add `processFile` method to `Loader`.
    *   *Logic:*
        *   Open file.
        *   Scan line by line.
        *   Unmarshal JSON to `map[string]any`.
        *   Append to `batch []map[string]any`.
        *   If `len(batch) >= 2000`, call `flushBatch`.
    *   *Action:* Add `flushBatch` method.
    *   *Logic:*
        *   Group items by `type` field.
        *   Iterate groups.
        *   Construct Cypher query (dynamic label/type).
        *   Execute via Driver.

3.  **Step 1.C (Graph State):** Implement git hash retrieval.
    *   *Action:* Add helper function `getGitCommit() (string, error)` using `exec.Command("git", "rev-parse", "HEAD")`.
    *   *Action:* Add `updateGraphState` method to `Loader`.

#### Phase 2: Neo4j Integration

1.  **Step 2.A (Connection & Setup):**
    *   *Action:* Implement `NewLoader` using `internal/config`.
    *   *Action:* Implement `CleanDB` method (execute delete query).
    *   *Action:* Implement `EnsureConstraints` method.

2.  **Step 2.B (Cypher Queries):**
    *   *Nodes Query:*
        ```cypher
        UNWIND $batch AS row
        MERGE (n:Entity {id: row.id})
        SET n += row
        // Dynamic label injection: MERGE ... SET n:<Label>
        ```
        *Note:* Neo4j doesn't allow dynamic labels in parameters. We must inject the label into the query string safely (sanitize/validate input).

    *   *Edges Query:*
        ```cypher
        UNWIND $batch AS row
        MATCH (source:Entity {id: row.source})
        MATCH (target:Entity {id: row.target})
        MERGE (source)-[r:REL_TYPE]->(target)
        // Dynamic type injection: MERGE ... -[r:<Type>]-> ...
        ```

#### Phase 3: CLI Command (`cmd/graphdb`)

1.  **Step 3.A (Command Registration):**
    *   *Action:* Update `cmd/graphdb/main.go`.
    *   *Detail:* Add `case "import": handleImport(os.Args[2:])`.

2.  **Step 3.B (Handler Implementation):**
    *   *Action:* Implement `handleImport` function.
    *   *Detail:*
        *   Parse flags.
        *   Load Config.
        *   Initialize `Loader`.
        *   Run `l.Import(...)`.
        *   Handle errors gracefully.

### Testing Strategy
1.  **Unit Testing (Logic):**
    *   Test `processFile` logic by mocking the "flush" step (use a callback or interface).
    *   Verify grouping logic handles mixed types correctly.
2.  **Integration Testing:**
    *   Since we cannot spin up Neo4j easily in CI yet, rely on manual verification (Plan assumes developer runs this against local DB).
    *   The plan will include a "Dry Run" flag for the loader that prints Cypher queries instead of executing them, allowing verification of query construction.

## 🎯 Success Criteria
1.  Running `graphdb import -nodes nodes.jsonl -edges edges.jsonl -clean` populates the local Neo4j database.
2.  The resulting graph has the same node counts as the JS implementation.
3.  The `GraphState` node is updated with the current commit hash.
4.  No dependency on `node` or `import_to_neo4j.js`.
