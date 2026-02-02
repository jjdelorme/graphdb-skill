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
        return this.parser.parse(sourceCode);
    }

    /**
     * Pass 1: Identify Definitions
     */
    scanDefinitions(tree) {
        const definitions = [];
        
        const visit = (node) => {
            // Function Definitions
            if (node.type === 'function_definition') {
                const funcName = this._extractFunctionName(node);
                if (funcName) {
                    definitions.push({
                        name: funcName,
                        type: 'Function',
                        line: node.startPosition.row + 1,
                        end_line: node.endPosition.row + 1,
                        complexity: this._calculateComplexity(node)
                    });
                }
            }

            // Class Definitions
            if (node.type === 'class_specifier' || node.type === 'struct_specifier') {
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
                     name: className,
                     type: 'Class',
                     line: node.startPosition.row + 1,
                     inherits: inherits
                 });
            }

            // Global Variables
            if (node.type === 'declaration' && node.parent.type === 'translation_unit') {
                const name = this._extractDeclarationName(node);
                if (name) {
                    definitions.push({
                        name: name,
                        type: 'Global',
                        line: node.startPosition.row + 1
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
    scanReferences(tree, knownGlobals) {
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
                 return; // Do not descend into nested functions (lambdas handled in body scan?)
             }
             
             for (let i = 0; i < node.childCount; i++) visit(node.child(i));
        };
        
        visit(tree.rootNode);
        return references;
    }

    // --- Helpers ---

    _extractFunctionName(node) {
        const declarator = node.childForFieldName('declarator');
        if (!declarator) return null;

        let d = declarator;
        // Drill down to identifier (skipping pointers/refs)
        while (d.type === 'function_declarator' || d.type === 'pointer_declarator' || d.type === 'reference_declarator') {
             const sub = d.childForFieldName('declarator');
             if (sub) d = sub; else break;
        }
        return d.text;
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
        return name ? name.replace(/[*&]/g, '').trim() : null;
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
