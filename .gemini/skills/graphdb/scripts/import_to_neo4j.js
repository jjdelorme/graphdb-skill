const neo4jService = require('./Neo4jService');
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

// Configuration - look for nodes/edges relative to project root or in standard location
const ROOT_DIR = path.resolve(__dirname, '../../../../');
const NODES_FILE = process.argv[2] || path.join(ROOT_DIR, '.gemini/graph_data/nodes.json');
const EDGES_FILE = process.argv[3] || path.join(ROOT_DIR, '.gemini/graph_data/edges.json');

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
            await new Promise(res => setTimeout(res, 1000 * attempts)); // Backoff
        }
    }
}

async function run() {
  const session = neo4jService.getSession();
  
  try {
    console.log(`Connecting to Neo4j at ${neo4jService.uri}...`);
    
    // 1. Clear Database
    console.log('Clearing existing database...');
    try {
        await runWithRetry(session, 'CALL { MATCH (n) DETACH DELETE n } IN TRANSACTIONS OF 10000 ROWS');
    } catch (e) {
        console.warn('Batched delete failed, trying iterative delete:', e.message);
        // Fallback doesn't need retry logic as it's already a fallback
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
    
    // 3. Load Nodes
    if (!fs.existsSync(NODES_FILE)) {
        throw new Error(`Nodes file not found: ${NODES_FILE}`);
    }
    console.log(`Reading ${NODES_FILE}...`);
    const nodesData = JSON.parse(fs.readFileSync(NODES_FILE, 'utf8'));
    console.log(`Loaded ${nodesData.length} nodes from file.`);

    console.log('Importing nodes...');
    const nodeBatches = chunkArray(nodesData, BATCH_SIZE);
    
    for (let i = 0; i < nodeBatches.length; i++) {
        const batch = nodeBatches[i];
        const byType = {};
        batch.forEach(node => {
            const type = node.type || 'Unknown';
            if (!byType[type]) byType[type] = [];
            byType[type].push(node);
        });

        for (const [type, typeNodes] of Object.entries(byType)) {
             const safeLabel = type.replace(/[^a-zA-Z0-9_]/g, '');
             await runWithRetry(session,
                `
                UNWIND $batch AS row
                MERGE (n:Entity {id: row.id})
                SET n += row, n:${safeLabel}
                `,
                { batch: typeNodes }
             );
        }
        process.stdout.write(`\rImported node batch ${i + 1}/${nodeBatches.length}`);
    }
    console.log('\nNodes imported.');

    // 4. Load Edges
    if (fs.existsSync(EDGES_FILE)) {
        console.log(`Reading ${EDGES_FILE}...`);
        const edgesData = JSON.parse(fs.readFileSync(EDGES_FILE, 'utf8'));
        console.log(`Loaded ${edgesData.length} edges from file.`);

        console.log('Importing edges...');
        const edgeBatches = chunkArray(edgesData, BATCH_SIZE);

        for (let i = 0; i < edgeBatches.length; i++) {
            const batch = edgeBatches[i];
            const byType = {};
            batch.forEach(edge => {
                const type = edge.type || 'RELATED_TO';
                if (!byType[type]) byType[type] = [];
                byType[type].push(edge);
            });

            for (const [type, typeEdges] of Object.entries(byType)) {
                const safeType = type.replace(/[^a-zA-Z0-9_]/g, '');
                await runWithRetry(session,
                    `
                    UNWIND $batch AS row
                    MATCH (source:Entity {id: row.source})
                    MATCH (target:Entity {id: row.target})
                    MERGE (source)-[r:${safeType}]->(target)
                    `,
                    { batch: typeEdges }
                );
            }
            process.stdout.write(`\rImported edge batch ${i + 1}/${edgeBatches.length}`);
        }
        console.log('\nEdges imported.');
    } else {
        console.warn(`Edges file not found: ${EDGES_FILE}`);
    }

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

function chunkArray(array, size) {
  const chunks = [];
  for (let i = 0; i < array.length; i += size) {
    chunks.push(array.slice(i, i + size));
  }
  return chunks;
}

run();