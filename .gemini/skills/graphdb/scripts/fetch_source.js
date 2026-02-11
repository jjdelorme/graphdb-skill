const { execSync } = require('child_process');
const path = require('path');

// Paths
const ROOT_DIR = path.resolve(__dirname, '../../../../');
const BIN_PATH = path.join(ROOT_DIR, 'bin/graphdb');

async function main() {
    const args = process.argv.slice(2);
    const idIndex = args.indexOf('--id');
    const nodeId = idIndex !== -1 ? args[idIndex + 1] : null;

    if (!nodeId) {
        console.error('Usage: node fetch_source.js --id <NodeID>');
        process.exit(1);
    }

    try {
        const output = execSync(`"${BIN_PATH}" query --type fetch-source --target "${nodeId}"`, { 
            encoding: 'utf8',
            cwd: ROOT_DIR 
        });
        console.log(output);
    } catch (error) {
        console.error('Error:', error.message);
        process.exit(1);
    }
}

main();
