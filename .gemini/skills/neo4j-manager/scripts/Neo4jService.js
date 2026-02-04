const neo4j = require('neo4j-driver');
const path = require('path');
const fs = require('fs');

class Neo4jService {
    constructor() {
        // Try to load .env from project root
        this._loadEnv();

        // Defaults
        this.uri = process.env.NEO4J_URI || 'bolt://localhost:7687';
        this.user = process.env.NEO4J_USER || 'neo4j';
        this.password = process.env.NEO4J_PASSWORD;

        if (!this.password) {
            console.warn('Warning: NEO4J_PASSWORD not found in environment.');
        }

        this.driver = neo4j.driver(this.uri, neo4j.auth.basic(this.user, this.password));
    }

    _loadEnv() {
        let currentPath = process.cwd();
        for (let i = 0; i < 5; i++) {
            const envPath = path.join(currentPath, '.env');
            if (fs.existsSync(envPath)) {
                require('dotenv').config({ path: envPath });
                return;
            }
            const parent = path.dirname(currentPath);
            if (parent === currentPath) break;
            currentPath = parent;
        }
    }

    getSession(database = null) {
        const config = database ? { database } : {};
        return this.driver.session(config);
    }

    async close() {
        await this.driver.close();
    }
}

module.exports = new Neo4jService();
