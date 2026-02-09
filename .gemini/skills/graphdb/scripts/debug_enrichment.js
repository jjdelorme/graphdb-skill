const neo4jService = require('./Neo4jService');

async function main() {
    const session = neo4jService.getSession();
    try {
        console.log("Checking for missing embeddings...");
        
        // Count total nodes
        const totalRes = await session.run('MATCH (n:Function) RETURN count(n) as c');
        const total = neo4jService.toNum(totalRes.records[0].get('c'));
        
        // Count embedded
        const embeddedRes = await session.run('MATCH (n:Function) WHERE n.embedding IS NOT NULL RETURN count(n) as c');
        const embedded = neo4jService.toNum(embeddedRes.records[0].get('c'));
        
        console.log(`Total Functions: ${total}`);
        console.log(`Embedded: ${embedded} (${((embedded/total)*100).toFixed(1)}%)`);
        
        if (embedded < total) {
            console.log("
Sample of missing nodes:");
            const missingRes = await session.run('MATCH (n:Function) WHERE n.embedding IS NULL RETURN n.label, n.file LIMIT 20');
            missingRes.records.forEach(r => {
                console.log(` - ${r.get('n.label')} (${r.get('n.file')})`);
            });
        }
        
    } catch (e) {
        console.error(e);
    } finally {
        await session.close();
        await neo4jService.close();
    }
}

main();
