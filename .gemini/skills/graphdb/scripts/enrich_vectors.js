const neo4jService = require('./Neo4jService');
const fs = require('fs');
const path = require('path');
const VectorService = require('./services/VectorService');

// Neo4j Vector Config
const VECTOR_DIMENSIONS = 768; // Gemini Embedding 001
const INDEX_NAME = 'function_embeddings';
const BATCH_SIZE = 50; // Processing batch size

async function run() {
    const session = neo4jService.getSession();
    const vectorService = new VectorService();

    try {
        console.log(`Connected to Neo4j.`);

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
            // AND are not flagged as errors
            // AND are not in node_modules
            const result = await session.run(`
                MATCH (f:Function)
                WHERE f.file IS NOT NULL 
                  AND f.embedding IS NULL
                  AND f.embedding_error IS NULL
                  AND f.start_line IS NOT NULL
                  AND f.end_line IS NOT NULL
                  AND NOT f.file CONTAINS 'node_modules'
                RETURN f.id as id, f.label as name, f.file as file, f.start_line as start, f.end_line as end
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
                        await session.run("MATCH (f:Function {id: $id}) SET f.embedding_error = 'File Not Found'", { id: item.id });
                    }
                } catch (e) {
                    console.error(`Error reading source for ${item.name}: ${e.message}`);
                     await session.run("MATCH (f:Function {id: $id}) SET f.embedding_error = $err", { id: item.id, err: e.message });
                }
            }

            if (textsToEmbed.length > 0) {
                // Generate Embeddings
                const vectors = await vectorService.embedDocuments(textsToEmbed);

                // Write Back
                const updateBatch = [];
                const errorBatch = [];
                for (let i = 0; i < validItems.length; i++) {
                    const vector = vectors[i];
                    if (vector) {
                        updateBatch.push({
                            id: validItems[i].id,
                            embedding: vector
                        });
                    } else {
                        errorBatch.push(validItems[i].id);
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

                if (errorBatch.length > 0) {
                    await session.run(`
                        UNWIND $ids as id
                        MATCH (f:Function {id: id})
                        SET f.embedding_error = 'Embedding failed after retries'
                    `, { ids: errorBatch });
                    console.warn(`Flagged ${errorBatch.length} functions with errors.`);
                }
            }
        }

        console.log(`Finished. Total functions enriched: ${processedCount}`);

    } catch (e) {
        console.error("Error in enrichment:", e);
    } finally {
        await session.close();
        await neo4jService.close();
    }
}

run();