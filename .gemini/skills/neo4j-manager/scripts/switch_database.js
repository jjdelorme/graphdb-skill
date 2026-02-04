const neo4jService = require('./Neo4jService');

async function switchDatabase() {
    const args = process.argv.slice(2);
    if (args.length !== 1) {
        console.log('Usage: node switch_database.js <database_name>');
        process.exit(1);
    }

    const targetDb = args[0];
    const systemSession = neo4jService.getSession('system');

    try {
        console.log(`Requesting switch to: '${targetDb}'...`);
        
        // 1. List Databases
        const result = await systemSession.run('SHOW DATABASES YIELD name, currentStatus');
        const dbs = result.records.map(r => ({
            name: r.get('name'),
            status: r.get('currentStatus')
        }));

        const currentOnline = dbs.find(d => d.name !== 'system' && d.status === 'online');
        const targetExists = dbs.find(d => d.name === targetDb);

        // 2. Check if already active
        if (currentOnline && currentOnline.name === targetDb) {
            console.log(`✅ Database '${targetDb}' is already active.`);
            return;
        }

        // 3. Stop currently active database (required for Community Edition)
        if (currentOnline) {
            console.log(`⏹️  Stopping active database '${currentOnline.name}'...`);
            await systemSession.run(`STOP DATABASE \`${currentOnline.name}\` WAIT`);
        }

        // 4. Create or Start Target
        if (!targetExists) {
            console.log(`🆕 Database '${targetDb}' does not exist. Creating...`);
            await systemSession.run(`CREATE DATABASE \`${targetDb}\` WAIT`);
            console.log(`✅ Created and started '${targetDb}'.`);
        } else {
            console.log(`▶️  Starting database '${targetDb}'...`);
            await systemSession.run(`START DATABASE \`${targetDb}\` WAIT`);
            console.log(`✅ Started '${targetDb}'.`);
        }

    } catch (e) {
        console.error('❌ Error switching database:', e.message);
        if (e.message.includes('Unsupported administration command')) {
            console.error('   Hint: This script requires Neo4j 5.x Community or Enterprise.');
        }
    } finally {
        await systemSession.close();
        await neo4jService.close();
    }
}

switchDatabase();
