const fs = require('fs');
const path = require('path');
const neo4jService = require('./Neo4jService');
const GraphBuilder = require('../extraction/core/GraphBuilder');
const CppAdapter = require('../extraction/adapters/CppAdapter');
const CsharpAdapter = require('../extraction/adapters/CsharpAdapter');
const VbAdapter = require('../extraction/adapters/VbAdapter');
const SqlAdapter = require('../extraction/adapters/SqlAdapter');
const AspAdapter = require('../extraction/adapters/AspAdapter');

// Paths
const ROOT_DIR = path.resolve(__dirname, '../../../../');
const CPP_WASM_PATH = path.join(__dirname, '../extraction/node_modules/tree-sitter-cpp/tree-sitter-cpp.wasm');
const CS_WASM_PATH = path.join(__dirname, '../extraction/node_modules/tree-sitter-c-sharp/tree-sitter-c_sharp.wasm');

async function updateFile(filePath) {
    // filePath can be relative or absolute. Resolve to absolute first.
    let absolutePath = path.isAbsolute(filePath) ? filePath : path.resolve(ROOT_DIR, filePath);
    
    // Calculate relative path for DB
    const relPath = path.relative(ROOT_DIR, absolutePath).replace(/\\/g, '/');

    console.error(`Surgical update for: ${relPath}`);

    // 1. Setup GraphBuilder for single file
    const cppAdapter = new CppAdapter(CPP_WASM_PATH);
    const csAdapter = new CsharpAdapter(CS_WASM_PATH);
    const vbAdapter = new VbAdapter();
    const sqlAdapter = new SqlAdapter();
    const aspAdapter = new AspAdapter({ csharp: csAdapter, vb: vbAdapter });

    const builder = new GraphBuilder({
        root: ROOT_DIR,
        outputDir: '/tmp/graph_ignored', // Won't be used
        adapters: {
            cpp: cppAdapter,
            csharp: csAdapter,
            vb: vbAdapter,
            sql: sqlAdapter,
            asp: aspAdapter
        }
    });

    // 2. Parse file
    // Check if file exists (it might have been deleted)
    if (!fs.existsSync(absolutePath)) {
        console.error("File deleted. Removing from graph...");
        await deleteFileFromGraph(relPath);
        return;
    }

    const { nodes, edges } = await builder.run([absolutePath], true); // true = skipWrite

    // 3. Update Neo4j
    const session = neo4jService.getSession();
    try {
        await session.writeTransaction(async tx => {
            // A. Delete old data for this file
            console.error("Deleting old subgraph...");
            await tx.run(`
                MATCH (f:File {file: $path})
                OPTIONAL MATCH (child)-[:DEFINED_IN]->(f)
                DETACH DELETE f, child
            `, { path: relPath });

            // B. Re-create File Node
            await tx.run(`
                MERGE (f:File {file: $path})
                SET f.updated_at = timestamp()
            `, { path: relPath });

            // C. Create Nodes
            // Separate nodes into "Internal" (defined in this file) and "External" (references)
            const internalNodes = nodes.filter(n => n.file === relPath && n.type !== 'File');
            
            console.error(`Creating ${internalNodes.length} internal nodes...`);
            for (const node of internalNodes) {
                const labels = node.type; // e.g. 'Function'
                
                // Properties to set
                const props = { ...node };
                delete props.id; // Internal ID
                delete props.type;
                
                // We use dynamic labels in Cypher via apoc or simple string injection if safe.
                // Since types are controlled (Function, Class), string injection is relatively safe here.
                // But safer is to use `CALL apoc.create.node`.
                // For simplicity and standard Cypher, we'll assume a limited set of types.
                // Actually, `node.type` comes from adapters.
                
                await tx.run(`
                    MATCH (f:File {file: $file})
                    CREATE (n:\`${labels}\` $props)
                    CREATE (n)-[:DEFINED_IN]->(f)
                `, { file: relPath, props });
            }

            // D. Create Edges
            console.error(`Creating ${edges.length} edges...`);
            for (const edge of edges) {
                if (edge.type === 'DEFINED_IN') continue;

                const sourceNode = nodes.find(n => n.id === edge.source);
                const targetNode = nodes.find(n => n.id === edge.target);

                if (!sourceNode || !targetNode) continue;

                // Source should be internal (since we only parsed this file)
                if (sourceNode.file !== relPath) continue;

                const sourceQuery = `MATCH (source:\`${sourceNode.type}\` {label: $sourceLabel, file: $sourceFile})`;
                
                let targetQuery = '';
                let targetParams = {};
                
                if (targetNode.file) {
                    targetQuery = `MATCH (target:\`${targetNode.type}\` {label: $targetLabel, file: $targetFile})`;
                    targetParams = { targetLabel: targetNode.label, targetFile: targetNode.file };
                } else {
                    targetQuery = `MATCH (target:\`${targetNode.type}\` {label: $targetLabel})`;
                    targetParams = { targetLabel: targetNode.label };
                }

                await tx.run(`
                    ${sourceQuery}
                    ${targetQuery}
                    MERGE (source)-[:${edge.type}]->(target)
                `, {
                    sourceLabel: sourceNode.label,
                    sourceFile: sourceNode.file,
                    ...targetParams
                });
            }
        });
        console.error("Update complete.");
    } catch (e) {
        console.error("Error updating Neo4j:", e);
        throw e;
    } finally {
        await session.close();
    }
}

async function deleteFileFromGraph(relPath) {
    const session = neo4jService.getSession();
    try {
        await session.run(`
            MATCH (f:File {file: $path})
            OPTIONAL MATCH (child)-[:DEFINED_IN]->(f)
            DETACH DELETE f, child
        `, { path: relPath });
    } finally {
        await session.close();
    }
}

// Allow running directly
if (require.main === module) {
    const filePath = process.argv[2];
    if (!filePath) {
        console.error("Usage: node update_file.js <file_path>");
        process.exit(1);
    }
    updateFile(filePath).then(() => {
        neo4jService.close();
    }).catch(e => {
        console.error(e);
        neo4jService.close();
        process.exit(1);
    });
}

module.exports = { updateFile };
