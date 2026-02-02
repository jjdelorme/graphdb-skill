const { test, describe, mock } = require('node:test');
const assert = require('node:assert');
const path = require('node:path');
const fs = require('node:fs');

describe('extract_graph_v2 (Discovery Logic)', () => {
    test('should identify files from compile_commands.json and VIEW glob', async (t) => {
        // Mock fs.existsSync
        const existsMock = mock.method(fs, 'existsSync', (p) => {
            if (p.endsWith('compile_commands.json')) return true;
            return false;
        });

        // Mock glob for zero-config discovery
        const glob = require('glob');
        const originalSync = glob.sync;
        glob.sync = mock.fn((pattern, options) => {
            // VERIFY: Pattern should NOT be restricted to VIEW/
            assert.ok(!pattern.startsWith('VIEW/'), `Pattern should not be restricted to VIEW/, but was: ${pattern}`);
            
            if (pattern.includes('cs')) {
                return [
                    path.join(options.cwd, 'src/App.cs'),
                    path.join(options.cwd, 'VIEW/Legacy.cs'),
                    path.join(options.cwd, 'lib/Helper.cs')
                ];
            }
            return [];
        });

        // Mock GraphBuilder via require.cache
        const gbPath = require.resolve('../core/GraphBuilder');
        const builderRunMock = mock.fn(async (fileList) => {
            // Verify zero-config discovery
            assert.ok(fileList.some(f => f.endsWith('App.cs')), 'Should include App.cs from src/');
            assert.ok(fileList.some(f => f.endsWith('Legacy.cs')), 'Should include Legacy.cs from VIEW/');
            assert.ok(fileList.some(f => f.endsWith('Helper.cs')), 'Should include Helper.cs from lib/');
            
            return Promise.resolve();
        });

        class MockGraphBuilder {
            constructor() {}
            run(fileList) { return builderRunMock(fileList); }
        }

        require.cache[gbPath] = {
            id: gbPath,
            filename: gbPath,
            loaded: true,
            exports: MockGraphBuilder
        };

        // Mock compile commands path in fs.readFileSync
        const originalReadFileSync = fs.readFileSync;
        const readMock = mock.method(fs, 'readFileSync', function() {
            const p = arguments[0];
            if (typeof p === 'string' && p.endsWith('compile_commands.json')) {
                return JSON.stringify([
                    { file: 'src/main.cpp', directory: '/root' },
                    { file: 'VIEW/oda/excluded.cpp', directory: '/root/ODA_CAD' }
                ]);
            }
            // Fallback for other files
            return originalReadFileSync.apply(fs, arguments);
        });

        // Now require the script. 
        const { main } = require('../extract_graph.js');
        
        await main();

        assert.strictEqual(builderRunMock.mock.callCount(), 1, 'GraphBuilder.run should be called once');
        
        // Cleanup
        delete require.cache[gbPath];
        existsMock.mock.restore();
        readMock.mock.restore();
        glob.sync = originalSync;
    });
});