# GraphDB Skill Ecosystem

This project is fundamentally a **Gemini CLI Skill** specialized for navigating, understanding, and modernizing large legacy codebases. 

By representing your codebase as a Code Property Graph (CPG) enriched with semantic vector embeddings, this skill gives the Gemini CLI unprecedented spatial and contextual awareness of complex architectures. All interactions with the graph database are natively routed through the Gemini CLI when the skill is properly registered (e.g., by ensuring the `.gemini/skills/graphdb/SKILL.md` file is present in your workspace). 

The skill relies on a high-performance, cross-platform Go binary (compiled for Windows, Linux, and macOS) to handle the heavy lifting of code parsing, graph construction, and vector search. This allows the CLI agent to rapidly query deep structural and semantic dependencies directly within its standard execution context.

## 📦 Installation

You do not need to clone this repository or build from source to use the skill in your own projects. All necessary files and pre-compiled binaries are packaged into a single release bundle available on our [GitHub Releases page](https://github.com/jjdelorme/graphdb-skill/releases).

### Linux / macOS

Run this one-liner from your project's root directory:

```bash
curl -sL https://github.com/jjdelorme/graphdb-skill/releases/latest/download/graphdb-bundle-linux.tar.gz | tar -xzv
```

*Note: The extraction process preserves executable permissions, but if you encounter issues, run: `chmod +x .gemini/skills/graphdb/scripts/graphdb`*

### Windows (PowerShell)

Run this command from your project's root directory:

```powershell
curl.exe -sL https://github.com/jjdelorme/graphdb-skill/releases/latest/download/graphdb-bundle-windows.tar.gz -o bundle.tar.gz; tar.exe -xzvf bundle.tar.gz; del bundle.tar.gz
```

This downloads and extracts the `.gemini/` directory structure directly into your project, instantly registering the `SKILL.md` definitions, the specialized agents, and the compiled Go binary. The bundle automatically includes `.gitignore` files to prevent the downloaded skill components from being committed to your repository.

### Pre-releases (Beta)

If you want to try the latest beta features (or if the `latest` stable release does not yet include the bundle), you must specify the exact version tag instead of `latest` in the download URL. 

For example, to install `v1.8.403-beta` on Linux/macOS:

```bash
curl -sL https://github.com/jjdelorme/graphdb-skill/releases/download/v1.8.403-beta/graphdb-bundle-linux.tar.gz | tar -xzv
```

**(For Windows PowerShell users):**
```powershell
curl.exe -sL https://github.com/jjdelorme/graphdb-skill/releases/download/v1.8.403-beta/graphdb-bundle-windows.tar.gz -o bundle.tar.gz; tar.exe -xzvf bundle.tar.gz; del bundle.tar.gz
```

## ⚙️ Configuration & Credentials

Before using the skill, you must create a `.env` file in your project root. This file contains your credentials for configuring the Neo4j database connection, the embedding models for vector search, and the LLMs for semantic clustering.

You can also enable detailed logging for the `graphdb` binary by specifying a file path in the `GRAPHDB_LOG` variable.

```ini
# Neo4j Configuration
NEO4J_URI=bolt://localhost:7687
NEO4J_USER=neo4j
NEO4J_PASSWORD=your_secure_password
NEO4J_DATABASE=neo4j

# Google Cloud / AI Configuration (For Embeddings and RPG Extraction)
GOOGLE_CLOUD_PROJECT=your_project_id
GOOGLE_CLOUD_LOCATION=global
GEMINI_EMBEDDING_MODEL=gemini-embedding-001
GEMINI_EMBEDDING_DIMENSIONS=768
GEMINI_GENERATIVE_MODEL=gemini-3.1-flash-lite

# Logging & Execution
GRAPHDB_LOG=graphdb.log # Enable logging to this file
GRAPHDB_DIR=. # Optional: Base directory for source files and state lookup
LLM_CONCURRENCY=5 # Optional: Number of concurrent LLM requests during RPG enrichment
GEMINI_BATCH_GCS_BUCKET=your-gcs-bucket-name # Optional: Default GCS bucket for Vertex AI Batch jobs
```

### Custom LLM Backends (Experimental)

> [!WARNING]
> This capability is strictly **Experimental** and not guaranteed to be stable.

If you want to use a privately hosted model (like a custom Cloud Run Gemma endpoint, local Ollama, Azure OpenAI, etc.) instead of the default Vertex AI, you can override the Go SDK's `BaseURL` and backend type by adding these variables to your `.env` file:

```ini
# The type of backend (e.g., "gemini", "openai", "anthropic", "azure", "ollama")
GENAI_BACKEND=gemini

# Your custom endpoint URL (e.g., https://your-cloud-run-url or http://localhost:11434/v1)
GENAI_BASE_URL=https://<YOUR_CUSTOM_ENDPOINT_URL>

# API key (or "dummy-key" if your custom server ignores it)
GENAI_API_KEY=dummy-key

# API version mapping (e.g., v1beta)
GENAI_API_VERSION=v1beta

# The specific model ID your backend should use
GEMINI_GENERATIVE_MODEL=gemma-4
```

## 🤖 Multi-Agent Orchestration

This project is designed to integrate seamlessly with the [plan-commands](https://github.com/jjdelorme/plan-commands) orchestration framework. 

While the GraphDB Skill can be used as a standalone tool, we highly recommend using it alongside a structured multi-agent orchestration pattern. When combined with the **Protocol Lifecycle** defined by `plan-commands`, the system becomes capable of handling complex, multi-step modernization tasks self-correctingly. For more details on using the Protocol Lifecycle, please refer to the `plan-commands` documentation.

### The Scout Agent

To support this ecosystem, we provide a specialized **Scout** agent (located in `.gemini/agents/scout.md`). 

Within the `plan-commands` lifecycle, the Scout acts as the primary "Researcher" during the strategy phase. Instead of relying purely on brute-force text search, the Scout natively leverages this GraphDB skill to:
*   Map deep architectural dependencies.
*   Identify global state usage across the codebase.
*   Find architectural "seams" for safe refactoring.

*Note: Enabling the full multi-agent orchestration is highly recommended for large refactoring campaigns, but it is not required to use the GraphDB functionality.*

## 🛠️ Build & Ingestion Workflow

To analyze a codebase, you must first ingest it into the Graph Database. This is done with a single command that handles parsing, embedding, clustering, and importing:

```bash
.gemini/skills/graphdb/scripts/graphdb build-all -dir <target-dir>
```

This command sequentially executes the full pipeline:
1.  **Ingest:** Parses source code and generates structural graph data.
2.  **Import:** Loads the data into the active graph database.
3.  **Enrich:** Generates semantic embeddings and builds the RPG intent layer.
4.  **Modernize:** Calculates architectural risk and test linkages.

For advanced users requiring granular control, each step can be run individually. Refer to the [Skill Documentation](.gemini/skills/graphdb/SKILL.md) for manual pipeline details.

### Asynchronous Feature Enrichment (Vertex AI Batch API)

For very large codebases, you can run the build sequence using the Vertex AI Batch API to process features asynchronously. This uploads the prompts to a Google Cloud Storage (GCS) bucket and processes them in bulk rather than running inline queries, pausing the build until the batch job succeeds.

1. **Submit Batch Build:**
   ```bash
   .gemini/skills/graphdb/scripts/graphdb build-all --batch --gcs-bucket <your-gcs-bucket>
   ```
   *This executes Ingestion and Import phases, submits the feature extraction job to Vertex AI, and pauses the build.*
   *Note: If `--gcs-bucket` is omitted, the CLI will fall back to the `GEMINI_BATCH_GCS_BUCKET` variable in your `.env`.*

2. **Resume and Finalize Build:**
   Once the batch job completes on Google Cloud, resume the build to import features and complete the remaining phases (Git History, Contamination, Test Linkage):
   ```bash
   .gemini/skills/graphdb/scripts/graphdb build-all --resume
   ```
   *If the batch job is still running, this command will report the active status and exit safely. You can run it again later.*



## 🔍 Usage & Analysis

The project follows a **"Graph-First"** workflow powered by the **`graphdb` Go binary**. It provides a unified interface for structural (Neo4j), semantic (Vector Embeddings), and intent-based (RPG) analysis.

### Query Commands

All queries use the same pattern: `.gemini/skills/graphdb/scripts/graphdb query -type <type> [options]`

*   **Intent-Based Search (RPG):** Find where a concept lives in the codebase.
    ```bash
    .gemini/skills/graphdb/scripts/graphdb query -type search-features -target "authentication"
    ```
*   **Explore Feature Hierarchy:** Navigate the RPG domain/feature tree.
    ```bash
    .gemini/skills/graphdb/scripts/graphdb query -type explore-domain -target "domain-rpg"
    ```
*   **Dependency Analysis:** Determine what a function depends on.
    ```bash
    .gemini/skills/graphdb/scripts/graphdb query -type neighbors -target "function_name"
    ```
*   **Impact Analysis:** Find upstream callers affected by a change.
    ```bash
    .gemini/skills/graphdb/scripts/graphdb query -type impact -target "function_name" -depth 3
    ```
*   **Hybrid Context:** Combine structural dependencies with semantic similarity.
    ```bash
    .gemini/skills/graphdb/scripts/graphdb query -type hybrid-context -target "function_name"
    ```
*   **Other query types:** `search-similar`, `globals`, `seams`, `fetch-source`, `locate-usage`.

### Text Search (Fallback)
Use standard `search_file_content` (Ripgrep) **ONLY** when the `graphdb` skill cannot provide the necessary data (e.g., searching for non-code assets or literal TODOs).
