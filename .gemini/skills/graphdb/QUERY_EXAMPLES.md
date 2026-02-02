# Codebase Query Examples

The `query_graph.js` script allows you to query the Neo4j knowledge graph for various architectural insights.

## Usage
**Base Command:**
```powershell
node .gemini/skills/graphdb/scripts/query_graph.js <query-type> [options]
```

## Query Types

### 1. Hotspots
Finds functions with the highest risk scores (complexity × change frequency).
```powershell
node .gemini/skills/graphdb/scripts/query_graph.js hotspots --module <module_name>
```

### 2. Seams
Finds functions that are "seams" - pure business logic functions called by UI-contaminated code. These are primary candidates for service extraction.
```powershell
node .gemini/skills/graphdb/scripts/query_graph.js seams --module <module_name>
```

### 3. UI Contamination
Shows stats on how many functions are UI-contaminated vs. pure in a module.
```powershell
node .gemini/skills/graphdb/scripts/query_graph.js ui-contamination --module <module_name>
```

### 4. Function Details
Get full properties of a specific function.
```powershell
node .gemini/skills/graphdb/scripts/query_graph.js function --function <function_name>
```

### 5. Co-Change
Find files that frequently change together with a specific file.
```powershell
node .gemini/skills/graphdb/scripts/query_graph.js co-change --file <file_path>
```

### 6. Impact Analysis
Find functions that call a specific function (up to 3 levels deep).
```powershell
node .gemini/skills/graphdb/scripts/query_graph.js impact --function <function_name>
```

### 7. Test Context
Identify dependencies (calls and globals) that need to be handled when writing tests for a function.
```powershell
node .gemini/skills/graphdb/scripts/query_graph.js test-context --function <function_name>
```

### 8. Global Access
Find global variables accessed by functions in a module.
```powershell
node .gemini/skills/graphdb/scripts/query_graph.js globals --module <module_name>
```

### 9. Service Extraction Candidates
Find pure functions with high usage that are good candidates to be moved into a service.
```powershell
node .gemini/skills/graphdb/scripts/query_graph.js extract-service --module <module_name>
```

### 10. Overall Progress
Shows project-wide metrics on contamination and risk.
```powershell
node .gemini/skills/graphdb/scripts/query_graph.js progress
```