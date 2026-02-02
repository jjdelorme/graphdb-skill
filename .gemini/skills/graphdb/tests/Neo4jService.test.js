const { test, describe, mock, before, after } = require('node:test');
const assert = require('node:assert');
const path = require('node:path');
const fs = require('node:fs');

describe('Neo4jService', () => {
    test('should load credentials from environment variables', (t) => {
        // Mock process.env
        const originalEnv = process.env;
        process.env = { ...originalEnv };
        process.env.NEO4J_URI = 'bolt://test-host:7687';
        process.env.NEO4J_USER = 'test-user';
        process.env.NEO4J_PASSWORD = 'test-password';

        // Clear cache to re-initialize
        delete require.cache[require.resolve('../scripts/Neo4jService.js')];
        const neo4jService = require('../scripts/Neo4jService.js');

        assert.strictEqual(neo4jService.uri, 'bolt://test-host:7687');
        assert.strictEqual(neo4jService.user, 'test-user');
        assert.strictEqual(neo4jService.password, 'test-password');

        // Restore
        process.env = originalEnv;
    });

    test('should fail gracefully if credentials are missing (warning only)', (t) => {
        // Mock fs.existsSync to prevent re-loading .env during test
        const existsMock = mock.method(fs, 'existsSync', () => false);

        const originalEnv = process.env;
        process.env = { ...originalEnv };
        delete process.env.NEO4J_USER;
        delete process.env.NEO4J_PASSWORD;

        delete require.cache[require.resolve('../scripts/Neo4jService.js')];
        const neo4jService = require('../scripts/Neo4jService.js');

        assert.strictEqual(neo4jService.user, undefined);
        
        process.env = originalEnv;
        existsMock.mock.restore();
    });
});
