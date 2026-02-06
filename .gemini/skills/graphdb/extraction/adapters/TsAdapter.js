const Parser = require('tree-sitter');
const { typescript } = require('tree-sitter-typescript');

class TsAdapter {
    constructor() {
        this.parser = new Parser();
        this.parser.setLanguage(typescript);
    }

    async init() {
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
            // Function Declarations
            if (node.type === 'function_declaration' || node.type === 'generator_function_declaration') {
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

            // Method Definitions (in classes/interfaces)
            if (node.type === 'method_definition') {
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

            // Arrow Functions / Function Expressions assigned to variables
            if (node.type === 'variable_declarator') {
                const nameNode = node.childForFieldName('name');
                const valueNode = node.childForFieldName('value');
                
                if (nameNode && valueNode && (valueNode.type === 'arrow_function' || valueNode.type === 'function_expression')) {
                    definitions.push({
                        name: nameNode.text,
                        type: 'Function',
                        line: node.startPosition.row + 1,
                        end_line: node.endPosition.row + 1,
                        complexity: this._calculateComplexity(valueNode)
                    });
                }
            }

            // Class & Interface Definitions
            if (node.type === 'class_declaration' || node.type === 'interface_declaration') {
                 const nameNode = node.childForFieldName('name');
                 const className = nameNode ? nameNode.text : 'anonymous';
                 
                 const inherits = [];
                 
                 // Classes: class_heritage -> extends_clause / implements_clause
                 // Interfaces: extends_type_clause
                 
                 const heritage = node.children.find(c => c.type === 'class_heritage');
                 const interfaceExtends = node.children.find(c => c.type === 'extends_type_clause');
                 
                 const collectTypes = (clause) => {
                     clause.children.forEach(c => {
                         if (c.type === 'type_identifier' || c.type === 'nested_type_identifier' || c.type === 'generic_type' || c.type === 'identifier') {
                             inherits.push(c.text);
                         }
                     });
                 };

                 if (heritage) {
                     const extendsClause = heritage.children.find(c => c.type === 'extends_clause');
                     const implementsClause = heritage.children.find(c => c.type === 'implements_clause');
                     if (extendsClause) collectTypes(extendsClause);
                     if (implementsClause) collectTypes(implementsClause);
                 }
                 
                 if (interfaceExtends) {
                     collectTypes(interfaceExtends);
                 }

                 definitions.push({
                     name: className,
                     type: 'Class',
                     line: node.startPosition.row + 1,
                     inherits: inherits
                 });
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
             // We start scanning contexts (Functions/Methods)
             let sourceFunc = null;
             let bodyNode = null;

             if (node.type === 'function_declaration' || node.type === 'generator_function_declaration' || node.type === 'method_definition') {
                 const nameNode = node.childForFieldName('name');
                 if (nameNode) sourceFunc = nameNode.text;
                 bodyNode = node.childForFieldName('body');
             }
             else if (node.type === 'variable_declarator') {
                 const valueNode = node.childForFieldName('value');
                 if (valueNode && (valueNode.type === 'arrow_function' || valueNode.type === 'function_expression')) {
                     const nameNode = node.childForFieldName('name');
                     if (nameNode) sourceFunc = nameNode.text;
                     bodyNode = valueNode.childForFieldName('body');
                 }
             }

             if (sourceFunc && bodyNode) {
                 const localScope = new Set();
                 this._collectLocals(node, localScope); // Collect params
                 this._collectLocals(bodyNode, localScope); // Collect vars in body
                 this._scanBodyForRefs(bodyNode, sourceFunc, localScope, knownGlobals, references);
                 // Don't recurse into this node's children via the main loop, we handled the body.
                 // But wait, if there are nested functions, we might miss them if we stop?
                 // Actually, _scanBodyForRefs recurses. But we should be careful about nested definitions.
                 // For simplicity, let's just let the main loop continue, but we need to ensure we don't duplicate work?
                 // No, standard pattern in other adapters is to NOT recurse into children from here if we handled it?
                 // In CsharpAdapter, it says "Do not descend into nested methods if any".
                 return; 
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
            'if_statement', 'for_statement', 'for_in_statement', 'while_statement', 
            'do_statement', 'switch_case', 'catch_clause', 'ternary_expression'
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
        // Parameters
        const params = node.childForFieldName('parameters');
        if (params) {
            params.children.forEach(p => {
                const name = p.type === 'identifier' ? p.text : null; // Simple param
                if (name) localScope.add(name);
                // TODO: Handle destructuring, default values, typed params
                // required_parameter -> pattern -> identifier
                if (p.type === 'required_parameter' || p.type === 'optional_parameter') {
                    const pattern = p.childForFieldName('pattern');
                    if (pattern && pattern.type === 'identifier') localScope.add(pattern.text);
                }
            });
        }

        // Variable Declarations
        if (node.type === 'variable_declarator') {
            const nameNode = node.childForFieldName('name');
            if (nameNode && nameNode.type === 'identifier') localScope.add(nameNode.text);
        }

        // Recurse broadly for declarations
        // We only want top-level locals for this scope, but let's just grab everything declared in the block
        // This is a simplification.
        if (node.type === 'lexical_declaration' || node.type === 'variable_declaration') {
             for (let i = 0; i < node.childCount; i++) this._collectLocals(node.child(i), localScope);
        }
    }

    _scanBodyForRefs(node, sourceFunc, localScope, knownGlobals, refs) {
        // 1. Function Calls
        if (node.type === 'call_expression' || node.type === 'new_expression') {
            const funcNode = node.childForFieldName('function') || node.childForFieldName('constructor'); 
            if (funcNode) {
                let calleeName = funcNode.text;
                // Handle method calls: obj.method()
                if (funcNode.type === 'member_expression') {
                     const propNode = funcNode.childForFieldName('property');
                     if (propNode) calleeName = propNode.text;
                }
                refs.push({ source: sourceFunc, target: calleeName, type: 'Call' });
            }
        }
        
        // 2. Variable Usage
        if (node.type === 'identifier') {
            const name = node.text;
            if (!localScope.has(name)) {
                let isCallPart = false;
                
                // Check if part of a call/member access
                if (node.parent) {
                    // foo() -> foo is function
                    if (node.parent.type === 'call_expression' && node.parent.childForFieldName('function')?.id === node.id) isCallPart = true;
                    // new Foo() -> Foo is constructor
                    if (node.parent.type === 'new_expression' && node.parent.childForFieldName('constructor')?.id === node.id) isCallPart = true;
                    
                    // obj.prop -> prop is property name (often not a global ref unless we track fields)
                    // obj.method() -> method is Call, not Usage
                    if (node.parent.type === 'member_expression') {
                         if (node.parent.childForFieldName('property')?.id === node.id) isCallPart = true; 
                    }
                }
                
                if (!isCallPart) {
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

module.exports = TsAdapter;
