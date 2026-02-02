const Parser = require('tree-sitter');
const CSharp = require('tree-sitter-c-sharp');

class CsharpAdapter {
    constructor() {
        this.parser = new Parser();
        this.parser.setLanguage(CSharp);
    }

    async init() {
        // Native parser is synchronous, keeping interface consistent
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
            // Method Definitions
            if (node.type === 'method_declaration' || node.type === 'local_function_statement') {
                const nameNode = node.childForFieldName('name');
                if (nameNode) {
                    definitions.push({
                        name: nameNode.text,
                        type: 'Function',
                        line: node.startPosition.row + 1,
                        end_line: node.endPosition.row + 1,
                        complexity: this._calculateComplexity(node)
                    });
                }
            }

            // Class Definitions
            if (node.type === 'class_declaration' || node.type === 'struct_declaration') {
                 const nameNode = node.childForFieldName('name');
                 const className = nameNode ? nameNode.text : 'anonymous';
                 
                 const inherits = [];
                 const bases = node.childForFieldName('base_list') || node.children.find(c => c.type === 'base_list');
                 if (bases) {
                    bases.children.forEach(b => {
                        // base_list -> : -> type_identifier
                        // or qualified_name
                        if (b.type === 'identifier' || b.type === 'qualified_name' || b.type === 'type_identifier' || b.type === 'simple_type') {
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

            // "Global" Fields (Static fields)
            if (node.type === 'field_declaration') {
                // Check if static
                let isStatic = false;
                const modifiers = node.children.find(c => c.type === 'modifier' && c.text === 'static');
                if (modifiers) isStatic = true;

                if (isStatic) {
                     // field_declaration children might be variable_declaration or just multiple variable_declarator
                     // In C#: field_declaration -> variable_declaration -> variable_declarator
                     // OR field_declaration -> variable_declarator (depending on version/grammar)
                     
                     // Helper to find variable_declarators recursively in this node
                     const findVars = (n) => {
                         if (n.type === 'variable_declarator') {
                             const nameNode = n.childForFieldName('name');
                             if (nameNode) {
                                 definitions.push({
                                     name: nameNode.text,
                                     type: 'Global', // Treating static class fields as globals
                                     line: node.startPosition.row + 1
                                 });
                             }
                         }
                         for (let i = 0; i < n.childCount; i++) findVars(n.child(i));
                     };
                     findVars(node);
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
             if (node.type === 'method_declaration') {
                 const nameNode = node.childForFieldName('name');
                 if (nameNode) {
                     const funcName = nameNode.text;
                     const body = node.childForFieldName('body');
                     if (body) {
                         const localScope = new Set();
                         this._collectLocals(node, localScope);
                         this._scanBodyForRefs(body, funcName, localScope, knownGlobals, references);
                     }
                 }
                 // Do not descend into nested methods if any
             }
             
             for (let i = 0; i < node.childCount; i++) visit(node.child(i));
        };
        
        visit(tree.rootNode);
        return references;
    }

    // --- Helpers ---

    _calculateComplexity(node) {
        let complexity = 1;
        const branchingTypes = new Set([
            'if_statement', 'for_statement', 'foreach_statement', 'while_statement', 
            'switch_statement', 'catch_clause', 'conditional_expression'
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
        const params = node.childForFieldName('parameters');
        if (params) {
            params.children.forEach(p => {
                if (p.type === 'parameter') {
                    const nameNode = p.childForFieldName('name');
                    if (nameNode) localScope.add(nameNode.text);
                }
            });
        }

        // 2. Body declarations
        const body = node.childForFieldName('body');
        if (body) {
            const walkLocals = (n) => {
                if (n.type === 'local_declaration_statement') {
                    const findVars = (v) => {
                        if (v.type === 'variable_declarator') {
                             const name = v.childForFieldName('name');
                             if (name) localScope.add(name.text);
                        }
                        for(let i=0; i<v.childCount; i++) findVars(v.child(i));
                    };
                    findVars(n);
                }
                
                for (let i = 0; i < n.childCount; i++) walkLocals(n.child(i));
            }
            walkLocals(body);
        }
    }

    _scanBodyForRefs(node, sourceFunc, localScope, knownGlobals, refs) {
        // 1. Function Calls
        if (node.type === 'invocation_expression') {
            const funcNode = node.childForFieldName('function'); 
            if (funcNode) {
                let calleeName = funcNode.text;
                if (funcNode.type === 'member_access_expression') {
                     const nameNode = funcNode.childForFieldName('name');
                     if (nameNode) calleeName = nameNode.text;
                }
                refs.push({ source: sourceFunc, target: calleeName, type: 'Call' });
            }
        }
        
        // 2. Variable Usage
        if (node.type === 'identifier') {
            const name = node.text;
            if (!localScope.has(name)) {
                let isCallFunc = false;
                if (node.parent) {
                    if (node.parent.type === 'invocation_expression') {
                        const funcChild = node.parent.childForFieldName('function');
                        // if we are the function being called, we are not "using" a variable (unless we are a delegate, but let's skip that)
                        // Actually, if identifier is 'MyFunc', and we call 'MyFunc()', MyFunc is a Call, not Usage.
                        
                        // We need to check if 'node' is effectively the 'function' part of invocation
                        // The 'function' field might be the identifier itself.
                        if (funcChild && funcChild.id === node.id) isCallFunc = true;
                    }
                    if (node.parent.type === 'member_access_expression') {
                         // if we are the 'name' (rhs) of access, and parent is function of invocation...
                         if (node.parent.parent && node.parent.parent.type === 'invocation_expression') {
                             const funcChild = node.parent.parent.childForFieldName('function');
                             if (funcChild && funcChild.id === node.parent.id) {
                                 // We are 'Bar' in 'Foo.Bar()'. This is a Call part.
                                 if (node.parent.childForFieldName('name').id === node.id) isCallFunc = true;
                             }
                         }
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

module.exports = CsharpAdapter;
