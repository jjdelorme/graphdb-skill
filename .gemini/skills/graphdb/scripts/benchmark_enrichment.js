const VectorService = require('./services/VectorService');
const fs = require('fs');
const path = require('path');

// Mock dependencies
const BATCH_SIZE = 50;

// Mock Data
const mockFunctions = Array.from({ length: BATCH_SIZE }, (_, i) => ({
    id: `func-${i}`,
    name: `function_${i}`,
    file: `src/utils_${i % 5}.js`, // 5 files, 10 functions per file
    start: (i % 10) * 10 + 1,
    end: (i % 10) * 10 + 5
}));

// Mock FS
const originalReadFileSync = fs.readFileSync;
const mockFileContent = Array.from({ length: 200 }, (_, i) => `Line ${i + 1}`).join('\n');
const fsStats = { reads: 0 };
fs.readFileSync = (path, encoding) => {
    fsStats.reads++;
    // Simulate slight I/O delay? Sync doesn't delay event loop, but takes CPU time.
    // We'll just count reads.
    return mockFileContent;
};

// Mock VectorService
class MockVectorService extends VectorService {
    // Override the single item embedder to simulate latency
    async _embedSingle(text) {
        // Simulate network latency: 50ms per item
        // With concurrency of 5, 50 items should take ~500ms (10 batches * 50ms) instead of 2500ms
        await new Promise(resolve => setTimeout(resolve, 50)); 
        return Array(768).fill(0.1);
    }
}

async function runBenchmark() {
    console.log("Starting Benchmark (Optimized Simulation)...");
    const start = Date.now();
    const vectorService = new MockVectorService();
    
    // Logic from enrich_vectors.js (New Optimization)
    const fileGroups = {};
    for (const item of mockFunctions) {
        if (!fileGroups[item.file]) fileGroups[item.file] = [];
        fileGroups[item.file].push(item);
    }

    const textsToEmbed = [];
    
    for (const relPath in fileGroups) {
        const groupItems = fileGroups[relPath];
        const absPath = path.resolve(__dirname, '../../../../', relPath); // Dummy path
        
        // Mock I/O: Read file ONCE per group
        if (fs.existsSync(absPath) || true) { 
             const content = fs.readFileSync(absPath, 'utf8');
             const lines = content.split('\n');
             
             for (const item of groupItems) {
                 const sourceCode = lines.slice(item.start - 1, item.end).join('\n');
                 textsToEmbed.push(`Function Name: ${item.name}\nSource Code:\n${sourceCode}`);
             }
        }
    }

    await vectorService.embedDocuments(textsToEmbed);
    
    const duration = Date.now() - start;
    console.log(`Processed ${BATCH_SIZE} items in ${duration}ms`);
    console.log(`File Reads: ${fsStats.reads}`);
    
    // Restore
    fs.readFileSync = originalReadFileSync;
}

runBenchmark();