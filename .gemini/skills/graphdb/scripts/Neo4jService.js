const neo4j = require('neo4j-driver');
const path = require('path');
const fs = require('fs');

class Neo4jService {
    constructor() {
        // Try to load .env from project root
        // We look for .env in current dir, then parent, up to 5 levels
        this._loadEnv();

        this.uri = process.env.NEO4J_URI || 
                   `bolt://${process.env.NEO4J_HOST || 'localhost'}:${process.env.NEO4J_PORT || '7687'}`;
        this.user = process.env.NEO4J_USER;
        this.password = process.env.NEO4J_PASSWORD;

        if (!this.user || !this.password) {
            console.warn('Warning: NEO4J_USER or NEO4J_PASSWORD not found in environment.');
        }

        this.driver = neo4j.driver(this.uri, neo4j.auth.basic(this.user, this.password));
    }

    _loadEnv() {
        // Try explicit path first (4 levels up from scripts/)
        const explicitPath = path.resolve(__dirname, '../../../../.env');
        if (fs.existsSync(explicitPath)) {
            require('dotenv').config({ path: explicitPath });
            return;
        }

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

    /**
     * Safely converts a value to a number, handling Neo4j Integer types.
     */
    toNum(val) {
        if (val === null || val === undefined) return null;
        if (typeof val.toNumber === 'function') return val.toNumber();
        return Number(val);
    }

    getSession() {
        return this.driver.session();
    }

    async close() {
        await this.driver.close();
    }

    async run(query, params = {}, session = null) {
        const targetSession = session || this.getSession();
        try {
            const result = await targetSession.run(query, params);
            return result;
        } finally {
            if (!session) await targetSession.close();
        }
    }
}

module.exports = new Neo4jService();
