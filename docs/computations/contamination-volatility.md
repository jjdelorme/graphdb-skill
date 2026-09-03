---
type: Attested Computation
title: Volatility Flood-Fill & Distance Decay Computation
description: Sanctioned graph calculation that propagates runtime fragility upward through the call graph and computes distance-decayed risk scores.
tags: [computations, contamination, volatility, risk, graph-traversal]
runtime: neo4j-cypher
executor:
  resource: /cmd/graphdb/cmd_enrich_contamination.go
  receipt: [is_volatile, volatility_score, risk_score]
attester:
  resource: /internal/query/neo4j_contamination.go
generated: { by: "antigravity/documenter-agent", at: "2026-09-03T14:30:00Z" }
verified: { by: "human:jasondel@google.com", at: "2026-09-03T14:30:00Z" }
status: stable
sources:
  - id: contamination-impl
    resource: /internal/query/neo4j_contamination.go
    title: Contamination Query Implementation
    author: process:pipeline
    last_modified: 2026-09-03T14:00:00Z
  - id: contamination-pipeline
    resource: /pipeline/phase4-contamination.md
    title: Phase 4 Contamination & Risk Specification
    author: process:pipeline
    last_modified: 2026-09-03T14:00:00Z
---

# Computation

### Step 1: Upward Volatility Propagation
```cypher
MATCH (volatile:Function {is_volatile: true})
MATCH (caller:Function)-[:CALLS*]->(volatile)
SET caller.is_volatile = true
```

### Step 2: Shortest-Path Distance Decay
```cypher
MATCH (f:Function)
OPTIONAL MATCH p = shortestPath((f)-[:CALLS*]->(leaf:Function))
WHERE leaf.is_volatile = true AND (f = leaf OR NOT (leaf)-[:CALLS]->(:Function {is_volatile: true}))
WITH f, min(length(p)) AS min_dist
SET f.volatility_score = CASE
    WHEN min_dist IS NULL THEN 0.0
    ELSE 1.0 / (toFloat(min_dist) + 1.0)
END
```

### Step 3: Composite Risk Normalization
```cypher
MATCH (f:Function)
OPTIONAL MATCH (caller:Function)-[:CALLS]->(f)
WITH f, count(DISTINCT caller) AS fan_in
OPTIONAL MATCH (f)-[:CALLS]->(callee:Function)
WITH f, fan_in, count(DISTINCT callee) AS fan_out
OPTIONAL MATCH (f)-[:DEFINED_IN]->(file:File)
WITH f, fan_in, fan_out, coalesce(file.change_frequency, 0) AS churn
WITH f, ((toFloat(fan_in) * 0.4) + (toFloat(fan_out) * 0.1) + (f.volatility_score * 3.0) + (toFloat(churn) * 0.4)) AS raw_risk
WITH max(raw_risk) AS max_risk
MATCH (f:Function)
SET f.risk_score = CASE
    WHEN max_risk = 0 THEN 0.0
    ELSE ((toFloat(f.fan_in) * 0.4) + (toFloat(f.fan_out) * 0.1) + (f.volatility_score * 3.0) + (toFloat(coalesce(f.churn, 0)) * 0.4)) / max_risk
END
```

# What the attester checks

The attester (`internal/query/neo4j_contamination.go`) verifies:[^contamination-impl]
1. **Pre-flight Assertion:** Ensures `HasVolatilityData()` returns true before updating nodes.
2. **Range Invariants:** $0.0 \le \text{volatility\_score} \le 1.0$ and $0.0 \le \text{risk\_score} \le 1.0$.
3. **Decay Monotonicity:** A caller at distance $D+1$ from volatile leaves must never have a higher volatility score than a caller at distance $D$.

[^contamination-impl]: [`neo4j_contamination.go`](file:///home/jasondel/dev/graphdb-skill/internal/query/neo4j_contamination.go)
[^contamination-pipeline]: [`phase4-contamination.md`](file:///pipeline/phase4-contamination.md)
