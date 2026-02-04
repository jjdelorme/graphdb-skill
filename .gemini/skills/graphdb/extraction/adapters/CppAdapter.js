const Parser = require('tree-sitter');
const Cpp = require('tree-sitter-cpp');

class CppAdapter {
    constructor() {
        this.parser = new Parser();
        this.parser.setLanguage(Cpp);
    }

    async init() {
        // Native parser is synchronous
        return Promise.resolve();
    }

    parse(sourceCode) {
        // Safety: Truncate excessively large files (hard limit 1MB to prevent OOM)
        if (sourceCode.length > 1024 * 1024) {
            console.warn('[CppAdapter] File too large (1MB+), truncating.');
            sourceCode = sourceCode.substring(0, 1024 * 1024);
        }

        // Automatic Chunking for files > 30k chars (Tree-sitter 16-bit limit workaround)
        if (sourceCode.length > 30000) {
            console.log(`[CppAdapter] Large file detected (${sourceCode.length} chars). Using Chunking Strategy.`);
            return this._parseChunks(sourceCode);
        }

        try {
            const tree = this.parser.parse(sourceCode);
            return { type: 'single', tree: tree, source: sourceCode };
        } catch (e) {
            console.warn(`[CppAdapter] Tree-sitter parse failed: ${e.message}. Falling back to Regex scan.`);
            return { type: 'fallback', source: sourceCode };
        }
    }

    _parseChunks(source) {
        const chunks = [];
        // Split by "}\n" which is a common top-level delimiter in C/C++
        // We look for a closing brace at the start of a line or followed by newline
        const rawChunks = source.split(/\n}\s*\n/); 
        
        let lineOffset = 0;
        for (const rawChunk of rawChunks) {
            const chunkContent = rawChunk + "\n}\n"; // Re-add delimiter
            const chunkLines = chunkContent.split('\n').length - 1; // Approx
            
            try {
                // Parse chunk individually
                const tree = this.parser.parse(chunkContent);
                chunks.push({ 
                    tree: tree, 
                    lineOffset: lineOffset,
                    valid: true
                });
            } catch (e) {
                chunks.push({
                    source: chunkContent,
                    lineOffset: lineOffset,
                    valid: false
                });
            }
            
            lineOffset += chunkLines;
        }
        
