const Neo4jService = require('./Neo4jService');
const SnippetService = require('./tools/SnippetService');
const path = require('path');

async function main() {
    const args = process.argv.slice(2);
    
    const getArg = (name) => {
        const idx = args.indexOf(name);
        return idx !== -1 ? args[idx + 1] : null;
    };

    const sourceId = getArg('--source');
    const targetId = getArg('--target');
    const contextLines = parseInt(getArg('--context') || '2', 10);

    if (!sourceId || !targetId) {
        console.error('Usage: node locate_usage.js --source <SourceID> --target <TargetID> [--context <lines>]');
        process.exit(1);
    }

    try {
        const query = `
            MATCH (source) WHERE source.id = $sourceId 
            MATCH (target) WHERE target.id = $targetId 
            RETURN source.file as file, source.start_line as start, source.end_line as end, target.name as name
        `;
        
        const result = await Neo4jService.run(query, { sourceId, targetId });

        if (result.records.length === 0) {
            console.error(`Source or Target node not found.`);
            process.exit(1);
        }

        const record = result.records[0];
        const file = record.get('file');
        const start = Neo4jService.toNum(record.get('start'));
        const end = Neo4jService.toNum(record.get('end'));
        const targetName = record.get('name');

        if (!file || start === null || end === null) {
            console.error(`Source node ${sourceId} missing location info.`);
            process.exit(1);
        }

        const absPath = path.resolve(process.cwd(), file);
        
        // 1. Read the source scope
        const scopeContent = await SnippetService.sliceFile(absPath, start, end);
        
        // 2. Find usages
        // targetName might be null if target is anonymous? Unlikely for dependencies.
        if (!targetName) {
             console.error(`Target node ${targetId} has no name.`);
             process.exit(1);
        }

        const matches = SnippetService.findPatternInScope(scopeContent, targetName, contextLines, start);
        
        // 3. Output as JSON
        console.log(JSON.stringify(matches, null, 2));

    } catch (error) {
        console.error('Error:', error);
        process.exit(1);
    } finally {
        await Neo4jService.close();
    }
}

main();
