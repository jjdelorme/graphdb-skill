require('dotenv').config({ path: '../../../../.env' });
const neo4j = require('neo4j-driver');
const VectorService = require('./services/VectorService');
const { program } = require('commander');

// Configuration
const DB_HOST = process.env.NEO4J_HOST || 'localhost';
const DB_USER = process.env.NEO4J_USER || 'neo4j';
const DB_PASS = process.env.NEO4J_PASSWORD || 'your_strong_password';
const DB_PORT = process.env.NEO4J_PORT || '7687';
const URI = `bolt://${DB_HOST}:${DB_PORT}`;
const INDEX_NAME = 'function_embeddings';

program
    .version('1.0.0')
    .requiredOption('-q, --query <text>', 'Natural language query or code snippet')
    .option('-l, --limit <number>', 'Number of results', '5')
    .parse(process.argv);

const options = program.opts();

async function run() {
    const driver = neo4j.driver(URI, neo4j.auth.basic(DB_USER, DB_PASS));
    const session = driver.session();
    const vectorService = new VectorService();

    try {
        console.log(`Generating embedding for: "${options.query}"`);
        const vectors = await vectorService.embedDocuments([options.query]);
        const queryVector = vectors[0];

        if (!queryVector) {
            console.error("Failed to generate embedding for query.");
            process.exit(1);
        }

        console.log("Searching graph...");
        
        // Vector Search Query
        const result = await session.run(`
            CALL db.index.vector.queryNodes('${INDEX_NAME}', ${parseInt(options.limit)}, $queryVector)
            YIELD node, score
            RETURN node.name as name, node.file as file, node.start_line as line, score
        `, { queryVector });

        console.log('\n--- Search Results ---');
        result.records.forEach(r => {
            const score = r.get('score');
            const name = r.get('name');
            const file = r.get('file');
            const line = r.get('line');
            
            console.log(`[${(score * 100).toFixed(1)}%] ${name}`);
            console.log(`       File: ${file}:${line}`);
        });
        console.log('----------------------');

    } catch (e) {
        console.error("Search failed:", e);
    } finally {
        await session.close();
        await driver.close();
    }
}

run();
