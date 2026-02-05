const { test, describe, mock } = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');

describe('Enrichment Logic', () => {
    
    test('Test 1: Source Extraction - slices correct lines', () => {
        const mockFileContent = `Line 1
Line 2
Line 3
Line 4
Line 5`;
        
        const originalReadFileSync = fs.readFileSync;
        fs.readFileSync = mock.fn((path, encoding) => mockFileContent);

        // Simulate the logic we will write in enrich_vectors.js
        const extractSource = (filePath, start, end) => {
            const content = fs.readFileSync(filePath, 'utf8');
            const lines = content.split('\n');
            return lines.slice(start - 1, end).join('\n');
        };

        try {
            const result = extractSource('dummy.cs', 2, 4);
            assert.strictEqual(result, "Line 2\nLine 3\nLine 4");
        } finally {
            fs.readFileSync = originalReadFileSync;
        }
    });

    test('Test 2: Cypher Generation - structure for batch update', () => {
        const batch = [
            { id: 'n1', embedding: [0.1, 0.2] },
            { id: 'n2', embedding: [0.3, 0.4] }
        ];

        // The query we intend to use
        const query = `
            UNWIND $batch as row 
            MATCH (f:Function {id: row.id}) 
            SET f.embedding = row.embedding
        `;

        // Verify it contains key clauses
        assert.ok(query.includes("UNWIND $batch"));
        assert.ok(query.includes("SET f.embedding"));
    });

    test('Test 3: Batch Optimization - groups functions by file and reads once', () => {
        const batch = [
            { id: 1, file: 'a.js', start: 1, end: 1 },
            { id: 2, file: 'a.js', start: 3, end: 3 },
            { id: 3, file: 'b.js', start: 1, end: 1 }
        ];

        let readCounts = {};
        const mockFs = {
            existsSync: () => true,
            readFileSync: (f) => {
                readCounts[f] = (readCounts[f] || 0) + 1;
                return "L1\nL2\nL3\nL4";
            }
        };

        // Implementation we want to test
        const processBatch = (items) => {
            const results = [];
            // Group by file
            const groups = {};
            for (const item of items) {
                if (!groups[item.file]) groups[item.file] = [];
                groups[item.file].push(item);
            }

            for (const file in groups) {
                const content = mockFs.readFileSync(file);
                const lines = content.split('\n');
                for (const item of groups[file]) {
                    results.push({
                        id: item.id,
                        code: lines.slice(item.start - 1, item.end).join('\n')
                    });
                }
            }
            return results;
        };

        const processed = processBatch(batch);

        assert.strictEqual(processed.length, 3);
        assert.strictEqual(readCounts['a.js'], 1, 'Should read a.js only once');
        assert.strictEqual(readCounts['b.js'], 1, 'Should read b.js only once');
        assert.strictEqual(processed.find(p => p.id === 1).code, 'L1');
        assert.strictEqual(processed.find(p => p.id === 2).code, 'L3');
    });
});