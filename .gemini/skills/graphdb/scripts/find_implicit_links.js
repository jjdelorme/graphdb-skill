const { execSync } = require('child_process');
const path = require('path');

// Paths
const ROOT_DIR = path.resolve(__dirname, '../../../../');
const BIN_PATH = path.join(ROOT_DIR, 'bin/graphdb');

async function main() {
    const args = process.argv.slice(2);
    
    // Helper to find flag values
    const getVal = (flag) => {
        const idx = args.indexOf(flag);
        if (idx !== -1 && args[idx+1]) return args[idx+1];
        return null;
    };

    const query = getVal('--query') || getVal('-q');
    const limit = getVal('--limit') || getVal('-l') || '5';

    if (!query) {
        console.error("Usage: node find_implicit_links.js --query <text>");
        process.exit(1);
    }

    let goArgs = ['query', '--type', 'search-similar', '--target', `"${query}"`, '--limit', limit];

    try {
        const output = execSync(`"${BIN_PATH}" ${goArgs.join(' ')}`, { 
            encoding: 'utf8',
            cwd: ROOT_DIR 
        });
        
        const results = JSON.parse(output);
        
        console.log('\n--- Search Results ---');
        if (Array.isArray(results)) {
            results.forEach(r => {
                const node = r.node;
                const score = r.score;
                const name = node.label || node.properties.name || node.id;
                const file = node.properties.file_path || node.properties.file || 'unknown';
                const line = node.properties.start_line || 0;
                
                console.log(`[${(score * 100).toFixed(1)}%] ${name}`);
                console.log(`       File: ${file}:${line}`);
            });
        }
        console.log('----------------------');
    } catch (e) {
        console.error("Search failed:", e.message);
        process.exit(1);
    }
}

main().catch(console.error);
