const GraphBuilder = require('../core/GraphBuilder');
const fs = require('fs');
const path = require('path');
const assert = require('assert');

// Mock Stream
class MockStream {
    constructor() { this.data = []; }
    write(chunk) { this.data.push(JSON.parse(chunk)); }
    end() {}
}

const TEST_FILE = 'dummy.mock';

async function testImplicitGlobal() {
    console.log("Testing Implicit Global Logic...");
    
    // Create dummy file
    fs.writeFileSync(TEST_FILE, "dummy content");

    // Mock Config
    const mockAdapter = {
        init: async () => {},
        parse: () => ({ delete: () => {} }),
        scanDefinitions: () => [],
        scanReferences: () => [
            { source: 'MyFunc', target: 'g_Val', type: 'ImplicitGlobalWrite' }
        ]
    };

    const builder = new GraphBuilder({
        root: '.',
        outputDir: './test_output',
        adapters: { mock: mockAdapter }
    });
    
    // Hijack streams
    builder.nodesStream = new MockStream();
    builder.edgesStream = new MockStream();
    
    // Mock getAdapter
    builder._getAdapterForFile = () => mockAdapter;
    
    // Run
    try {
        await builder.run([TEST_FILE]);
        
        // Verify Edges
        const edges = builder.edgesStream.data;
        const writeEdge = edges.find(e => e.type === 'WRITES_TO_GLOBAL');
        
        assert.ok(writeEdge, "Should find WRITES_TO_GLOBAL edge");
        
        // Verify Nodes (Inferred Global)
        const nodes = builder.nodesStream.data;
        // logic: getNode('Global', 'g_Val', null, { inferred: true })
        // The ID is MD5('Global:g_Val')
        const globalNode = nodes.find(n => n.label === 'g_Val' && n.type === 'Global');
        assert.ok(globalNode, "Should emit Global node");
        assert.strictEqual(globalNode.inferred, true, "Global node should be inferred");
        
        console.log("PASS: Implicit Global Test");
    } finally {
        if (fs.existsSync(TEST_FILE)) fs.unlinkSync(TEST_FILE);
        if (fs.existsSync('./test_output')) {
             if (fs.existsSync('./test_output/nodes.jsonl')) fs.unlinkSync('./test_output/nodes.jsonl');
             if (fs.existsSync('./test_output/edges.jsonl')) fs.unlinkSync('./test_output/edges.jsonl');
             fs.rmdirSync('./test_output');
        }
    }
}

testImplicitGlobal().catch(e => {
    console.error(e);
    process.exit(1);
});
