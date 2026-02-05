const neo4jService = require('./.gemini/skills/graphdb/scripts/Neo4jService');

async function checkEmbeddings() {
    const session = neo4jService.getSession();
    try {
        console.log("Checking if Function nodes have embeddings...");
        const result = await session.run("MATCH (f:Function) WHERE f.embedding IS NOT NULL RETURN count(f) as count");
        console.log("Functions with embeddings: " + result.records[0].get('count'));

        const resultNull = await session.run("MATCH (f:Function) WHERE f.embedding IS NULL RETURN count(f) as count");
        console.log("Functions WITHOUT embeddings: " + resultNull.records[0].get('count'));

    } catch (e) {
        console.error("Check failed:", e);
    } finally {
        await session.close();
        await neo4jService.close();
    }
}

checkEmbeddings();