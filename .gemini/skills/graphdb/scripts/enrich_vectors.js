const neo4jService = require('./Neo4jService');
const fs = require('fs');
const path = require('path');
const VectorService = require('./services/VectorService');

// Neo4j Vector Config
const VECTOR_DIMENSIONS = parseInt(process.env.GEMINI_EMBEDDING_DIMENSIONS || "768", 10);
const INDEX_NAME = 'function_embeddings';
const BATCH_SIZE = 200; // Processing batch size

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
                start: neo4jService.toNum(r.get('start')),
                end: neo4jService.toNum(r.get('end'))
            }));

            console.log(`Processing batch ${++batchCount} (${batch.length} items)...`);
            
            // Group by file to optimize I/O
            const fileGroups = {};
            for (const item of batch) {
                if (!fileGroups[item.file]) fileGroups[item.file] = [];
                fileGroups[item.file].push(item);
            }

            // Extract Source Code
            const textsToEmbed = [];
            const validItems = [];
            const errorUpdates = []; // { id, error }

            for (const relPath in fileGroups) {
                const groupItems = fileGroups[relPath];
                const absPath = path.resolve(__dirname, '../../../../', relPath);

                try {
                    if (fs.existsSync(absPath)) {
                        const content = fs.readFileSync(absPath, 'utf8');
                        const lines = content.split('\n');
                        
                        for (const item of groupItems) {
                            try {
                                // Tree-sitter lines are 1-based usually, check ExtractGraph logic.
                                // Pass 1: line: node.startPosition.row + 1. So yes, 1-based.
                                // Array slice is 0-based.
                                // Lines 1-based: Start=1 means Index=0.
                                const sourceCode = lines.slice(item.start - 1, item.end).join('\n');
                                
                                // Optimize: Add Function Name to context
                                const text = `Function Name: ${item.name}\nSource Code:\n${sourceCode}`;
                                textsToEmbed.push(text);
                                validItems.push(item);
                            } catch (e) {
                                console.error(`Error extracting source for ${item.name}: ${e.message}`);
                                errorUpdates.push({ id: item.id, error: e.message });
                            }
                        }
                    } else {
                        console.warn(`File not found: ${absPath}`);
                        for (const item of groupItems) {
                            errorUpdates.push({ id: item.id, error: 'File Not Found' });
                        }
                    }
                } catch (e) {
                    console.error(`Error reading file ${relPath}: ${e.message}`);
                    for (const item of groupItems) {
                        errorUpdates.push({ id: item.id, error: `File Read Error: ${e.message}` });
                    }
                }
            }

            // Commit I/O errors immediately
            if (errorUpdates.length > 0) {
                await session.run(`
                    UNWIND $updates as row
                    MATCH (f:Function {id: row.id})
                    SET f.embedding_error = row.error
                `, { updates: errorUpdates });
            }

            if (textsToEmbed.length > 0) {
                // Generate Embeddings
                const vectors = await vectorService.embedDocuments(textsToEmbed);

                // Write Back
                const updateBatch = [];
                const embeddingErrors = [];
                
                for (let i = 0; i < validItems.length; i++) {
                    const vector = vectors[i];
                    if (vector) {
                        updateBatch.push({
                            id: validItems[i].id,
                            embedding: vector
                        });
                    } else {
                        embeddingErrors.push(validItems[i].id);
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

                if (embeddingErrors.length > 0) {
                    await session.run(`
                        UNWIND $ids as id
                        MATCH (f:Function {id: id})
                        SET f.embedding_error = 'Embedding failed after retries'
                    `, { ids: embeddingErrors });
                    console.warn(`Flagged ${embeddingErrors.length} functions with embedding errors.`);
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