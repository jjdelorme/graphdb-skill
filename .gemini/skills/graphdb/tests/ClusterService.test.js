const { describe, it } = require('node:test');
const assert = require('node:assert');
const ClusterService = require('../scripts/services/ClusterService');

describe('ClusterService', () => {
    it('should cluster obvious groups', () => {
        // Two groups: near [0,0] and near [10,10]
        const vectors = [
            [0.1, 0.1], [0.2, 0.1], [0.1, 0.2],
            [10.1, 10.1], [10.2, 10.1], [10.1, 10.2]
        ];

        const result = ClusterService.cluster(vectors, 2);
        
        assert.strictEqual(result.k, 2);
        assert.ok(result.score > 0.8, 'Should have a high silhouette score');
        
        // Check that members of group 1 have same cluster ID
        assert.strictEqual(result.clusters[0], result.clusters[1]);
        assert.strictEqual(result.clusters[1], result.clusters[2]);
        
        // Check that members of group 2 have same cluster ID
        assert.strictEqual(result.clusters[3], result.clusters[4]);
        assert.strictEqual(result.clusters[4], result.clusters[5]);
        
        // Check groups are different
        assert.notStrictEqual(result.clusters[0], result.clusters[3]);
    });

    it('should auto-tune K', () => {
        // Three clear groups
        const vectors = [
            [1, 1], [1.1, 1.1],
            [10, 10], [10.1, 10.1],
            [100, 100], [100.1, 100.1]
        ];

        const result = ClusterService.cluster(vectors); // No K
        
        assert.strictEqual(result.k, 3, 'Should auto-detect 3 clusters');
        assert.ok(result.score > 0.9);
    });

    it('should handle single vector gracefully', () => {
        const result = ClusterService.cluster([[1,1]]);
        assert.strictEqual(result.k, 1);
        assert.strictEqual(result.clusters[0], 0);
    });
});
