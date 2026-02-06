const neo4jService = require('./Neo4jService');

const commitHash = process.argv[2];

if (!commitHash) {
    console.error("Please provide a commit hash as an argument.");
    process.exit(1);
}

async function setCommit() {
    console.log(`Updating graph state to commit: ${commitHash}`);
    try {
        await neo4jService.updateGraphState(commitHash);
        console.log("Graph state updated successfully.");
        const state = await neo4jService.getGraphState();
        console.log("Current State:", state);
    } catch (e) {
        console.error("Failed to update graph state:", e);
    } finally {
        await neo4jService.close();
    }
}

setCommit();
