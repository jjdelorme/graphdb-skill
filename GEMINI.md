# GraphDB Skill Ecosystem

## 📖 Project Overview

This workspace hosts the **GraphDB Skill**, a powerful subsystem for the Gemini CLI designed to analyze, visualize, and assist in the modernization of large legacy codebases.

It employs a **Hybrid Architecture** that combines:
1.  **Code Property Graph (CPG):** A Neo4j database representing precise structural relationships (Calls, Inheritance, Variable Usage) extracted from source code (C++, C#, VB.NET, SQL, etc.).
2.  **Vector Embeddings:** Semantic understanding of code functions to identify implicit links and "conceptual" dependencies that static analysis misses.

## 📂 Repository Structure

*   **`.gemini/skills/graphdb/`**: The core skill. Contains logic for parsing code (Tree-sitter), building the graph, and querying it.
*   **`.gemini/skills/neo4j-manager/`**: A utility skill for managing Neo4j Community Edition databases (handling the single-active-database limitation).
*   **`plans/`**: Strategic documentation and architectural plans.

## 🚀 Getting Started

### Prerequisites

*   **Node.js**: v20+
*   **Neo4j Community Edition**: v5.x (Local) with Vector Index support (v5.11+).
*   **Google Cloud Project**: For Vertex AI embeddings (required for Vector Search).

### Configuration (`.env`)
Ensure a `.env` file exists in the project root with the following:

```ini
# Neo4j Configuration
NEO4J_URI=bolt://localhost:7687
NEO4J_USER=neo4j
NEO4J_PASSWORD=*DB Password*

# Google Cloud (For Embeddings)
GOOGLE_CLOUD_PROJECT=your_project_id
GOOGLE_CLOUD_LOCATION=us-central1
GEMINI_EMBEDDING_MODEL=gemini-embedding-001
GEMINI_EMBEDDING_DIMENSIONS=768
```

### Installation

Install dependencies for both skills. Note that the `graphdb` skill requires the `--legacy-peer-deps` flag due to `tree-sitter` version incompatibilities.

```bash
cd .gemini/skills/graphdb && npm install --legacy-peer-deps
cd ../neo4j-manager && npm install
cd ../../../ # Return to root
```

**Build the GraphDB Go Binary:**
The skill relies on a compiled Go binary. It MUST be built to the `.gemini/skills/graphdb/scripts/` directory. This is handled automatically by the Makefile. Always build from the project root:

```bash
make build
```

## 🛠️ Operational Guides

### Neo4j & SSH Operations (Remote Management)

We frequently operate on a remote Neo4j instance running in a GCP VM, accessed via an SSH session in tmux pane `0:0.2`.

**Target Pane:** `0:0.2` (SSH Session)
*DB Password* (see `.env`)

#### 1. Checking Database Status (Counts & Embeddings)
To check how many nodes have been processed (enriched with embeddings) without leaving the CLI:

```bash
# Command to send to the tmux pane
tmux send-keys -t 0:0.2 "cypher-shell -u neo4j -p *DB Password* 'MATCH (n) WHERE n.embedding IS NOT NULL RETURN count(n) as embedded'" Enter 

# Wait a few seconds, then capture the output
sleep 3 && tmux capture-pane -t 0:0.2 -p | tail -n 15
```

#### 2. Checking Disk Usage
To monitor if the database is growing too large for the remote disk:

```bash
# Check Neo4j data directory size
tmux send-keys -t 0:0.2 "du -sh /var/lib/neo4j/data" Enter 
sleep 1 && tmux capture-pane -t 0:0.2 -p | tail -n 5
```

#### 3. Calculating Progress & Estimates
If you have two data points (Time A count, Time B count), use this formula:
*   **Rate:** `(Count_B - Count_A) / (Time_B - Time_A_minutes)` = Items per Minute.
*   **Remaining:** `Total_Nodes - Count_B`
*   **ETA:** `Remaining / Rate` = Minutes to completion.

#### 4. Executing Complex Queries (File Transfer)
For complex queries (like vector search), it is safer to write a file remotely than to type long strings into the shell.

```bash
# 1. Create query file locally
echo "MATCH (n) RETURN count(n);" > query.cypher

# 2. 'Cat' it into the remote pane via heredoc
tmux send-keys -t 0:0.2 "cat > query.cypher << 'EOF'" Enter
tmux load-buffer query.cypher
tmux paste-buffer -t 0:0.2
tmux send-keys -t 0:0.2 Enter "EOF" Enter

# 3. Run it
tmux send-keys -t 0:0.2 "cypher-shell -u neo4j -p *DB Password* -f query.cypher" Enter
```

### Cross-Platform Compilation (Windows)

To compile the `graphdb` binary for Windows from a Linux environment (e.g., this Devbox), you must use Zig as the C cross-compiler because of the CGO dependency (`tree-sitter`).

#### 1. Install Zig Locally (One-time Setup)
```bash
# Download and extract Zig to a local tools directory
mkdir -p .gemini/tools && cd .gemini/tools
curl -L -O https://ziglang.org/download/0.13.0/zig-linux-x86_64-0.13.0.tar.xz
tar -xf zig-linux-x86_64-0.13.0.tar.xz
cd ../..
```

#### 2. Build Windows Binary
```bash
# Ensure Zig is in PATH
export PATH=$PWD/.gemini/tools/zig-linux-x86_64-0.13.0:$PATH

# Run Cross-Compilation
# CGO_ENABLED=1 is required for tree-sitter
# CC/CXX set to zig cc with the windows-gnu target
env CGO_ENABLED=1 \
    GOOS=windows \
    GOARCH=amd64 \
    CC="zig cc -target x86_64-windows-gnu" \
    CXX="zig c++ -target x86_64-windows-gnu" \
    go build -o dist/graphdb-win.exe ./cmd/graphdb
```

#### 3. Output
The binary will be located at `dist/graphdb-win.exe`. It is a standalone executable with no external dependencies.