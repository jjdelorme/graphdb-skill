---
type: Attested Computation
title: Actionable Feathers Seam Score Computation
description: Sanctioned graph calculation that computes the Return-on-Investment (ROI) score for candidate refactoring seams between structural communities.
tags: [computations, seams, feathers, roi, graph-calculation]
runtime: neo4j-cypher
parameters:
  - { name: limit, type: integer, required: false, default: 10 }
executor:
  resource: /cmd/graphdb/cmd_query.go
  receipt: [actionable_score, internal_fan_in, volatile_fan_out, cut_edges]
attester:
  resource: /internal/analysis/seam_ranker.go
generated: { by: "antigravity/documenter-agent", at: "2026-09-03T14:30:00Z" }
verified: { by: "human:jasondel@google.com", at: "2026-09-03T14:30:00Z" }
status: stable
sources:
  - id: seam-algorithm
    resource: /algorithms/actionable-seam-ranking.md
    title: Actionable Feathers Seam Ranking Algorithm
    author: process:pipeline
    last_modified: 2026-09-03T14:00:00Z
  - id: ranker-code
    resource: /internal/analysis/seam_ranker.go
    title: Actionable Feathers Seam Ranker Implementation
    author: process:pipeline
    last_modified: 2026-09-03T14:00:00Z
---

# Computation

```cypher
MATCH (srcComm:StructuralCommunity)<-[:IN_COMMUNITY]-(f:Function)
MATCH (f)-[r:CALLS]->(target:Function)
WHERE NOT (target)-[:IN_COMMUNITY]->(srcComm)
WITH srcComm, f, count(r) AS cut_edges, collect(target) AS external_targets
OPTIONAL MATCH (caller:Function)-[:CALLS]->(f)
WHERE (caller)-[:IN_COMMUNITY]->(srcComm)
WITH srcComm, f, cut_edges, count(caller) AS internal_fan_in, external_targets
UNWIND external_targets AS ext
WITH srcComm, f, cut_edges, internal_fan_in, sum(CASE WHEN ext.is_volatile THEN 1 ELSE 0 END) AS volatile_fan_out
WITH f, internal_fan_in, volatile_fan_out, cut_edges,
     ((toFloat(internal_fan_in) * toFloat(volatile_fan_out)) / (toFloat(cut_edges) + 1.0)) AS actionable_score
ORDER BY actionable_score DESC
RETURN f.id AS function_id,
       f.name AS function_name,
       internal_fan_in,
       volatile_fan_out,
       cut_edges,
       actionable_score
LIMIT $limit
```

This computation executes the sanctioned Feathers Seam Score handshake:[^seam-algorithm]
1. Identifies functions $f$ crossing community boundaries.
2. Computes the count of stable internal callers within the same community ($\text{InternalFanIn}$).
3. Computes the count of volatile external dependencies ($\text{VolatileFanOut}$).
4. Penalizes complex boundaries by dividing by $(\text{CutEdges} + 1)$.

# What the attester checks

The attester (`internal/analysis/seam_ranker.go`) verifies:[^ranker-code]
1. **Mathematical Invariant:** $\text{ActionableScore} = \frac{\text{InternalFanIn} \times \text{VolatileFanOut}}{\text{CutEdges} + 1.0}$.
2. **Cut-Edge Non-Negativity:** $\text{CutEdges} \ge 1$ for any cross-boundary candidate seam.
3. **Purity:** Functions flagged as `:SharedBoundary` components are partitioned into interface extraction recipes rather than raw seam scores.

[^seam-algorithm]: [`actionable-seam-ranking.md`](file:///algorithms/actionable-seam-ranking.md)
[^ranker-code]: [`seam_ranker.go`](file:///home/jasondel/dev/graphdb-skill/internal/analysis/seam_ranker.go)
