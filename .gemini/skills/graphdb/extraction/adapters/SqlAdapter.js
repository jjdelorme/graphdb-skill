class SqlAdapter {
    constructor() {
    }

    async init() {
        return Promise.resolve();
    }

    parse(sourceCode) {
        return { source: sourceCode, type: 'virtual_sql_tree' };
    }

    scanDefinitions(tree) {
        if (tree.type !== 'virtual_sql_tree') return [];
        const source = tree.source;
        const definitions = [];

        // Helper for identifiers: Matches [Name] or Name
        // Group 1: Bracketed content, Group 2: Word
        // We need to compose this carefully.
        // Simplified Pattern for "Name" or "Schema.Name"
        // We'll capture the full name for simplicity or just the leaf name.
        // Let's use specific regexes for each case to be safer.

        // CREATE PROCEDURE [dbo].[Name] or Name
        // We look for "CREATE PROC/PROCEDURE" followed by whitespace
        // Then optional schema: ( [S] . | S . )
        // Then name: ( [N] | N )
        // We will just grab the tokens and clean them.
        
        const procRegex = /create\s+(?:procedure|proc)\s+(?:\[?(\w+)\]?\.\[?(\w+)\]?|\[?(\w+)\]?)/gi;
        const triggerRegex = /create\s+trigger\s+(?:\[?(\w+)\]?\.\[?(\w+)\]?|\[?(\w+)\]?)\s+on\s+(?:\[?(\w+)\]?\.\[?(\w+)\]?|\[?(\w+)\]?)/gi;

        // Note: The previous regexes were a bit brittle with grouping. 
        // Let's try a more robust identifier pattern.
        // Identifier = \[([^\]]+)\]|\w+
        // SchemaQualified = (?:(Id)\.)?(Id) -- messy with groups.
        
        // Let's iterate with a simpler approach: Find the keyword, then parse the next tokens manually? 
        // No, regex is faster for this scale if we get it right.
        
        // Revised Regexes with Correct Escaping
        // Use [\w]+ to allow words.
        
        // Matches: CREATE PROC [dbo].[Name] -> Group 1=dbo, Group 2=Name
        // Matches: CREATE PROC [Name] -> Group 3=Name
        const procPattern = /create\s+(?:procedure|proc)\s+(?:(?:\[([\w]+)\]|(\w+))\.(?:\[([\w]+)\]|(\w+))|(?:\[([\w]+)\]|(\w+)))/gi;
        
        // Matches: CREATE TRIGGER [Name] ON [Table]
        // Trigger Name part: Same as Proc
        // ON part: Same structure
        const triggerPattern = /create\s+trigger\s+(?:(?:\[([\w]+)\]|(\w+))\.(?:\[([\w]+)\]|(\w+))|(?:\[([\w]+)\]|(\w+)))\s+on\s+(?:(?:\[([\w]+)\]|(\w+))\.(?:\[([\w]+)\]|(\w+))|(?:\[([\w]+)\]|(\w+)))/gi;

        const getLine = (index) => {
            return source.substring(0, index).split('\n').length;
        };

        let match;
        // Scan Procedures
        while ((match = procPattern.exec(source)) !== null) {
            // Groups:
            // 1: Schema (bracket)
            // 2: Schema (word)
            // 3: Name (bracket)
            // 4: Name (word)
            // 5: Name (bracket - no schema)
            // 6: Name (word - no schema)
            
            const name = match[3] || match[4] || match[5] || match[6];
            if (name) {
                definitions.push({
                    name: name,
                    type: 'Function',
                    line: getLine(match.index)
                });
            }
        }

        // Scan Triggers
        while ((match = triggerPattern.exec(source)) !== null) {
            // Name Groups: 1-6
            const trgName = match[3] || match[4] || match[5] || match[6];
            
            // Table Groups: 7-12
            // 7: Schema (bracket)
            // 8: Schema (word)
            // 9: Table (bracket)
            // 10: Table (word)
            // 11: Table (bracket - no schema)
            // 12: Table (word - no schema)
            const tblName = match[9] || match[10] || match[11] || match[12];

            if (trgName) {
                definitions.push({
                    name: trgName,
                    type: 'Trigger',
                    line: getLine(match.index),
                    watches: tblName 
                });
            }
        }

        return definitions;
    }

    scanReferences(tree, knownGlobals) {
        if (tree.type !== 'virtual_sql_tree') return [];
        const source = tree.source;
        const references = [];

        // Patterns
        // FROM/JOIN/UPDATE/INSERT INTO [Table]
        const tablePattern = /(?:from|join|update|insert\s+into)\s+(?:(?:\[([\w]+)\]|(\w+))\.(?:\[([\w]+)\]|(\w+))|(?:\[([\w]+)\]|(\w+)))/gi;
        
        // EXEC/EXECUTE [Proc]
        const execPattern = /(?:exec|execute)\s+(?:(?:\[([\w]+)\]|(\w+))\.(?:\[([\w]+)\]|(\w+))|(?:\[([\w]+)\]|(\w+)))/gi;

        const rawRefs = [];
        let match;

        while ((match = tablePattern.exec(source)) !== null) {
            // Groups 1-6 again (Schema...Name...NameNoSchema)
            // Target is the Name part.
            // For UPDATE [Table], it fits.
            // For INSERT INTO [Table], it fits.
            const name = match[3] || match[4] || match[5] || match[6];
            if (name && !this._isKeyword(name)) {
                rawRefs.push({ name: name, type: 'Table', index: match.index });
            }
        }
        
        while ((match = execPattern.exec(source)) !== null) {
            const name = match[3] || match[4] || match[5] || match[6];
             if (name && !this._isKeyword(name)) {
                rawRefs.push({ name: name, type: 'Call', index: match.index });
            }
        }

        // Context definitions (to link refs to source)
        const defs = this.scanDefinitions(tree);
        // We need ranges. scanDefinitions doesn't return ranges, let's re-scan or assume usage.
        // Actually, let's just re-use the regex logic to get ranges.
        
        const procPattern = /create\s+(?:procedure|proc)\s+(?:(?:\[([\w]+)\]|(\w+))\.(?:\[([\w]+)\]|(\w+))|(?:\[([\w]+)\]|(\w+)))/gi;
        const triggerPattern = /create\s+trigger\s+(?:(?:\[([\w]+)\]|(\w+))\.(?:\[([\w]+)\]|(\w+))|(?:\[([\w]+)\]|(\w+)))\s+on\s+(?:(?:\[([\w]+)\]|(\w+))\.(?:\[([\w]+)\]|(\w+))|(?:\[([\w]+)\]|(\w+)))/gi;

        const ranges = [];
        while ((match = procPattern.exec(source)) !== null) {
             const name = match[3] || match[4] || match[5] || match[6];
             ranges.push({ start: match.index, name: name });
        }
        while ((match = triggerPattern.exec(source)) !== null) {
             const name = match[3] || match[4] || match[5] || match[6];
             ranges.push({ start: match.index, name: name });
        }
        ranges.sort((a, b) => a.start - b.start);
        
        for (let i = 0; i < ranges.length; i++) {
            ranges[i].end = (i < ranges.length - 1) ? ranges[i+1].start : source.length;
        }

        rawRefs.forEach(ref => {
            const container = ranges.find(r => ref.index >= r.start && ref.index < r.end);
            if (container) {
                let type = 'Usage';
                if (ref.type === 'Call') type = 'Call';
                // Note: Table references are 'Usage'.
                
                references.push({
                    source: container.name,
                    target: ref.name,
                    type: type
                });
            } else {
                // If not in a container, maybe it's global script?
                // But tests check for UpdateInventory -> Products
            }
        });

        // Add Trigger Watches
        defs.forEach(d => {
            if (d.type === 'Trigger' && d.watches) {
                references.push({
                    source: d.name,
                    target: d.watches,
                    type: 'Watches'
                });
            }
        });

        return references;
    }

    _isKeyword(name) {
        const keywords = new Set(['select', 'from', 'where', 'insert', 'update', 'delete', 'join', 'on', 'as', 'begin', 'end', 'go', 'set', 'declare', 'if', 'else', 'values']);
        return keywords.has(name.toLowerCase());
    }
}

module.exports = SqlAdapter;