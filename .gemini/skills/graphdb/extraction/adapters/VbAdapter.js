const Parser = require('tree-sitter');
const VbNet = require('tree-sitter-vb-dotnet');

class VbAdapter {
    constructor() {
        this.parser = new Parser();
        this.parser.setLanguage(VbNet);
    }

    async init() {
        // Native parser doesn't need async init, but keeping interface consistent
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
            // Class/Structure
            if (node.type === 'class_block' || node.type === 'structure_block' || node.type === 'module_block') {
                 // Try to find name. In AST dump: class_block -> identifier (child 2 usually)
                 // But better to search children for identifier if no explicit field
                 let name = 'anonymous';
                 // Some grammars use specific fields, debug showed 'name' field in class_block?
                 // (name field found: MyClass)
                 const nameNode = node.childForFieldName('name');
                 if (nameNode) name = nameNode.text;
                 else {
                     // Fallback: look for first identifier after modifiers
                     for(let i=0; i<node.childCount; i++) {
                         if (node.child(i).type === 'identifier') {
                             name = node.child(i).text;
                             break;
                         }
                     }
                 }

                 const inherits = [];
                 // Inheritance Heuristic
                 for (let i = 0; i < node.childCount; i++) {
                     const child = node.child(i);
                     if (child.type === 'inherits_statement') {
                         // Extract type
                         const type = child.childForFieldName('type'); // Guess field name
                         if (type) inherits.push(type.text);
                         else {
                             // Iterate children to find type/identifier
                             for(let j=0; j<child.childCount; j++) {
                                 const c = child.child(j);
                                 if (c.type === 'type' || c.type === 'identifier' || c.type === 'qualified_name') {
                                     inherits.push(c.text);
                                 }
                             }
                         }
                     }
                     // Fallback for broken AST (Inherits as field)
                     if (child.type === 'field_declaration') {
                         const text = child.text.trim();
                         if (text.startsWith('Inherits')) {
                             // Check next sibling or content of this node?
                             // In debug output: field_declaration (Inherits) then field_declaration (Base)
                             // Or maybe "Inherits Base" is one node?
                             // If text is "Inherits", check next sibling
                             if (text === 'Inherits') {
                                 const next = node.child(i + 1);
                                 if (next && (next.type === 'field_declaration' || next.type === 'identifier')) {
                                     inherits.push(next.text.trim());
                                 }
                             } else {
                                 // "Inherits BaseClass" in one node?
                                 const parts = text.split(/\s+/);
                                 if (parts[0] === 'Inherits' && parts.length > 1) {
                                     inherits.push(parts[1]);
                                 }
                             }
                         }
                     }
                 }

                 definitions.push({
                     name: name,
                     type: 'Class',
                     line: node.startPosition.row + 1,
                     inherits: inherits
                 });
            }

            // Methods
            if (node.type === 'method_declaration' || node.type === 'sub_declaration' || node.type === 'function_declaration') {
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
             if (node.type === 'method_declaration' || node.type === 'sub_declaration' || node.type === 'function_declaration') {
                 const nameNode = node.childForFieldName('name');
                 if (nameNode) {
                     const funcName = nameNode.text;
                     const localScope = new Set();
                     this._collectLocals(node, localScope);
                     this._scanBodyForRefs(node, funcName, localScope, knownGlobals, references);
                 }
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
            'if_statement', 'select_statement', 'for_statement', 'while_statement', 
            'do_loop_statement', 'try_statement', 'catch_clause'
        ]);
        
        const walk = (n) => {
            if (branchingTypes.has(n.type)) complexity++;
            for (let i = 0; i < n.childCount; i++) walk(n.child(i));
        }
        walk(node);
        return complexity;
    }

    _collectLocals(node, localScope) {
        // Parameters
        const params = node.childForFieldName('parameters');
        if (params) {
            params.children.forEach(p => {
                const name = p.childForFieldName('name');
                if (name) localScope.add(name.text);
            });
        }

        // Dim statements
        const walk = (n) => {
            if (n.type === 'dim_statement' || n.type === 'local_declaration_statement') {
                 // In debug: dim_statement -> identifier
                 // or variable_declarator
                 n.children.forEach(child => {
                     if (child.type === 'identifier') localScope.add(child.text);
                     if (child.type === 'variable_declarator') {
                         const name = child.childForFieldName('name');
                         if (name) localScope.add(name.text);
                     }
                 });
            }
            for (let i = 0; i < n.childCount; i++) walk(n.child(i));
        }
        walk(node);
    }

    _scanBodyForRefs(node, sourceFunc, localScope, knownGlobals, refs) {
        // Function Calls
        if (node.type === 'call_statement' || node.type === 'invocation') {
            // debug: call_statement -> expression -> invocation
            // invocation -> target (identifier or member_access)
            
            // If we are at 'invocation' node
            let target = node.childForFieldName('target');
            if (!target && node.type === 'call_statement') {
                 // drill down
                 // call_statement -> expression -> invocation?
                 // Or just traverse children
            }
            
            if (target) {
                let calleeName = target.text;
                if (target.type === 'member_access') {
                    const member = target.childForFieldName('member');
                    if (member) calleeName = member.text;
                }
                refs.push({ source: sourceFunc, target: calleeName, type: 'Call' });
            }
        }
        
        // Variable Usage
        if (node.type === 'identifier') {
            const name = node.text;
            if (!localScope.has(name)) {
                // Determine if this identifier is part of a declaration or call target
                // If parent is dim_statement, it's a declaration (handled in locals)
                // If parent is invocation and this is target, it's a Call (handled above)
                
                let isUsage = true;
                if (node.parent) {
                    if (node.parent.type === 'dim_statement' || node.parent.type === 'variable_declarator') isUsage = false;
                    if (node.parent.type === 'invocation' && node.parent.childForFieldName('target').id === node.id) isUsage = false;
                    if (node.parent.type === 'member_access') {
                        // If we are the member part, we might be a call if parent is invocation
                        if (node.parent.parent && node.parent.parent.type === 'invocation') {
                             if (node.parent.childForFieldName('member').id === node.id) isUsage = false;
                        }
                    }
                }

                if (isUsage) {
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

module.exports = VbAdapter;
