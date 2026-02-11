const { execSync } = require('child_process');
const path = require('path');

// Paths
const ROOT_DIR = path.resolve(__dirname, '../../../../');
const BIN_PATH = path.join(ROOT_DIR, 'bin/graphdb');

async function main() {
    const args = process.argv.slice(2);
    
    const getArg = (name) => {
        const idx = args.indexOf(name);
        return idx !== -1 ? args[idx + 1] : null;
    };

    const sourceId = getArg('--source');
    const targetId = getArg('--target');

    if (!sourceId || !targetId) {
        console.error('Usage: node locate_usage.js --source <SourceID> --target <TargetID>');
        process.exit(1);
    }

    try {
        const output = execSync(`"${BIN_PATH}" query --type locate-usage --target "${sourceId}" --target2 "${targetId}"`, { 
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