        return { type: 'chunked', chunks: chunks };
    }

    /**
     * Pass 1: Identify Definitions
     */
    scanDefinitions(parseResult) {
        if (parseResult.type === 'fallback') {
            return this._scanDefinitionsRegex(parseResult.source);
        }

        const allDefinitions = [];

        if (parseResult.type === 'single') {
            return this._scanTreeDefinitions(parseResult.tree, 0);
        }

        if (parseResult.type === 'chunked') {
            for (const chunk of parseResult.chunks) {
                if (chunk.valid) {
                    const defs = this._scanTreeDefinitions(chunk.tree, chunk.lineOffset);
                    allDefinitions.push(...defs);
                } else {
                    // Fallback for this specific chunk
                    const defs = this._scanDefinitionsRegex(chunk.source);
                    defs.forEach(d => { 
                        d.line += chunk.lineOffset; 
                        d.end_line += chunk.lineOffset; 
                    });
                    allDefinitions.push(...defs);
                }
            }
        }

        return allDefinitions;
    }

    _scanTreeDefinitions(tree, lineOffset) {
        const definitions = [];
        const visit = (node) => {
            // Function Definitions
            if (node.type === 'function_definition') {
                const funcName = this._extractFunctionName(node);
                if (funcName) {
                    definitions.push({
                        name: funcName,
                        type: 'Function',
                        line: node.startPosition.row + 1 + lineOffset,
                        end_line: node.endPosition.row + 1 + lineOffset,
                        complexity: this._calculateComplexity(node)
                    });
                }
            }
            // Class Definitions
            else if (node.type === 'class_specifier' || node.type === 'struct_specifier') {
                 const nameNode = node.childForFieldName('name');
                 const className = nameNode ? nameNode.text : 'anonymous';
                 
                 const inherits = [];
                 const bases = node.children.find(c => c.type === 'base_class_clause');
                 if (bases) {
                    bases.children.forEach(b => {
                        if (b.type === 'type_identifier' || b.type === 'qualified_identifier') {
                            inherits.push(b.text);
                        }
                    });
                 }

                 definitions.push({
                     name: this._truncateLabel(className),
                     type: 'Class',
                     line: node.startPosition.row + 1 + lineOffset,
                     inherits: inherits
                 });
            }
            // Declarations
            else if (node.type === 'declaration' && this._isTopLevel(node)) {
                const name = this._extractDeclarationName(node);
                if (name) {
                    definitions.push({
                        name: name,
                        type: 'Global',
                        line: node.startPosition.row + 1 + lineOffset
                    });
                }
                const funcDeclName = this._extractFunctionDeclarationName(node);
                if (funcDeclName) {
                     definitions.push({
                        name: funcDeclName,
                        type: 'Function',
                        line: node.startPosition.row + 1 + lineOffset,
                        is_definition: false,
                        complexity: 0
                    });
                }
            }

            for (let i = 0; i < node.childCount; i++) visit(node.child(i));
        };
        visit(tree.rootNode);
        return definitions;
    }

    /**
     * Pass 2: Identify References
     */
    scanReferences(parseResult, knownGlobals) {
        if (parseResult.type === 'fallback') return []; 

        const allReferences = [];
        
        if (parseResult.type === 'single') {
            return this._scanTreeReferences(parseResult.tree, knownGlobals);
        }

        if (parseResult.type === 'chunked') {
            for (const chunk of parseResult.chunks) {
                if (chunk.valid) {
                    const refs = this._scanTreeReferences(chunk.tree, knownGlobals);
                    allReferences.push(...refs);
                }
            }
        }
        
        return allReferences;
    }

    _scanTreeReferences(tree, knownGlobals) {
        const references = [];
        const visit = (node) => {
             if (node.type === 'function_definition') {
                 const funcName = this._extractFunctionName(node);
                 if (funcName) {
                     const body = node.childForFieldName('body');
                     if (body) {
                         const localScope = new Set();
                         this._collectLocals(node, localScope);
                         this._scanBodyForRefs(body, funcName, localScope, knownGlobals, references);
                     }
                 }
                 return; 
             }
             for (let i = 0; i < node.childCount; i++) visit(node.child(i));
        };
        visit(tree.rootNode);
        return references;
    }

    // --- Helpers ---

    _truncateLabel(label) {
        if (!label) return label;
        if (label.length > 128) return label.substring(0, 125) + '...';
        return label;
    }


    // --- Helpers ---

    _scanDefinitionsRegex(source) {
        const definitions = [];
        const lines = source.split(/\r?\n/);
        
        // Regex to capture: ReturnType FunctionName(Args)
        // Group 2 is the function name
        const funcRegex = /^\s*(?:(?:virtual|static|inline|friend)\s+)*(?:(?:[\w:*&<>]|::)+\s+)+([*&]?\w+)\s*\(/;

        for (let i = 0; i < lines.length; i++) {
            const line = lines[i];
            if (line.trim().startsWith('//')) continue;
            if (line.trim().startsWith('#')) continue; 

            const match = line.match(funcRegex);
            if (match) {
                // Ensure it's not a control structure like "if ("
                const name = match[1].replace(/[*&]/g, ''); // Remove pointer/ref chars if caught
                if (['if', 'while', 'for', 'switch', 'catch'].includes(name)) continue;

                // Basic heuristic: check if line ends with '{' or just assumes it's a definition if it looks like one
                // This is "optimistic" scanning
                definitions.push({
                    name: name,
                    type: 'Function',
                    line: i + 1,
                    end_line: i + 1, // Cannot determine end reliably
                    complexity: 1
                });
            }
        }
        return definitions;
    }

    _isTopLevel(node) {
        let current = node.parent;
        while (current) {
            if (current.type === 'function_definition' || current.type === 'lambda_expression') {
                return false;
            }
            if (current.type === 'translation_unit') {
                return true;
            }
            current = current.parent;
        }
        return true;
    }

    _extractFunctionDeclarationName(node) {
        // Look for function_declarator inside declaration
        // Iterative search to avoid stack overflow
        const stack = [node];
        
        while (stack.length > 0) {
            const n = stack.pop();
            
            if (n.type === 'function_declarator') {
                let d = n.childForFieldName('declarator');
                while (d && (d.type === 'pointer_declarator' || d.type === 'reference_declarator')) {
                     d = d.childForFieldName('declarator');
                }
                return d ? d.text : null;
            }
            
            // Push children in reverse order to preserve traversal order (optional but good)
            for (let i = n.childCount - 1; i >= 0; i--) {
                stack.push(n.child(i));
            }
        }
        return null;
    }

    _extractFunctionName(node) {
        const declarator = node.childForFieldName('declarator');
        if (!declarator) return null;

        let d = declarator;
        // Drill down to identifier (skipping pointers/refs)
        while (d.type === 'function_declarator' || d.type === 'pointer_declarator' || d.type === 'reference_declarator') {
             const sub = d.childForFieldName('declarator');
             if (sub) d = sub; else break;
        }
        return this._truncateLabel(d.text);
    }

    _extractDeclarationName(node) {
        const declarator = node.children.find(c => c.type === 'init_declarator' || c.type === 'identifier' || c.type === 'array_declarator');
        let name = null;
        if (declarator) {
             if (declarator.type === 'init_declarator') {
                 const sub = declarator.childForFieldName('declarator');
                 if (sub) name = sub.text.split('[')[0]; 
             } else if (declarator.type === 'array_declarator') {
                 const sub = declarator.childForFieldName('declarator');
                 if (sub) name = sub.text;
             } else {
                 name = declarator.text;
             }
        }
        return name ? this._truncateLabel(name.replace(/[*&]/g, '').trim()) : null;
    }

    _calculateComplexity(node) {
        let complexity = 1;
        const branchingTypes = new Set([
            'if_statement', 'for_statement', 'while_statement', 
            'case_statement', 'catch_clause', 'conditional_expression'
        ]);
        
        const walk = (n) => {
            if (branchingTypes.has(n.type)) complexity++;
            if (n.type === 'binary_expression') {
                const op = n.childForFieldName('operator');
                if (op && (op.text === '&&' || op.text === '||')) complexity++;
            }
            for (let i = 0; i < n.childCount; i++) walk(n.child(i));
        }
        walk(node);
        return complexity;
    }

    _collectLocals(node, localScope) {
        // 1. Parameters
        const declarator = node.childForFieldName('declarator');
        if (declarator) {
             let d = declarator;
             while (d && (d.type === 'pointer_declarator' || d.type === 'reference_declarator')) {
                 d = d.childForFieldName('declarator');
             }
             
             if (d && d.type === 'function_declarator') {
                 const params = d.childForFieldName('parameters');
                 if (params) {
                     params.children.forEach(p => {
                         if (p.type === 'parameter_declaration') {
                             const pNameNode = p.childForFieldName('declarator');
                             if (pNameNode) {
                                 localScope.add(pNameNode.text.replace(/[*&]/g, '').trim());
                             }
                         }
                     });
                 }
             }
        }

        // 2. Body declarations
        const body = node.childForFieldName('body');
        if (body) {
            const walkLocals = (n) => {
                if (n.type === 'declaration') {
                    const initDecl = n.children.find(c => c.type === 'init_declarator');
                    if (initDecl) {
                        const id = initDecl.childForFieldName('declarator');
                        if (id) {
                             const name = id.text.split('=')[0].trim().replace(/[*&]/g, '');
                             localScope.add(name);
                        }
                    } else {
                         const id = n.children.find(c => c.type === 'identifier');
                         if (id) localScope.add(id.text);
                    }
                }
                if (n.type === 'init_declarator') {
                     const id = n.childForFieldName('declarator');
                     if (id) localScope.add(id.text.replace(/[*&]/g, '').trim());
                }
                
                for (let i = 0; i < n.childCount; i++) walkLocals(n.child(i));
            }
            walkLocals(body);
        }
    }

    _scanBodyForRefs(node, sourceFunc, localScope, knownGlobals, refs) {
        // 1. Function Calls
        if (node.type === 'call_expression') {
            const funcNode = node.childForFieldName('function');
            if (funcNode) {
                const calleeName = funcNode.text;
                refs.push({ source: sourceFunc, target: calleeName, type: 'Call' });
            }
        }
        
        // 2. Variable Usage
        if (node.type === 'identifier') {
            const name = node.text;
            if (!localScope.has(name)) {
                // Check if it's the function name in a call (don't count as usage)
                let isCallFunc = false;
                if (node.parent && node.parent.type === 'call_expression') {
                     const funcChild = node.parent.childForFieldName('function');
                     if (funcChild && funcChild.id === node.id) {
                         isCallFunc = true;
                     }
                }
                
                if (!isCallFunc) {
                    if (knownGlobals.has(name)) {
                         refs.push({ source: sourceFunc, target: name, type: 'Usage' });
                    }
                }
            }
        }
        
        for (let i = 0; i < node.childCount; i++) {
             this._scanBodyForRefs(node.child(i), sourceFunc, localScope, knownGlobals, refs);
        }
    }
}

module.exports = CppAdapter;
