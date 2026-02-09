# GraphDB Skill Architectural Analysis

A comparative analysis between the current `graphdb-skill` implementation and the `alpine/VIEW` version.

## 🚀 Key Architectural Shift: Stateful to Streaming

The primary difference is a transition from an **in-memory, stateful processing model** to a **stateless, streaming architecture**. This change is designed to support codebases with millions of entities that would otherwise exceed Node.js heap limits.

### 1. Stateless Graph Construction
*   **Deterministic IDs**: The `alpine/VIEW` version replaces auto-incrementing counters with MD5-hashed deterministic IDs (e.g., `crypto.createHash('md5').update("${type}:${name}").digest('hex')`).
*   **Memory Efficiency**: The `GraphBuilder` no longer maintains massive `nodes` and `edges` arrays or a `nodeMap`. Instead, it emits entities immediately to disk as they are discovered. It also invokes `global.gc()` periodically during extraction to force garbage collection.
*   **JSONL vs JSON**: Data is now stored in `.jsonl` (JSON Lines) format. This allows for append-only writes and line-by-line reading, bypassing the memory overhead of parsing a single giant JSON array.

### 2. Scalable Data Ingestion
*   **Streaming Import**: `import_to_neo4j.js` now uses `readline` to stream `.jsonl` files.
*   **Database-Level Deduplication**: Since the builder is now stateless, it may emit the same node multiple times (e.g., if a class is referenced in many files). The import script now relies on Neo4j's `MERGE` and `ON CREATE SET` logic to handle deduplication at the database level rather than in Node.js memory.

### 3. Reliability & Cost Control
*   **Safe Re-indexing**: Introduced `manage_embeddings.js`, which allows backing up existing vector embeddings to a local file before a full re-scan. This enables updating extraction logic without the significant cost and time of re-generating embeddings for unchanged code.
*   **Enrichment Circuit Breaker**: `enrich_vectors.js` now features deadlock detection (tracking `seenIds`) and a circuit breaker that aborts the process if a batch fails completely, preventing infinite loops during API failures. It also increases the batch size to 500 for better throughput.
*   **Dynamic Adapter Loading**: `update_file.js` now uses safe dynamic imports for language adapters, allowing the tool to function even if specific WASM dependencies are missing.

### 4. Language Support & Feature Differences

| Feature | Current Version | `alpine/VIEW` Version |
| :--- | :--- | :--- |
| **TypeScript** | ✅ Supported (`TsAdapter.js`) | ❌ Removed (`tree-sitter-typescript` dependency removed) |
| **VB.NET** | ✅ Supported | ⚠️ Code present but disabled in `extract_graph.js` |
| **Implicit Globals** | ❌ Basic | ✅ Detects writes to undefined variables |
| **Surgical Updates** | ✅ Basic | ✅ Improved MERGE logic and added `--file-list` CLI argument for targeted extraction |
| **Tree Sitter** | `^0.22.4` | ⚠️ Downgraded to `^0.21.1` (Likely for stability/compatibility) |

## 💡 Summary Recommendation

The **`alpine/VIEW` version is architecturally superior for large-scale enterprise repositories** due to its streaming nature and deterministic ID system. It solves the "Out of Memory" issues inherent in the current design.

However, if the project requires **TypeScript or VB.NET** support, the current version must be maintained or the adapters must be ported into the new streaming architecture.