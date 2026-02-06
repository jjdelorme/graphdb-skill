const fs = require('fs');
const path = require('path');

class GraphBuilder {
    constructor(config) {
        this.config = config; // { root, outputDir, adapters: { cpp: Adapter, ... } }
        this.nodes = [];
        this.edges = [];
        this.nodeMap = new Map(); // "Type:Name" -> ID
        this.nodeIdCounter = 0;
        
        // Registry for Pass 2
        this.definedGlobals = new Set();
    }

    // Helper: Get or Create Node ID
    getNode(type, name, file, metadata = {}) {
        const key = `${type}:${name}`; 
        
        if (!this.nodeMap.has(key)) {
            const id = `n${this.nodeIdCounter++}`;
            const node = { id, label: name, type, ...metadata };
            if (file) node.file = file;
            this.nodes.push(node);
            this.nodeMap.set(key, id);
            return id;
        }
        
        const id = this.nodeMap.get(key);
        // Update existing node
        const index = parseInt(id.substring(1));
        const existingNode = this.nodes[index];
        
        if (existingNode) {
            if (file && !existingNode.file) existingNode.file = file;
            // Merge metadata if needed (e.g. line numbers)
            if (metadata.start_line && !existingNode.start_line) {
                 Object.assign(existingNode, metadata);
            }
        }
        return id;
    }

    addEdge(sourceId, targetId, type) {
        this.edges.push({ source: sourceId, target: targetId, type });
    }

    async run(fileList, skipWrite = false) {
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
            if (processed % 100 === 0) console.log(`[Pass 1] Processing ${processed}/${fileList.length}`);
            
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
                        // Remove intermediate metadata from the node object
                        const nodeObj = this.nodes[parseInt(id.substring(1))];
                        if (nodeObj && nodeObj.inherits) delete nodeObj.inherits;
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
            if (processed % 100 === 0) console.log(`[Pass 2] Processing ${processed}/${fileList.length}`);
            
            const adapter = this._getAdapterForFile(file);
            if (!adapter) continue;

            try {
                const source = fs.readFileSync(file, 'utf8');
                const tree = adapter.parse(source, file);
                const references = adapter.scanReferences(tree, this.definedGlobals);
                
                for (const ref of references) {
                    const sourceId = this.getNode('Function', ref.source, null); // Should exist from Pass 1
                    
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

        if (!skipWrite) {
            this._writeOutput();
        }
        return { nodes: this.nodes, edges: this.edges };
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

    _writeOutput() {
        const outDir = this.config.outputDir;
        if (!fs.existsSync(outDir)) fs.mkdirSync(outDir, { recursive: true });
        
        console.log(`Writing ${this.nodes.length} nodes and ${this.edges.length} edges...`);
        fs.writeFileSync(path.join(outDir, 'nodes.json'), JSON.stringify(this.nodes, null, 2));
        fs.writeFileSync(path.join(outDir, 'edges.json'), JSON.stringify(this.edges, null, 2));
        console.log("Done.");
    }
}

module.exports = GraphBuilder;
