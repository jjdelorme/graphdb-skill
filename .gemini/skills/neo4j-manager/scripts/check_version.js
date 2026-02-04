const neo4jService = require('./Neo4jService');

async function checkVersion() {
    const session = neo4jService.getSession('system');
    try {
        const result = await session.run('CALL dbms.components() YIELD name, versions, edition');
        const record = result.records[0];
        console.log(`Neo4j Version: ${record.get('name')} ${record.get('versions')[0]} (${record.get('edition')})`);
    } catch (e) {
        console.error('Error checking version:', e.message);
    } finally {
        await session.close();
        await neo4jService.close();
    }
}

checkVersion();
