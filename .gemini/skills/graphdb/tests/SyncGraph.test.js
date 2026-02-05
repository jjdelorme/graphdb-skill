const { test, describe, mock, beforeEach, afterEach } = require('node:test');
const assert = require('node:assert');
const path = require('node:path');
const child_process = require('child_process');

describe('sync_graph (Incremental Updates)', () => {
    let neo4jMock;
    let execSyncMock;
    let updateFileMock;
    let syncGraph;
    const repoRoot = path.resolve(__dirname, '../../../../');

    beforeEach(() => {
        // Mock Neo4jService
        neo4jMock = {
            getGraphState: mock.fn(),
            updateGraphState: mock.fn(),
            close: mock.fn(),
            getSession: mock.fn(() => ({ close: mock.fn() }))
        };

        const neo4jPath = require.resolve('../scripts/Neo4jService.js');
        require.cache[neo4jPath] = {
            id: neo4jPath,
            filename: neo4jPath,
            loaded: true,
            exports: neo4jMock
        };

        // Mock update_file
        updateFileMock = mock.fn();
        const updateFilePath = require.resolve('../scripts/update_file.js');
        require.cache[updateFilePath] = {
            id: updateFilePath,
            filename: updateFilePath,
            loaded: true,
            exports: { updateFile: updateFileMock }
        };

        // Mock execSync
        execSyncMock = mock.method(child_process, 'execSync');

        // Load module
        const syncGraphModule = require('../scripts/sync_graph.js');
        syncGraph = syncGraphModule.syncGraph;
    });

    afterEach(() => {
        mock.restoreAll();
        // Clear cache to allow fresh mocks
        delete require.cache[require.resolve('../scripts/sync_graph.js')];
    });

    test('should do nothing if graph is up to date', async (t) => {
        neo4jMock.getGraphState.mock.mockImplementation(() => Promise.resolve({ commit: 'hash1' }));
        execSyncMock.mock.mockImplementation((cmd) => {
            if (cmd.includes('rev-parse HEAD')) return 'hash1';
            return '';
        });

        const result = await syncGraph();

        assert.strictEqual(result.synced, true);
        assert.strictEqual(result.updatedFiles, 0);
        assert.strictEqual(neo4jMock.updateGraphState.mock.callCount(), 0); // No update needed if matched
    });

    test('should update state only if no source files changed', async (t) => {
        neo4jMock.getGraphState.mock.mockImplementation(() => Promise.resolve({ commit: 'hash1' }));
        execSyncMock.mock.mockImplementation((cmd) => {
            if (cmd.includes('rev-parse HEAD')) return 'hash2';
            if (cmd.includes('git diff')) return 'README.md\npackage.json'; // Non-source files
            return '';
        });

        const result = await syncGraph();

        assert.strictEqual(result.synced, true);
        assert.strictEqual(updateFileMock.mock.callCount(), 0);
        assert.strictEqual(neo4jMock.updateGraphState.mock.callCount(), 1);
        assert.deepStrictEqual(neo4jMock.updateGraphState.mock.calls[0].arguments, ['hash2']);
    });

    test('should perform surgical update for small changes', async (t) => {
        neo4jMock.getGraphState.mock.mockImplementation(() => Promise.resolve({ commit: 'hash1' }));
        execSyncMock.mock.mockImplementation((cmd) => {
            if (cmd.includes('rev-parse HEAD')) return 'hash2';
            if (cmd.includes('git diff')) return 'src/Logic.cs\nsrc/Utils.cpp';
            return '';
        });

        const result = await syncGraph();

        assert.strictEqual(result.synced, true);
        assert.strictEqual(result.updatedFiles, 2);
        assert.strictEqual(updateFileMock.mock.callCount(), 2);
        
        // Verify paths passed to updateFile
        const args0 = updateFileMock.mock.calls[0].arguments[0];
        const args1 = updateFileMock.mock.calls[1].arguments[0];
        assert.ok(args0.endsWith('Logic.cs'));
        assert.ok(args1.endsWith('Utils.cpp'));

        assert.strictEqual(neo4jMock.updateGraphState.mock.callCount(), 1);
    });

    test('should skip update for macro changes (> 5 files) without force', async (t) => {
        neo4jMock.getGraphState.mock.mockImplementation(() => Promise.resolve({ commit: 'hash1' }));
        execSyncMock.mock.mockImplementation((cmd) => {
            if (cmd.includes('rev-parse HEAD')) return 'hash2';
            if (cmd.includes('git diff')) return Array(10).fill('file.cs').join('\n');
            return '';
        });

        const result = await syncGraph(false); // force = false

        assert.strictEqual(result.synced, false);
        assert.strictEqual(updateFileMock.mock.callCount(), 0);
        assert.strictEqual(neo4jMock.updateGraphState.mock.callCount(), 0);
    });

    test('should proceed with update for macro changes IF force is true', async (t) => {
        neo4jMock.getGraphState.mock.mockImplementation(() => Promise.resolve({ commit: 'hash1' }));
        execSyncMock.mock.mockImplementation((cmd) => {
            if (cmd.includes('rev-parse HEAD')) return 'hash2';
            if (cmd.includes('git diff')) return Array(6).fill('file.cs').join('\n');
            return '';
        });

        const result = await syncGraph(true); // force = true

        assert.strictEqual(result.synced, true);
        assert.strictEqual(result.updatedFiles, 6);
        assert.strictEqual(updateFileMock.mock.callCount(), 6);
        assert.strictEqual(neo4jMock.updateGraphState.mock.callCount(), 1);
    });
});