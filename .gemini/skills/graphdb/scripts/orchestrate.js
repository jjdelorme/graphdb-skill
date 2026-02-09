const { execSync } = require('child_process');
const path = require('path');

const ROOT_DIR = path.resolve(__dirname, '../../../../');
const EXTRACT_SCRIPT = path.join(__dirname, '../extraction/extract_graph.js');
const IMPORT_SCRIPT = path.join(__dirname, 'import_to_neo4j.js');

function run(cmd) {
    console.log(`
>>> Running: ${cmd}`);
    execSync(cmd, { stdio: 'inherit', cwd: ROOT_DIR });
}

function main() {
    try {
        console.log("Starting Graph Refresh Workflow...");
        
        // 1. Extract
        run(`node ${EXTRACT_SCRIPT}`);
        
        // 2. Import
        run(`node ${IMPORT_SCRIPT}`);
        
        console.log("
Workflow Complete.");
    } catch (e) {
        console.error("Workflow failed:", e.message);
        process.exit(1);
    }
}

main();
