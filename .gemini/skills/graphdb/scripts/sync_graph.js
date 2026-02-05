const { execSync } = require('child_process');
const path = require('path');
const neo4jService = require('./Neo4jService');
const { updateFile } = require('./update_file');

// Paths
const ROOT_DIR = path.resolve(__dirname, '../../../../');

// Extensions to watch (must match extract_graph.js)
const EXTENSIONS = new Set([
    '.c', '.cc', '.cpp', '.cxx', '.h', '.hh', '.hpp', '.hxx', '.inl',
    '.cs', '.vb', '.asp', '.aspx', '.cshtml', '.razor', '.sql'
]);

async function syncGraph(force = false) {
    console.error("Checking graph synchronization...");
    let result = { synced: false, updatedFiles: 0 };

    try {
        // 1. Get State
        const state = await neo4jService.getGraphState();
        let currentCommit;
        try {
            currentCommit = execSync('git rev-parse HEAD', { cwd: ROOT_DIR }).toString().trim();
        } catch (e) {
            console.error("Failed to get current git commit. Is this a git repo?");
            return result;
        }

        if (!state) {
            console.error("No graph state found. Please run full ingestion first.");
            return result;
        }

        const lastCommit = state.commit;
        if (lastCommit === currentCommit) {
            console.error("Graph is up to date.");
            result.synced = true;
            return result;
        }

        const lastCommitDisplay = lastCommit ? lastCommit.substring(0, 7) : 'Unknown';
        console.error(`Graph is behind. Indexed: ${lastCommitDisplay}, HEAD: ${currentCommit.substring(0, 7)}`);

        // 2. Diff
        console.error("Calculating diff...");
        let diffOutput;
        try {
            if (!lastCommit) {
                 console.warn("Previous commit unknown. Cannot calculate diff. Proceeding with surgical update on all files (or skipping if too risky).");
                 // In this edge case, we can't really diff. 
                 // If we assume everything changed, we might trigger a full rebuild.
                 // For now, let's treat it as "changedFiles = []" which triggers a state update, 
                 // effectively "claiming" the current state is the new baseline.
                 // This effectively "Resets" the tracking.
                 diffOutput = "";
            } else {
                 diffOutput = execSync(`git diff --name-only ${lastCommit} ${currentCommit}`, { cwd: ROOT_DIR }).toString();
            }
        } catch (e) {
            console.error("Failed to diff. The last indexed commit might be unreachable.");
            return result;
        }

        const changedFiles = diffOutput.split('\n')
            .map(line => line.trim())
            .filter(line => line.length > 0)
            .filter(file => {
                 const ext = path.extname(file).toLowerCase();
                 return EXTENSIONS.has(ext);
            });

        console.error(`Found ${changedFiles.length} changed source files.`);

        // 3. Strategy
        if (changedFiles.length === 0) {
            console.error("No source code changes detected. Updating state only.");
            await neo4jService.updateGraphState(currentCommit);
            result.synced = true;
            return result;
        }

        if (changedFiles.length > 5 && !force) {
            console.warn(`Macro-change detected (${changedFiles.length} files).`);
            console.warn("Skipping auto-sync. Please run full ingestion or use 'node scripts/sync_graph.js --force'.");
            return result;
        }

        // 4. Surgical Update
        console.error("Starting surgical update...");
        for (const file of changedFiles) {
             const absPath = path.join(ROOT_DIR, file);
             try {
                 await updateFile(absPath);
             } catch (e) {
                 console.error(`Failed to update ${file}:`, e);
             }
        }

        // 5. Update State
        await neo4jService.updateGraphState(currentCommit);
        console.error("Graph synced successfully.");
        result.synced = true;
        result.updatedFiles = changedFiles.length;
        return result;

    } catch (e) {
        console.error("Sync failed:", e);
        return result;
    }
}

if (require.main === module) {
    const force = process.argv.includes('--force');
    syncGraph(force).then(() => neo4jService.close());
}

module.exports = { syncGraph };