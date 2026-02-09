# Feature Implementation Plan: Toolchain Fixes (GraphDB Skill)

## 📋 Todo Checklist
- [x] Restore Node dependencies
- [x] Refactor `query_graph.js` to use `commander`
- [x] Verify CLI functionality with test queries

## 🔍 Analysis & Investigation
The `graphdb` skill is a critical component for the architect agent to map the codebase. Currently, it suffers from two issues:
1.  **Fragile Argument Parsing:** `scripts/query_graph.js` uses a manual loop to parse `process.argv`. This is error-prone and doesn't handle flags or missing values correctly.
2.  **Unused Dependency:** `commander` is listed in `package.json` but not used in the code.

The goal is to modernize the CLI entry point of the skill to use `commander`, ensuring robust argument parsing and auto-generated help.

### Dependencies
- `commander` (already in `package.json`)
- `neo4j-driver` (existing)

## 📝 Implementation Plan

### Prerequisites
- Working Node.js environment.
- Access to `.gemini/skills/graphdb`.

### Step-by-Step Implementation

#### Phase 1: Dependency Restoration
1.  **Step 1.A (Install):** Ensure all dependencies are installed.
    *   *Action:* Run `npm install` in `.gemini/skills/graphdb`.
    *   *Goal:* Ensure `commander` and other libs are available in `node_modules`.

#### Phase 2: Refactor CLI Logic
1.  **Step 2.A (The Harness):** Verify current failure mode (or baseline).
    *   *Action:* Run `node scripts/query_graph.js --help` (from skill dir).
    *   *Goal:* Observe that it currently treats `--help` as a query type or fails, demonstrating lack of proper CLI handling.
2.  **Step 2.B (The Implementation):** Refactor `scripts/query_graph.js`.
    *   *Action:* Modify `scripts/query_graph.js` to:
        *   Import `{ Command }` from `commander`.
        *   Instantiate a new `Command`.
        *   Define a generic action or subcommands that map to the `queries` object.
        *   Define global options: `--module <pattern>`, `--function <name>`, `--file <path>`, `--k <number>`.
        *   Ensure `await syncGraph()` is called before executing any query.
        *   Handle output JSON formatting (handling BigInts) as the original code does.
    *   *Detail:*
        ```javascript
        const { Command } = require('commander');
        const program = new Command();
        
        program
          .name('query_graph')
          .description('CLI to query the Neo4j graph')
          .version('1.0.0');

        // Add options that apply to various commands
        program
          .option('-m, --module <pattern>', 'Module pattern')
          .option('-f, --function <name>', 'Function name')
          .option('-F, --file <path>', 'File path')
          .option('-k, --k <number>', 'Cluster count');

        program
          .argument('<query_type>', 'Type of query to run')
          .action(async (queryType, options) => {
             // ... syncGraph logic ...
             // ... dispatch to queries[queryType] ...
          });
        ```
3.  **Step 2.C (The Verification):** Verify the harness.
    *   *Action:* Run `node scripts/query_graph.js --help`.
    *   *Success:* Should show standard Commander help output.
    *   *Action:* Run a real query (e.g., `progress`).
    *   *Success:* Should return JSON output.

### Testing Strategy
*   **Manual Verification:** Since this is a tool script, we will verify by running it with various arguments.
    *   `node scripts/query_graph.js progress`
    *   `node scripts/query_graph.js seams --module "Trucks"`
    *   `node scripts/query_graph.js impact --function "Calculate"` (assuming a function name)

## 🎯 Success Criteria
*   `query_graph.js` uses `commander` for argument parsing.
*   Running with `--help` displays usage information.
*   Existing functionality (queries) remains accessible and produces correct JSON output.
