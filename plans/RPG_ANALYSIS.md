# Analysis of RPG Integration Strategy

**Date:** February 10, 2026
**Target:** `@plans/rpg_integration_strategy.md`
**References:** `plans/RPG.pdf`, `plans/RPG-Encoder.pdf`

## 1. Executive Summary

The proposed **RPG Integration Strategy** provides a practical "Legacy-First" approach to adopting the Repository Planning Graph (RPG) architecture. It correctly identifies the core value proposition (The Dual-View: Intent vs. Implementation) and proposes a viable graph schema.

However, the strategy makes significant compromises in the **Construction Pipeline** (specifically Phase 6: "Vector-First Bootstrap") that deviate from the research findings. These deviations risk creating a "noisy" or "inverted" hierarchy that groups code by *implementation similarity* rather than *functional intent*.

## 2. Consistency Check

| Concept | Strategy Proposal | Research (`RPG-Encoder.pdf`) | Verdict |
| :--- | :--- | :--- | :--- |
| **Architecture** | **Dual-View:** Functional Tree + Dependency Graph. | **Dual-View:** $V_H$ (Functional) + $V_L$ (Implementation) linked by $\mathcal{E}_{feature}$ and $\mathcal{E}_{dep}$. | ✅ **Consistent** |
| **Grounding** | `(:Function)-[:IMPLEMENTS]->(:Feature)` | Links abstract nodes to "Directory Scopes" and atomic code entities. | ✅ **Consistent** (Simplified) |
| **Search** | `search-features` (Intent $	o$ Node) | `SearchNode` (Intent $	o$ Feature/Code). | ✅ **Consistent** |
| **Evolution** | **Vector Distance** triggers re-routing. | **LLM Judgement** ("Major vs Minor Drift") triggers re-routing. | ⚠️ **Deviation** (See Section 3.3) |
| **Construction** | **Bottom-Up Clustering** (K-Means on vectors). | **Hybrid:** Global "Domain Discovery" (Top-Down) + Atomic Feature Extraction + LLM-guided Hierarchy Construction. | ❌ **Major Deviation** |

## 3. Critical Blind Spots & Risks

### 3.1 The "Bottom-Up" Trap (Phase 2 & 6)
The strategy relies heavily on **Clustering (K-Means)** to build the hierarchy. `RPG-Encoder.pdf` (Appendix A.1.2) explicitly warns that "flat features are hard to navigate... often overlooks logical rules."

*   **The Research Insight:** A purely bottom-up approach (clustering code vectors) tends to group "Structurally Similar" code (e.g., all DTOs, all Parsers) rather than "Functionally Coherent" code (e.g., "User Registration" including its DTO, Parser, and Controller).
*   **The Risk:** The resulting tree may act more like a "File Browser" (Implementation View) than a "Feature Map" (Intent View), defeating the purpose of RPG.

### 3.2 Granularity Mismatch (Phase 6.1)
The "Vector-First" bootstrap suggests $K \approx \sqrt{N}$ (e.g., 100 clusters for 10k functions).

*   **The Research Insight:** RPG defines a "Leaf Node" as an **atomic capability** (e.g., "validate email format"), which typically maps to 1-3 functions.
*   **The Risk:** 100 clusters for 10k functions results in ~100 functions per feature. This is too coarse for an agent to use. A "Leaf Feature" containing 100 functions is a "Module," not a feature. The hierarchy needs 2-3 more layers of depth.

### 3.3 Semantic Routing Logic (Phase 4)
The strategy proposes `Vector Distance > Threshold` to trigger re-routing.

*   **The Research Insight:** Vector embeddings are notoriously unstable for "drift detection" in code. A small bug fix might flip a vector significantly, while a major logic change (e.g., changing a tax rate) might stay close.
*   **The Proposed Fix:** The paper uses **Top-Down Semantic Routing** (Algorithm 4). When a function changes, instead of comparing it to its *old self*, the system asks the **Root Feature**: "Which of your children best fits this description?" and traverses down. This ensures the function lands in the *current* best semantic home.

## 4. Recommendations for Review

To align the strategy with the research and ensure a successful implementation, I recommend the following adjustments:

### Recommendation 1: Hybrid Construction (Modify Phase 6)
Don't rely solely on K-Means. Introduce a **Top-Down "Seeding" Step**:
1.  **Domain Discovery:** Feed a high-level summary of the file tree (depth 2-3) to an LLM to generate 5-10 **Top-Level Domains** (e.g., "Auth", "Inventory", "Reporting").
2.  **Constrained Clustering:** When running K-Means, force the clusters to align with these Domains (e.g., cluster only within the `src/auth` folder first, then aggregate).
3.  *Why:* This guarantees the top of the tree is semantically valid, even if the leaves are noisy.

### Recommendation 2: Refine "Leaf" Definition
Explicitly state that the K-Means clusters are **Topic Nodes**, not **Leaf Features**.
*   **Action:** Add a post-processing step where the LLM examines a cluster and can decide to "Split" it into multiple finer-grained Features if it's too broad.

### Recommendation 3: Adopt Top-Down Routing (Modify Phase 4)
Replace the "Vector Distance" logic.
*   **New Logic:**
    1.  On `Function` update, generate a new brief description.
    2.  Check: Does it still align with its current Parent Feature? (LLM Check).
    3.  If NO: Trigger **Top-Down Routing** (Ask Root -> Domain -> Feature) to find its new home.

### Recommendation 4: Add "Directory Scope"
The research highlights the importance of **Structural Constraints**.
*   **Action:** Add a property `scope_path` to `Feature` nodes.
*   *Benefit:* Allows agents to prune the search space ("I only care about features that live inside `src/auth`").

## 5. Conclusion
The current strategy is a valid **engineering approximation** for legacy systems but risks creating an "Inverted RPG" (Implementation disguised as Intent). By injecting a small amount of **Top-Down Domain Discovery** at the start, we can ensure the resulting map is truly useful for agentic reasoning.
