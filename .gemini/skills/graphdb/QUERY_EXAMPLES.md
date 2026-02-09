# Codebase Query Examples

The `query_graph.js` script allows you to query the Neo4j knowledge graph for various architectural insights.

## Usage
**Base Command:**
```bash
node .gemini/skills/graphdb/scripts/query_graph.js <query-type> [options]
```

## Query Types

### 1. Hotspots
Finds functions with the highest risk scores (complexity × change frequency).
```bash
node .gemini/skills/graphdb/scripts/query_graph.js hotspots --module <module_name>
```

### 2. Seams
Finds functions that are "seams" - pure business logic functions called by UI-contaminated code. These are primary candidates for service extraction.
```bash
node .gemini/skills/graphdb/scripts/query_graph.js seams --module <module_name>
```

### 3. UI Contamination
Shows stats on how many functions are UI-contaminated vs. pure in a module.
```bash
node .gemini/skills/graphdb/scripts/query_graph.js ui-contamination --module <module_name>
```

### 4. Function Details
Get full properties of a specific function.
```bash
node .gemini/skills/graphdb/scripts/query_graph.js function --function <function_name>
```

### 5. Co-Change
Find files that frequently change together with a specific file.
```bash
node .gemini/skills/graphdb/scripts/query_graph.js co-change --file <file_path>
```

### 6. Impact Analysis
Find functions that call a specific function (upwards dependency).
```bash
# Default depth: 5 levels
node .gemini/skills/graphdb/scripts/query_graph.js impact --function <function_name>

# Custom depth (e.g., 10 levels)
node .gemini/skills/graphdb/scripts/query_graph.js impact --function <function_name> --depth 10
```

### 7. Test Context
Identify dependencies (calls and globals) that need to be handled when writing tests for a function.
```bash
node .gemini/skills/graphdb/scripts/query_graph.js test-context --function <function_name>
```

### 8. Global Access
Find global variables accessed by functions.

**Module Scope (Direct Usage):**
```bash
node .gemini/skills/graphdb/scripts/query_graph.js globals --module <module_name>
```

**Function Scope (Transitive Usage - "Deep Globals"):**
Finds all globals used by a function and its entire call tree.
```bash
# Full transitive search (default)
node .gemini/skills/graphdb/scripts/query_graph.js globals --function <function_name>

# Limit depth (e.g., 3 levels down)
node .gemini/skills/graphdb/scripts/query_graph.js globals --function <function_name> --depth 3
```

### 9. Service Extraction Candidates
Find pure functions with high usage that are good candidates to be moved into a service.
```bash
node .gemini/skills/graphdb/scripts/query_graph.js extract-service --module <module_name>
```

### 10. Overall Progress
Shows project-wide metrics on contamination and risk.
```bash
node .gemini/skills/graphdb/scripts/query_graph.js progress
```
