const neo4jService = require('./Neo4jService');

async function main() {
    const session = neo4jService.getSession();
    try {
        console.log("=== Graph Status Report ===
");
        
        // Node Counts
        const nodeRes = await session.run('MATCH (n) RETURN labels(n) as labels, count(n) as c ORDER BY c DESC');
        console.log("Nodes by Label:");
        nodeRes.records.forEach(r => {
            const labels = r.get('labels').filter(l => l !== 'Entity' && l !== 'Node').join(', ');
            const count = neo4jService.toNum(r.get('c'));
            console.log(`  ${labels || 'Unlabeled'}: ${count}`);
        });
        
        console.log("");
        
        // Edge Counts
        const edgeRes = await session.run('MATCH ()-[r]->() RETURN type(r) as type, count(r) as c ORDER BY c DESC');
        console.log("Edges by Type:");
        edgeRes.records.forEach(r => {
            console.log(`  ${r.get('type')}: ${neo4jService.toNum(r.get('c'))}`);
        });
        
        console.log("");
        
        // Embeddings
        const embedRes = await session.run('MATCH (n) WHERE n.embedding IS NOT NULL RETURN count(n) as c');
        console.log(`Nodes with Vector Embeddings: ${neo4jService.toNum(embedRes.records[0].get('c'))}`);
        
    } catch (e) {
        console.error(e);
    } finally {
        await session.close();
        await neo4jService.close();
    }
}

main();
