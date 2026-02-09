const neo4jService = require('./Neo4jService');
const fs = require('fs');
const path = require('path');
const readline = require('readline');
const { execSync } = require('child_process');

// Configuration
const ROOT_DIR = path.resolve(__dirname, '../../../../');
const NODES_FILE = process.argv[2] && !process.argv[2].startsWith('--') ? process.argv[2] : path.join(ROOT_DIR, '.gemini/graph_data/nodes.jsonl');
const EDGES_FILE = process.argv[3] && !process.argv[3].startsWith('--') ? process.argv[3] : path.join(ROOT_DIR, '.gemini/graph_data/edges.jsonl');

const BATCH_SIZE = 2000;
const MAX_RETRIES = 3;

async function runWithRetry(session, query, params = {}) {
    let attempts = 0;
    while (attempts < MAX_RETRIES) {
        try {
            return await session.run(query, params);
        } catch (e) {
            attempts++;
            console.warn(`Query failed (Attempt ${attempts}/${MAX_RETRIES}): ${e.message}`);
            if (attempts >= MAX_RETRIES) throw e;
            await new Promise(res => setTimeout(res, 1000 * attempts));
        }
    }
}

async function processStream(filePath, session, type, labelProcessor) {
    if (!fs.existsSync(filePath)) {
        console.warn(`File not found: ${filePath}`);
        return;
    }

    console.log(`Streaming ${type} from ${filePath}...`);
    const fileStream = fs.createReadStream(filePath);
    const rl = readline.createInterface({
        input: fileStream,
        crlfDelay: Infinity
    });

    let batch = [];
    let count = 0;

    for await (const line of rl) {
        if (!line.trim()) continue;
        try {
            batch.push(JSON.parse(line));
        } catch (e) {
            console.warn("Skipping invalid JSON line");
            continue;
        }

        if (batch.length >= BATCH_SIZE) {
            await processBatch(session, batch, labelProcessor);
            count += batch.length;
            process.stdout.write(`\rImported ${count} ${type}...`);
            batch = [];
        }
    }

    if (batch.length > 0) {
        await processBatch(session, batch, labelProcessor);
        count += batch.length;
    }
    console.log(`\nFinished importing ${count} ${type}.`);
}

async function processBatch(session, batch, processor) {
    // Group by type/label to optimize Cypher usage
    const groups = {};
    for (const item of batch) {
        const key = item.type || 'Unknown';
        if (!groups[key]) groups[key] = [];
        groups[key].push(item);
    }

    for (const [key, items] of Object.entries(groups)) {
        const safeKey = key.replace(/[^a-zA-Z0-9_]/g, '');
        await processor(session, safeKey, items);
    }
}

async function run() {
  const session = neo4jService.getSession();
  const isIncremental = process.argv.includes('--incremental');
  
  try {
    console.log(`Connecting to Neo4j at ${neo4jService.uri}...`);
    
    // 1. Clear Database (unless incremental)
    if (!isIncremental) {
        console.log('Clearing existing database...');
        try {
            await runWithRetry(session, 'CALL { MATCH (n) DETACH DELETE n } IN TRANSACTIONS OF 10000 ROWS');
        } catch (e) {
            console.warn('Batched delete failed, trying iterative delete:', e.message);
            while (true) {
                const result = await session.run('MATCH (n) WITH n LIMIT 10000 DETACH DELETE n RETURN count(n) as c');
                const count = neo4jService.toNum(result.records[0].get('c'));
                console.log(`Deleted ${count} nodes...`);
                if (count === 0) break;
            }
        }
        console.log('Database cleared.');

        // 2. Create Constraints
        console.log('Creating constraints...');
        try {
            await runWithRetry(session, 'CREATE CONSTRAINT node_id IF NOT EXISTS FOR (n:Entity) REQUIRE n.id IS UNIQUE');
            await runWithRetry(session, 'CREATE INDEX node_label IF NOT EXISTS FOR (n:Entity) ON (n.label)');
            await runWithRetry(session, 'CREATE INDEX node_file IF NOT EXISTS FOR (n:Entity) ON (n.file)');
        } catch (e) {
            console.warn('Constraint creation warning:', e.message);
        }
    } else {
        console.log("Incremental mode: Skipping clear and constraint creation.");
    }
    
    // 3. Import Nodes
    await processStream(NODES_FILE, session, 'nodes', async (sess, type, items) => {
        // MERGE logic for nodes
        // We set labels dynamically using the type
        await runWithRetry(sess,
            `
            UNWIND $batch AS row
            MERGE (n:Entity {id: row.id})
            SET n += row, n:${type}
            `,
            { batch: items }
        );
    });

    // 4. Import Edges
    await processStream(EDGES_FILE, session, 'edges', async (sess, type, items) => {
        // MERGE logic for edges
        await runWithRetry(sess,
            `
            UNWIND $batch AS row
            MATCH (source:Entity {id: row.source})
            MATCH (target:Entity {id: row.target})
            MERGE (source)-[r:${type}]->(target)
            // Can add properties to edges if any exist in row
            `,
            { batch: items }
        );
    });

    // 5. Update Graph State
    try {
        console.log("Updating Graph State...");
        const commitHash = execSync('git rev-parse HEAD', { cwd: ROOT_DIR }).toString().trim();
        await neo4jService.updateGraphState(commitHash);
        console.log(`Graph state updated to commit: ${commitHash}`);
    } catch (e) {
        console.error("Failed to update graph state:", e);
    }

    // 6. Verification
    const countResult = await runWithRetry(session, 'MATCH (n) RETURN count(n) as c');
    const edgeResult = await runWithRetry(session, 'MATCH ()-[r]->() RETURN count(r) as c');
    console.log('Final Verification:');
    console.log(`Total Nodes in DB: ${neo4jService.toNum(countResult.records[0].get('c'))}`);
    console.log(`Total Edges in DB: ${neo4jService.toNum(edgeResult.records[0].get('c'))}`);

  } catch (error) {
    console.error('Error during import:', error);
    process.exit(1);
  } finally {
    await session.close();
    await neo4jService.close();
  }
}

run();