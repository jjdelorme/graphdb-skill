const { test, describe, mock, before, beforeEach } = require('node:test');
const assert = require('node:assert');
const path = require('path');
const dotenv = require('dotenv');

// Load environment variables for testing
dotenv.config({ path: path.resolve(__dirname, '../../../.env') });

const VectorService = require('../scripts/services/VectorService');

describe('VectorService', () => {
    let mockClient;
    let mockPredict;

    before(() => {
        process.env.GOOGLE_CLOUD_PROJECT = 'test-project';
        process.env.GOOGLE_CLOUD_LOCATION = 'us-central1';
        process.env.GEMINI_EMBEDDING_MODEL = 'gemini-embedding-001';
    });

    beforeEach(() => {
        mockPredict = mock.fn(async () => {
            // Mocking the structure of PredictionServiceClient response
            return [{
                predictions: [{
                    embeddings: {
                        values: [0.1, 0.2, 0.3]
                    }
                }]
            }];
        });
        mockClient = {
            predict: mockPredict
        };
    });

    test('Test 1: Configuration - initializes with correct params', async () => {
        const service = new VectorService({ client: mockClient });
        assert.ok(service);
        assert.strictEqual(service.modelName, 'gemini-embedding-001');
        assert.strictEqual(service.project, 'test-project');
    });

    test('Test 2: Embedding Generation - returns vector for single string', async () => {
        const service = new VectorService({ client: mockClient });
        const input = "Test string";
        const expectedVector = [0.1, 0.2, 0.3];

        const result = await service.embedDocuments([input]);
        assert.deepStrictEqual(result[0], expectedVector);
        assert.strictEqual(mockPredict.mock.callCount(), 1);
    });

    test('Test 3: Rate Limit Handling (Retries) - retries on failure', async () => {
        let callCount = 0;
        mockPredict = mock.fn(async () => {
            callCount++;
            if (callCount <= 2) {
                const error = new Error("Quota exceeded");
                error.code = 8; // gRPC Resource Exhausted (429)
                throw error;
            }
            return [{
                predictions: [{
                    embeddings: {
                        values: [0.9, 0.9, 0.9]
                    }
                }]
            }];
        });
        
        mockClient = { predict: mockPredict };
        const service = new VectorService({ client: mockClient });
        service.sleep = async () => {}; 

        const result = await service.embedDocuments(["Retry Me"]);
        
        assert.strictEqual(callCount, 3);
        assert.deepStrictEqual(result[0], [0.9, 0.9, 0.9]);
    });
});