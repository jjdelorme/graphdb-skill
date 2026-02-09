const neo4jService = require('./Neo4jService');
const fs = require('fs');
const path = require('path');
const readline = require('readline');

const ROOT_DIR = path.resolve(__dirname, '../../../../');
const BACKUP_FILE = path.join(ROOT_DIR, '.gemini/graph_data/embeddings_backup.jsonl');

async function backup(session) {
    console.log("Backing up embeddings...");
    const result = await session.run('MATCH (n) WHERE n.embedding IS NOT NULL RETURN n.id as id, n.embedding as embedding');
    
    const stream = fs.createWriteStream(BACKUP_FILE);
    let count = 0;
    
    for (const record of result.records) {
        const id = record.get('id');
        const embedding = record.get('embedding');
        stream.write(JSON.stringify({ id, embedding }) + '
');
        count++;
    }
    stream.end();
    console.log(`Backed up ${count} embeddings to ${BACKUP_FILE}`);
}

async function restore(session) {
    if (!fs.existsSync(BACKUP_FILE)) {
        console.error(`Backup file not found: ${BACKUP_FILE}`);
        return;
    }
    
    console.log("Restoring embeddings...");
    const fileStream = fs.createReadStream(BACKUP_FILE);
    const rl = readline.createInterface({ input: fileStream, crlfDelay: Infinity });
    
    let batch = [];
    let count = 0;
    const BATCH_SIZE = 1000;

    for await (const line of rl) {
        if (!line.trim()) continue;
        batch.push(JSON.parse(line));
        
        if (batch.length >= BATCH_SIZE) {
            await writeBatch(session, batch);
            count += batch.length;
            process.stdout.write(`Restored ${count} embeddings...`);
            batch = [];
        }
    }
    if (batch.length > 0) {
        await writeBatch(session, batch);
        count += batch.length;
    }
    console.log(`
Restored ${count} embeddings.`);
}

async function writeBatch(session, batch) {
    await session.run(
        `
        UNWIND $batch as row
        MATCH (n:Entity {id: row.id})
        SET n.embedding = row.embedding
        `,
        { batch }
    );
}

async function main() {
    const action = process.argv[2];
    if (!action || (action !== 'backup' && action !== 'restore')) {
        console.log("Usage: node manage_embeddings.js [backup|restore]");
        process.exit(1);
    }

    const session = neo4jService.getSession();
    try {
        if (action === 'backup') await backup(session);
        if (action === 'restore') await restore(session);
    } catch (e) {
        console.error(e);
    } finally {
        await session.close();
        await neo4jService.close();
    }
}

main();
