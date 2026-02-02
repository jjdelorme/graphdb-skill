class AspAdapter {
    constructor(subAdapters) {
        this.csharpAdapter = subAdapters.csharp;
        this.vbAdapter = subAdapters.vb;
    }

    async init() {
        // Sub-adapters are initialized by the caller
    }

    async run(filePaths) {
        // This adapter usually is called per file by the builder, 
        // but looking at extract_graph_v2, the builder calls 'parse' on the adapter.
        // Wait, GraphBuilder.js logic:
        // builder.run(files) -> iterates files -> get adapter -> adapter.parse(content) -> adapter.scanDefinitions(tree)...
        // My AspAdapter needs to conform to this interface.
        // BUT, AspAdapter delegates. It doesn't return a single tree.
        // It returns a "Composite Result" or it merges results.
        
        // I need to check GraphBuilder.js to see how strictly it enforces "tree".
        // If GraphBuilder calls `scanDefinitions(tree)`, then `parse` MUST return a tree.
        // But here I have multiple trees (or one tree from a virtual file).
        
        // Strategy:
        // parse(content) -> returns { virtualTree: ..., languageAdapter: ... }
        // scanDefinitions(result) -> calls result.languageAdapter.scanDefinitions(result.virtualTree)
        
        // This requires GraphBuilder to be flexible or AspAdapter to abstract this.
        // If I can't change GraphBuilder, I must return a Proxy object that looks like a tree?
        // Or better: AspAdapter "is" the adapter.
        
        // Let's assume I can modify GraphBuilder if needed, but let's try to fit in.
        // If I use the "Whitespace Masking" strategy, I produce ONE string of C# (or VB) code.
        // Then I parse that string with the sub-adapter.
        // I return that tree.
        // Then GraphBuilder calls `scanDefinitions(tree)`.
        // The sub-adapter (CS or VB) logic works on that tree.
        // This works PERFECTLY if I delegate to the correct adapter's methods.
        
        // ISSUE: I need to know WHICH adapter to use for `scanDefinitions` later.
        // If I return a standard Tree, GraphBuilder doesn't know it came from AspAdapter logic that selected C# vs VB.
        // But `extract_graph_v2` selects adapter by file extension.
        // For .aspx, it selects AspAdapter.
        // AspAdapter.scanDefinitions(tree) will be called.
        // So AspAdapter needs to know which language was used for that tree.
        
        // So `parse` returns an object: { tree: tree, lang: 'csharp' } ?
        // GraphBuilder expects `tree`.
        // I will attach the lang to the tree object? `tree.language = 'csharp'`.
        // Then `scanDefinitions(tree)` checks `tree.language`.
        return Promise.resolve();
    }

    parse(sourceCode, filePath) {
        // 1. Detect Language
        let isVb = false;
        
        // Strategy A: Check File Extension (if provided)
        if (filePath && filePath.toLowerCase().endsWith('.asp')) {
            isVb = true; // Legacy ASP defaults to VBScript
        }

        // Strategy B: Check Page Directive (overrides extension if present)
        // e.g. <%@ Page Language="C#" %> or <%@ Language="VBScript" %>
        const langMatch = sourceCode.match(/<%@\s*(?:Page\s+)?.*?Language="?(\w+)"?.*?>/i);
        if (langMatch && langMatch[1]) {
            const lang = langMatch[1].toLowerCase();
            if (lang.includes('vb') || lang === 'vbscript') isVb = true;
            else if (lang.includes('c#') || lang === 'csharp') isVb = false;
        }
        
        // 2. Extract Code (Whitespace Masking)
        const maskedCode = this._maskHtml(sourceCode);
        
        // 3. Delegate Parse
        const adapter = isVb ? this.vbAdapter : this.csharpAdapter;
        
        let codeToParse = maskedCode;
        let lineOffset = 0;
        
        if (isVb) {
            // VB.NET parser requires a container (Class/Module) for methods.
            // We wrap the code in a dummy Module to ensure top-level scripts are parsed.
            codeToParse = "Module AspWrapper\n" + maskedCode + "\nEnd Module";
            lineOffset = -1;
        }

        const tree = adapter.parse(codeToParse);
        
        // 4. Tag tree with adapter for subsequent steps
        tree.userData = { adapter: adapter, lineOffset: lineOffset }; // 'userData' is safe to add to JS object
        
        return tree;
    }

    scanDefinitions(tree) {
        if (tree.userData && tree.userData.adapter) {
            const definitions = tree.userData.adapter.scanDefinitions(tree);
            
            // Adjust line numbers if offset is set
            if (tree.userData.lineOffset) {
                definitions.forEach(d => {
                    if (d.line) d.line += tree.userData.lineOffset;
                    if (d.end_line) d.end_line += tree.userData.lineOffset;
                });
            }
            
            return definitions;
        }
        return [];
    }

    scanReferences(tree, knownGlobals) {
        if (tree.userData && tree.userData.adapter) {
            return tree.userData.adapter.scanReferences(tree, knownGlobals);
        }
        return [];
    }

    _maskHtml(source) {
        // Replace all non-code characters with spaces, preserving newlines.
        let output = source.split(''); // Char array
        
        // We want to KEEP code in <% ... %> and <script runat="server"> ... </script>
        // And REPLACE everything else with spaces (preserving \n).
        
        // It is easier to identify Code Regions, and blank out everything else.
        const codeRegions = [];
        
        // Regex for <% ... %> (excluding directives <%@ ... %>)
        // Note: [^] matches any char including newline
        const blockRegex = /<%(?!@)([\s\S]*?)%>/g;
        let match;
        while ((match = blockRegex.exec(source)) !== null) {
            // match[2] is the content
            // We want to preserve the content positions.
            // The tags <% and %> should probably be spaced out or treated as whitespace?
            // If we behave like they are spaces, the code inside stays in place.
            // match.index is start of <%.
            // Content starts at match.index + match[0].indexOf(match[2])
            // Actually, simply:
            const start = match.index;
            const end = start + match[0].length;
            const contentStart = start + (match[0].startsWith('<%=') ? 3 : 2); 
            const contentEnd = end - 2;
            
            codeRegions.push({ start: contentStart, end: contentEnd });
        }

        // Regex for <script runat="server">
        // <script\s+[^>]*runat="server"[^>]*>([\s\S]*?)<\/script>
        const scriptRegex = /<script\s+[^>]*runat=["']server["'][^>]*>([\s\S]*?)<\/script>/gi;
        while ((match = scriptRegex.exec(source)) !== null) {
             // Find content group index
             // match[1] is content
             // We need exact indices of capture group 1
             // Regex exec doesn't give group indices directly in standard JS, only full match index.
             // We can calculate it.
             const fullMatch = match[0];
             const content = match[1];
             const preContent = fullMatch.split(content)[0]; // risky if content repeats?
             // Better:
             const contentStartOffset = fullMatch.indexOf(content);
             const start = match.index + contentStartOffset;
             const end = start + content.length;
             
             codeRegions.push({ start, end });
        }
        
        // Now we have regions to KEEP.
        // We iterate the char array. If index is NOT in a region, replace with space (unless newline).
        
        // Optimization: Create a mask array
        const keepMask = new Uint8Array(source.length); // 0 = replace, 1 = keep
        codeRegions.forEach(r => {
            for(let i=r.start; i<r.end; i++) keepMask[i] = 1;
        });

        for (let i = 0; i < source.length; i++) {
            if (keepMask[i] === 0) {
                if (output[i] !== '\n' && output[i] !== '\r') {
                    output[i] = ' ';
                }
            }
        }

        return output.join('');
    }
}

module.exports = AspAdapter;
