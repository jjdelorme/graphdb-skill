# Plan: Automated Seam Discovery via Semantic Clustering

**Goal:** Enable "Massive Modernization" of legacy codebases by identifying **Refactoring Seams** (boundaries where code can be decoupled). This automates the discovery of implicit modules within "God Classes" or "Big Balls of Mud," directly supporting Michael Feathers' strategies for breaking dependencies.

**Theory:** In legacy code, *structural* boundaries (folders, namespaces) often lie. *Semantic* boundaries (what the code *does*) are the source of truth. We use vector clustering to find these hidden boundaries.

---

## 1. Rationale: The "Seam" Problem
In "Working with Legacy Code", a **Seam** is a place where you can alter behavior without editing the code in that place. To create seams in a monolith, you must first identify **Cohesive Clusters**—groups of functions that belong together but are currently scattered or tangled.

*   **Current State:** User asks "Find billing code." (Search).
*   **Future State:** Tool suggests "I see 3 distinct clusters in `SystemManager.cpp`: `Logging`, `Auth`, and `Billing`." (Discovery).

---

## 2. Technical Approach: Client-Side Clustering
We will implement clustering logic within the Node.js skill (Client-Side) rather than relying on complex database plugins (Neo4j GDS). This keeps the skill portable and lightweight.

### A. The Algorithm
*   **Choice:** **K-Means** (or **K-Means++**)
*   **Why:** We often know roughly how many responsibilities a class *should* have (e.g., "Split this into 3 parts"), or we can iterate (Try K=2, K=3, K=4 and measure "Silhouette Score" to find the best fit).
*   **Library:** `ml-kmeans` (npm) or similar lightweight math lib.

### B. The Workflow (`query_graph.js suggest-seams`)

1.  **Scope Selection:**
    *   User targets a specific Node (e.g., a "God Class" or a "Utilities" folder).
    *   Tool fetches all `(:Function)` nodes connected to that target (methods of the class, or files in the folder).
2.  **Vector Retrieval:**
    *   Fetch the `embedding` vector for each function.
3.  **Clustering:**
    *   Run K-Means on the vectors.
    *   *Auto-tune:* If K is not provided, try K=2 to K=5 and pick the one with the best internal cohesion.
4.  **Labeling (The "GenAI" Magic):**
    *   For each cluster, find the "Centroid" (the most representative function).
    *   (Optional) Ask the LLM: "Here are 5 function names from this cluster: [A, B, C, D, E]. What is a good name for this module?"
5.  **Output:**
    *   Return a JSON report proposing the split.

---

## 3. Use Cases

### Case 1: Deconstructing a "God Class"
*   **Scenario:** `UserManager.cs` has 5,000 lines.
*   **Action:** `suggest-seams --target "UserManager.cs" --k 3`
*   **Result:**
    *   **Cluster A (Methods):** `ValidateEmail`, `CheckPassword`, `HashGen` -> **Proposed: `AuthService`**
    *   **Cluster B (Methods):** `UpdateRow`, `DeleteUser`, `FetchById` -> **Proposed: `UserRepository`**
    *   **Cluster C (Methods):** `FormatDisplay`, `RenderAvatar` -> **Proposed: `UserPresenter`**

### Case 2: Organizing a "Utils" Folder
*   **Scenario:** `src/common` has 200 mixed files.
*   **Action:** `suggest-seams --folder "src/common"`
*   **Result:** Identifies that 50 files are actually "String Manipulation" and 30 are "Date/Time Helpers".

---

## 4. Implementation Stages

### Phase 1: The Clustering Engine
*   **File:** `.gemini/skills/graphdb/scripts/services/ClusterService.js`
*   **Func:** `clusterVectors(vectors: number[][], k: number)`
*   **Output:** Indices grouped by cluster ID.

### Phase 2: Integration
*   **Command:** Update `query_graph.js` with `suggest-seams`.
*   **Logic:**
    1.  Cypher Query: `MATCH (c:Class {name: $name})<-[:MEMBER_OF]-(f:Function) RETURN f.name, f.embedding`
    2.  Call `ClusterService`.
    3.  Print report.

### Phase 3: "Virtual" Nodes (Graph Write-Back)
*   *Optional:* Write the results back to Neo4j.
*   **Create:** `(:ProposedModule {name: "Cluster 1"})`
*   **Link:** `(:Function)-[:BELONGS_TO_PROPOSAL]->(:ProposedModule)`
*   Allows the Agent to "Visualise" the refactor before touching code.

---

## 5. Dependencies
*   `ml-kmeans` (Lightweight, pure JS).
*   Existing `VectorService` (for embeddings).

