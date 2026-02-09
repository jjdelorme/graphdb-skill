const fs = require('fs');
const path = require('path');
const crypto = require('crypto');

class GraphBuilder {
    constructor(config) {
        this.config = config; // { root, outputDir, adapters: { cpp: Adapter, ... } }
        
        // Ensure output directory exists
        if (!fs.existsSync(this.config.outputDir)) {
            fs.mkdirSync(this.config.outputDir, { recursive: true });
        }

        // Initialize Streams
        this.nodesStream = fs.createWriteStream(path.join(this.config.outputDir, 'nodes.jsonl'));
        this.edgesStream = fs.createWriteStream(path.join(this.config.outputDir, 'edges.jsonl'));

        // Registry for Pass 2 (still needed for context, but kept minimal)
        // Ideally this should also be offloaded or minimized, but for now we keep globals.
        this.definedGlobals = new Set();
    }

    // Helper: Generate Deterministic ID
    _generateId(type, name) {
        return crypto.createHash('md5').update(`${type}:${name}`).digest('hex');
    }

    // Helper: Get or Create Node ID (Stateless Emit)
    getNode(type, name, file, metadata = {}) {
        const id = this._generateId(type, name);
        
        const node = { id, label: name, type, ...metadata };
        if (file) node.file = file;

        // Emit immediately
        this.nodesStream.write(JSON.stringify(node) + '\n');
        
        return id;
    }

    addEdge(sourceId, targetId, type) {
        const edge = { source: sourceId, target: targetId, type };
        this.edgesStream.write(JSON.stringify(edge) + '\n');
    }

    async run(fileList, skipWrite = false) { // skipWrite arg is deprecated but kept for signature compatibility
        console.log(`Starting Graph Build for ${fileList.length} files...`);

        // Initialize Adapters
        for (const key of Object.keys(this.config.adapters)) {
            console.log(`Initializing ${key} adapter...`);
            await this.config.adapters[key].init();
        }

        // --- Pass 1: Definitions ---
        console.log("Pass 1: Scanning Definitions...");
        let processed = 0;
        for (const file of fileList) {
            processed++;
            if (processed % 100 === 0) {
                console.log(`[Pass 1] Processing ${processed}/${fileList.length}`);
                if (global.gc) global.gc(); // Force GC to keep heap low
            }
            
            const adapter = this._getAdapterForFile(file);
            if (!adapter) continue;

            try {
                const source = fs.readFileSync(file, 'utf8');
                const tree = adapter.parse(source, file);
                const definitions = adapter.scanDefinitions(tree);
                
                const relPath = path.relative(this.config.root, file).replace(/\\/g, '/');
                const fileId = this.getNode('File', relPath, relPath);

                for (const def of definitions) {
                    const id = this.getNode(def.type, def.name, relPath, {
                        start_line: def.line,
                        end_line: def.end_line,
                        complexity: def.complexity,
                        inherits: def.inherits // metadata
                    });
                    this.addEdge(id, fileId, 'DEFINED_IN');
                    
                    if (def.type === 'Global') {
                        this.definedGlobals.add(def.name);
                    }
                    
                    // Handle Inheritance edges immediately
                    if (def.inherits) {
                        def.inherits.forEach(baseName => {
                            const baseId = this.getNode('Class', baseName, null);
                            this.addEdge(id, baseId, 'INHERITS_FROM');
                        });
                        // Note: We emit the node with 'inherits' metadata above. 
                        // The DB merge logic should handle cleanup if we want, 
                        // but keeping it in metadata is harmless.
                    }
                }
                
                // Cleanup
                if (tree.type === 'single' && tree.tree && tree.tree.delete) {
                    tree.tree.delete();
                } else if (tree.type === 'chunked' && tree.chunks) {
                    tree.chunks.forEach(chunk => {
                        if (chunk.tree && chunk.tree.delete) chunk.tree.delete();
                    });
                } else if (tree.delete) {
                    tree.delete();
                }
            } catch (e) {
                console.error(`Error in Pass 1 for ${file}:`, e);
            }
        }
        console.log(`Pass 1 Complete. Found ${this.definedGlobals.size} globals.`);

        // --- Pass 2: References ---
        console.log("Pass 2: Scanning References...");
        processed = 0;
        for (const file of fileList) {
            processed++;
            if (processed % 100 === 0) {
                console.log(`[Pass 2] Processing ${processed}/${fileList.length}`);
                if (global.gc) global.gc();
            }
            
            const adapter = this._getAdapterForFile(file);
            if (!adapter) continue;

            try {
                const source = fs.readFileSync(file, 'utf8');
                const tree = adapter.parse(source, file);
                const references = adapter.scanReferences(tree, this.definedGlobals);
                
                for (const ref of references) {
                    const sourceId = this.getNode('Function', ref.source, null); // Re-generates ID
                    
                    if (ref.type === 'Call') {
                        const targetId = this.getNode('Function', ref.target, null);
                        this.addEdge(sourceId, targetId, 'CALLS');
                        
                        // MFC Heuristic
                        if (ref.target.startsWith('Afx') || ref.target === 'MessageBox') {
                            const mfcId = this.getNode('MFC_API', ref.target, null);
                            this.addEdge(sourceId, mfcId, 'CALLS_MFC');
                        }
                    } else if (ref.type === 'Usage') {
                        const targetId = this.getNode('Global', ref.target, null); // Should exist
                        this.addEdge(sourceId, targetId, 'USES_GLOBAL');
                    } else if (ref.type === 'ImplicitGlobalWrite') {
                         // New logic for implicit globals mentioned in plan
                         // "Copy ImplicitGlobalWrite handling (emitting inferred: true nodes)"
                         const globalId = this.getNode('Global', ref.target, null, { inferred: true });
                         this.addEdge(sourceId, globalId, 'WRITES_TO_GLOBAL');
                    }
                }

                // Cleanup
                if (tree.type === 'single' && tree.tree && tree.tree.delete) {
                    tree.tree.delete();
                } else if (tree.type === 'chunked' && tree.chunks) {
                    tree.chunks.forEach(chunk => {
                        if (chunk.tree && chunk.tree.delete) chunk.tree.delete();
                    });
                } else if (tree.delete) {
                    tree.delete();
                }
            } catch (e) {
                 console.error(`Error in Pass 2 for ${file}:`, e);
            }
        }

        // Close streams
        this.nodesStream.end();
        this.edgesStream.end();
        
        // Return dummy object for compatibility if needed, or just void
        return { nodes: [], edges: [] }; 
    }

    _getAdapterForFile(filePath) {
        const ext = path.extname(filePath).toLowerCase();
        if (ext === '.cpp' || ext === '.c' || ext === '.h' || ext === '.hpp') {
            return this.config.adapters.cpp;
        }
        if (ext === '.cs') {
            return this.config.adapters.csharp;
        }
        if (ext === '.vb') {
            return this.config.adapters.vb;
        }
        if (ext === '.sql') {
            return this.config.adapters.sql;
        }
        if (ext === '.aspx' || ext === '.cshtml' || ext === '.asp') {
            return this.config.adapters.asp;
        }
        if (ext === '.ts' || ext === '.tsx') {
            return this.config.adapters.ts;
        }
        return null;
    }
}

module.exports = GraphBuilder;
