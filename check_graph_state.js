const neo4jService = require('./.gemini/skills/graphdb/scripts/Neo4jService');

async function checkGraph() {
    const session = neo4jService.getSession();
    try {
        console.log("Checking for vector indexes...");
        const indexes = await session.run("SHOW VECTOR INDEXES");
        indexes.records.forEach(r => {
            console.log("Index: " + r.get('name') + ", State: " + r.get('state'));
        });

        console.log("\nCounting nodes with labels...");
        const counts = await session.run("MATCH (n) RETURN labels(n) as labels, count(*) as count");
        counts.records.forEach(r => {
            console.log(r.get('labels') + ": " + r.get('count'));
        });

        console.log("\nChecking for plating-related nodes...");
        const plating = await session.run("MATCH (n) WHERE n.label CONTAINS 'Plating' OR n.file CONTAINS 'Plating' RETURN n.label, n.file LIMIT 5");
        plating.records.forEach(r => {
            console.log("Node: " + r.get('n.label') + " in " + r.get('n.file'));
        });

    } catch (e) {
        console.error("Check failed:", e);
    } finally {
        await session.close();
        await neo4jService.close();
    }
}

checkGraph();