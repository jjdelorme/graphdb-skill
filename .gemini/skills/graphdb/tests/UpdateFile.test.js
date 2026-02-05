const { test, describe, mock, beforeEach, afterEach } = require('node:test');
const assert = require('node:assert');
const path = require('node:path');
const fs = require('fs');

describe('update_file (Surgical Update)', () => {
    let neo4jMock;
    let graphBuilderMock;
    let txMock;
    let updateFile;
    const repoRoot = path.resolve(__dirname, '../../../../');

    beforeEach(() => {
        // Mock Neo4j
        txMock = {
            run: mock.fn(() => Promise.resolve())
        };
        neo4jMock = {
            getSession: mock.fn(() => ({
                writeTransaction: mock.fn(async (cb) => await cb(txMock)),
                close: mock.fn()
            })),
            close: mock.fn()
        };
        
        const neo4jPath = require.resolve('../scripts/Neo4jService.js');
        require.cache[neo4jPath] = {
            id: neo4jPath,
            filename: neo4jPath,
            loaded: true,
            exports: neo4jMock
        };

        // Mock GraphBuilder
        const gbPath = require.resolve('../extraction/core/GraphBuilder.js');
        class MockGraphBuilder {
            constructor() {}
            async run(files, skipWrite) {
                return {
                    nodes: [
                        { id: 'n0', type: 'File', file: 'src/Test.cs' },
                        { id: 'n1', type: 'Function', label: 'MyFunc', file: 'src/Test.cs' },
                        { id: 'n2', type: 'Function', label: 'ExternalFunc' } // Referenced only
                    ],
                    edges: [
                        { source: 'n1', target: 'n2', type: 'CALLS' }
                    ]
                };
            }
        }
        require.cache[gbPath] = {
            id: gbPath,
            filename: gbPath,
            loaded: true,
            exports: MockGraphBuilder
        };

        // Mock fs.existsSync
        mock.method(fs, 'existsSync', () => true);

        // Load Module
        const updateFileModule = require('../scripts/update_file.js');
        updateFile = updateFileModule.updateFile;
    });

    afterEach(() => {
        mock.restoreAll();
        delete require.cache[require.resolve('../scripts/update_file.js')];
    });

    test('should delete old subgraph and recreate nodes/edges', async (t) => {
        await updateFile('src/Test.cs');

        // 1. Transaction should be called
        assert.strictEqual(neo4jMock.getSession.mock.callCount(), 1);
        
        // 2. Cypher queries
        // - Delete old
        // - Merge File
        // - Create Internal Nodes (MyFunc)
        // - Create Edges (CALLS)
        assert.ok(txMock.run.mock.callCount() >= 4);

        const calls = txMock.run.mock.calls;
        
        // Check Delete
        assert.ok(calls[0].arguments[0].includes('DETACH DELETE f'));
        assert.ok(calls[0].arguments[1].path.endsWith('Test.cs'));

        // Check Merge File
        assert.ok(calls[1].arguments[0].includes('MERGE (f:File'));

        // Check Node Creation (MyFunc)
        const nodeCreationCall = calls.find(c => c.arguments[0].includes('CREATE (n:`Function`'));
        assert.ok(nodeCreationCall, 'Should create internal function node');
        
        // Check Edge Creation (CALLS)
        const edgeCreationCall = calls.find(c => c.arguments[0].includes('MERGE (source)-[:CALLS]->(target)'));
        assert.ok(edgeCreationCall, 'Should create CALLS edge');
    });

    test('should handle deleted files', async (t) => {
        // Mock fs.existsSync to return false
        fs.existsSync.mock.mockImplementation(() => false);
        
        // Mock direct session.run for deletion (since updateFile uses session.run for deleted files, not tx)
        const runMock = mock.fn();
        neo4jMock.getSession.mock.mockImplementation(() => ({
            run: runMock,
            close: mock.fn()
        }));

        await updateFile('src/Deleted.cs');

        assert.strictEqual(runMock.mock.callCount(), 1);
        assert.ok(runMock.mock.calls[0].arguments[0].includes('DETACH DELETE f'));
    });
});
