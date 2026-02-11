const { execSync } = require('child_process');
const path = require('path');
const fs = require('fs');

// Paths
const ROOT_DIR = path.resolve(__dirname, '../../../../');
const BIN_PATH = path.join(ROOT_DIR, 'bin/graphdb');

async function main() {
    // 0. Auto-Sync Check (Optional: Delegate to Go later)
    // For now, if sync_graph.js exists, we could call it, but 
    // to be truly "Strangler Fig", we should eventually have Go handle it.
    // Keeping it simple for the first pass.

    const args = process.argv.slice(2);
    if (args.length === 0) {
        console.error("Usage: node query_graph.js <query_type> [options]");
        process.exit(1);
    }

    const queryType = args[0];
    const remainingArgs = args.slice(1);

    // Map Query Types to Go Query Types
    // Go supports: search-features, search-similar, hybrid-context, neighbors, impact, globals, seams
    const typeMap = {
        'suggest-seams': 'seams',
        'seams': 'seams',
        'impact': 'impact',
        'globals': 'globals',
        'test-context': 'neighbors',
        'hybrid-context': 'hybrid-context',
        'neighbors': 'neighbors',
        'function': 'neighbors', // Close enough for now, or add specific 'function' type to Go
    };

    const goType = typeMap[queryType] || queryType;

    let goArgs = ['query', '--type', goType];

    // Helper to find flag values in remainingArgs
    const getVal = (flag) => {
        const idx = remainingArgs.indexOf(flag);
        if (idx !== -1 && remainingArgs[idx+1]) return remainingArgs[idx+1];
        return null;
    };

    const target = getVal('--function') || getVal('-f') || getVal('--module') || getVal('-m') || getVal('--file') || getVal('-F');
    if (target) {
        goArgs.push('--target', target);
    }

    const depth = getVal('--depth') || getVal('-d');
    if (depth) {
        goArgs.push('--depth', depth);
    }

    const k = getVal('--k') || getVal('-k');
    if (k) {
        goArgs.push('--limit', k); // Mapping k to limit for seams
    }

    // Module pattern specifically for seams
    const module = getVal('--module') || getVal('-m');
    if (module) {
        goArgs.push('--module', module);
    }

    // Execute
    try {
        const output = execSync(`"${BIN_PATH}" ${goArgs.join(' ')}`, { 
            encoding: 'utf8',
            cwd: ROOT_DIR 
        });
        // The Go binary already outputs JSON to stdout
        console.log(output);
    } catch (e) {
        // If it failed, it might be an unknown query type in Go
        // In that case, we might want to log a better error
        if (e.stdout) console.log(e.stdout);
        console.error(`Query failed: ${e.message}`);
        process.exit(1);
    }
}

main().catch(console.error);
