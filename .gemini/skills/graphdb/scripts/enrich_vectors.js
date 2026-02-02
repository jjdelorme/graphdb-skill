require('dotenv').config({ path: '../../../../.env' });
const neo4j = require('neo4j-driver');
const fs = require('fs');
const path = require('path');
const VectorService = require('./services/VectorService');

// Configuration
const DB_HOST = process.env.NEO4J_HOST || 'localhost';
const DB_USER = process.env.NEO4J_USER || 'neo4j';
const DB_PASS = process.env.NEO4J_PASSWORD || 'your_strong_password';
const DB_PORT = process.env.NEO4J_PORT || '7687';
const URI = `bolt://${DB_HOST}:${DB_PORT}`;

// Neo4j Vector Config
const VECTOR_DIMENSIONS = 768; // Gemini Embedding 001
const INDEX_NAME = 'function_embeddings';
const BATCH_SIZE = 50; // Processing batch size

async function run() {
    const driver = neo4j.driver(URI, neo4j.auth.basic(DB_USER, DB_PASS));
    const session = driver.session();
    const vectorService = new VectorService();

    try {
        console.log(`Connecting to Neo4j at ${URI}...`);

        // 1. Create Vector Index
        console.log(`Creating Vector Index '${INDEX_NAME}'...`);
        // Note: Syntax for 5.x. IF NOT EXISTS is key.
        // We use the default cosine similarity.
        const createIndexQuery = `
            CREATE VECTOR INDEX ${INDEX_NAME} IF NOT EXISTS
            FOR (n:Function)
            ON (n.embedding)
            OPTIONS {
                indexConfig: {
                    \`vector.dimensions\`: ${VECTOR_DIMENSIONS},
                    \`vector.similarity_function\`: 'cosine'
                }
            }
        `;
        await session.run(createIndexQuery);

        // 2. Processing Loop
        let processedCount = 0;
        let batchCount = 0;

        while (true) {
            // Fetch batch of functions that have a file but NO embedding
            const result = await session.run(`
                MATCH (f:Function)
                WHERE f.file IS NOT NULL 
                  AND f.embedding IS NULL
                  AND f.start_line IS NOT NULL
                  AND f.end_line IS NOT NULL
                RETURN f.id as id, f.name as name, f.file as file, f.start_line as start, f.end_line as end
                LIMIT ${BATCH_SIZE}
            `);

            if (result.records.length === 0) {
                console.log("No more functions to process.");
                break;
            }

            const batch = result.records.map(r => ({
                id: r.get('id'),
                name: r.get('name'),
                file: r.get('file'),
                start: r.get('start').toNumber(),
                end: r.get('end').toNumber()
            }));

            console.log(`Processing batch ${++batchCount} (${batch.length} items)...`);
            
            // Extract Source Code
            const textsToEmbed = [];
            const validItems = [];

            for (const item of batch) {
                try {
                    // Resolve file path relative to Project Root (4 levels up from script)
                    // item.file is usually stored as relative path "VIEW/..."
                    const absPath = path.resolve(__dirname, '../../../../', item.file);
                    
                    if (fs.existsSync(absPath)) {
                        const content = fs.readFileSync(absPath, 'utf8');
                        const lines = content.split('\n');
                        // Tree-sitter lines are 1-based usually, check ExtractGraph logic.
                        // Pass 1: line: node.startPosition.row + 1. So yes, 1-based.
                        // Array slice is 0-based.
                        // Lines 1-based: Start=1 means Index=0.
                        const sourceCode = lines.slice(item.start - 1, item.end).join('\n');
                        
                        // Optimize: Add Function Name to context
                        const text = `Function Name: ${item.name}\nSource Code:\n${sourceCode}`;
                        textsToEmbed.push(text);
                        validItems.push(item);
                    } else {
                        console.warn(`File not found: ${absPath}`);
                        // Mark as processed (e.g. set dummy embedding or ignore flag) to avoid infinite loop
                        // For now, we'll set a generic 'ignore' property if file missing, but let's just skip this item 
                        // and HOPE we don't pick it up again? No, query will pick it up again.
                        // We must flag it.
                        await session.run("MATCH (f:Function {id: $id}) SET f.embedding_error = 'File Not Found'", { id: item.id });
                    }
                } catch (e) {
                    console.error(`Error reading source for ${item.name}: ${e.message}`);
                }
            }

            if (textsToEmbed.length > 0) {
                // Generate Embeddings
                const vectors = await vectorService.embedDocuments(textsToEmbed);

                // Write Back
                const updateBatch = [];
                for (let i = 0; i < validItems.length; i++) {
                    const vector = vectors[i];
                    if (vector) {
                        updateBatch.push({
                            id: validItems[i].id,
                            embedding: vector
                        });
                    }
                }

                if (updateBatch.length > 0) {
                    await session.run(`
                        UNWIND $batch as row
                        MATCH (f:Function {id: row.id})
                        SET f.embedding = row.embedding
                    `, { batch: updateBatch });
                    
                    processedCount += updateBatch.length;
                    console.log(`Updated ${updateBatch.length} functions.`);
                }
            }
        }

        console.log(`Finished. Total functions enriched: ${processedCount}`);

    } catch (e) {
        console.error("Error in enrichment:", e);
    } finally {
        await session.close();
        await driver.close();
    }
}

run();
