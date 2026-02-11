const fs = require('fs');
const path = require('path');
const glob = require('glob');
const { execSync } = require('child_process');

// Paths
const ROOT_DIR = path.resolve(__dirname, '../../../../');
const BIN_PATH = path.join(ROOT_DIR, 'bin/graphdb');
const OUTPUT_DIR = path.join(ROOT_DIR, '.gemini/graph_data');
const NODES_FILE = path.join(OUTPUT_DIR, 'nodes.jsonl');
const EDGES_FILE = path.join(OUTPUT_DIR, 'edges.jsonl');

async function main() {
    if (!fs.existsSync(OUTPUT_DIR)) {
        fs.mkdirSync(OUTPUT_DIR, { recursive: true });
    }

    const args = process.argv.slice(2);
    let goArgs = [];

    // 1. Handle File List (Parity with legacy surgical update)
    const fileListArgIndex = args.indexOf('--file-list');
    if (fileListArgIndex !== -1 && args[fileListArgIndex + 1]) {
        goArgs.push('-file-list', args[fileListArgIndex + 1]);
    } else {
        goArgs.push('-dir', ROOT_DIR);
    }

    // 2. Add Output flags
    goArgs.push('-nodes', NODES_FILE, '-edges', EDGES_FILE);

    // 3. Optional: Pass through Vertex/GCP flags if present in .env or args
    // For now, assume Go binary reads from env or uses defaults
    if (process.env.GOOGLE_CLOUD_PROJECT) {
        goArgs.push('-project', process.env.GOOGLE_CLOUD_PROJECT);
    }

    console.log(`Delegating extraction to Go binary: ${BIN_PATH} ingest ${goArgs.join(' ')}`);
    
    try {
        execSync(`"${BIN_PATH}" ingest ${goArgs.join(' ')}`, { 
            stdio: 'inherit',
            cwd: ROOT_DIR 
        });
        console.log("Extraction completed via Go binary.");
    } catch (e) {
        console.error("Extraction failed:", e.message);
        process.exit(1);
    }
}

if (require.main === module) {
    main().catch(console.error);
}

module.exports = { main };
