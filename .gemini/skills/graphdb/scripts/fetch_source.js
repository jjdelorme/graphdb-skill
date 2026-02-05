const Neo4jService = require('./Neo4jService');
const SnippetService = require('./tools/SnippetService');
const path = require('path');

async function main() {
    const args = process.argv.slice(2);
    const idIndex = args.indexOf('--id');
    const nodeId = idIndex !== -1 ? args[idIndex + 1] : null;

    if (!nodeId) {
        console.error('Usage: node fetch_source.js --id <NodeID>');
        process.exit(1);
    }

    try {
        const result = await Neo4jService.run(
            `MATCH (n) WHERE n.id = $id RETURN n.file as file, n.start_line as start, n.end_line as end`,
            { id: nodeId }
        );

        if (result.records.length === 0) {
            console.error(`Node not found: ${nodeId}`);
            process.exit(1);
        }

        const record = result.records[0];
        const file = record.get('file');
        const start = Neo4jService.toNum(record.get('start'));
        const end = Neo4jService.toNum(record.get('end'));

        if (!file) {
            console.error(`Node ${nodeId} has no file associated.`);
            process.exit(1);
        }

        // Handle full file requested (start/end might be null for File nodes)
        // If start/end are missing, read the whole file? Or default to head?
        // The plan says "Slice: Return lines from start_line to end_line".
        // If they are missing, maybe just read the first 50 lines?
        // Or throw error?
        // Let's default to entire file if start/end are null (e.g. for a File node),
        // OR 1-50 if it's too big?
        // Let's stick to strict behavior: if start/end defined, slice.
        // If not, maybe it's a File node, so read all?
        
        let content;
        const absPath = path.resolve(process.cwd(), file);

        if (start !== null && end !== null) {
            content = await SnippetService.sliceFile(absPath, start, end);
        } else {
             // Maybe it's a File node or missing info.
             // Let's read the first 100 lines as a preview if specific lines aren't set.
             // Or better, just read the file but warn.
             // Actually, `sliceFile` expects numbers.
             // Let's default to 1..100 if missing, to be safe against huge files.
             // console.error(`Node ${nodeId} missing line info. Defaulting to first 50 lines.`);
             content = await SnippetService.sliceFile(absPath, 1, 50);
        }

        console.log(content);

    } catch (error) {
        console.error('Error:', error);
        process.exit(1);
    } finally {
        await Neo4jService.close();
    }
}

main();
