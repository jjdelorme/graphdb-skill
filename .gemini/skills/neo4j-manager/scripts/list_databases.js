const neo4jService = require('./Neo4jService');

async function listDatabases() {
    const systemSession = neo4jService.getSession('system');

    try {
        const result = await systemSession.run('SHOW DATABASES YIELD name, currentStatus, address, role, default');
        
        const dbs = result.records.map(r => ({
            name: r.get('name'),
            status: r.get('currentStatus'),
            default: r.get('default')
        }));

        console.log('\n--- Neo4j Databases ---');
        dbs.forEach(db => {
            const activeMarker = db.status === 'online' ? '🟢' : '🔴';
            const defaultMarker = db.default ? ' (Default)' : '';
            console.log(`${activeMarker} ${db.name.padEnd(15)} [${db.status}]${defaultMarker}`);
        });
        console.log('-----------------------\n');

    } catch (e) {
        console.error('Error listing databases:', e.message);
    } finally {
        await systemSession.close();
        await neo4jService.close();
    }
}

listDatabases();
