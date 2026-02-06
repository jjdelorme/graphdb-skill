const { kmeans } = require('ml-kmeans');

class ClusterService {
    /**
     * Clusters vectors and finds the best K if not provided.
     * @param {number[][]} vectors 
     * @param {number} [k] - Optional K. If omitted, tries 2-5 and picks best silhouette score.
     * @returns {Object} { clusters, centroids, k, score }
     */
    static cluster(vectors, k) {
        if (!vectors || vectors.length < 2) {
            return { clusters: (vectors || []).map(() => 0), centroids: [], k: 1, score: 0 };
        }

        if (k) {
            const result = kmeans(vectors, k, { initialization: 'kmeans++' });
            const score = this.calculateSilhouetteScore(vectors, result.clusters);
            return { ...result, k, score };
        }

        // Auto-tune K
        let bestResult = null;
        let maxK = Math.min(vectors.length - 1, 5);
        
        for (let currentK = 2; currentK <= maxK; currentK++) {
            const result = kmeans(vectors, currentK, { initialization: 'kmeans++' });
            const score = this.calculateSilhouetteScore(vectors, result.clusters);
            
            if (!bestResult || score > bestResult.score) {
                bestResult = { ...result, k: currentK, score };
            }
        }

        return bestResult;
    }

    /**
     * Simple Silhouette Score (-1 to 1)
     * 1: Well clustered
     * 0: Overlapping
     * -1: Mis-clustered
     */
    static calculateSilhouetteScore(vectors, clusters) {
        const n = vectors.length;
        if (n <= 1) return 0;

        const k = Math.max(...clusters) + 1;
        if (k <= 1) return 0;

        let totalS = 0;

        for (let i = 0; i < n; i++) {
            const clusterI = clusters[i];
            
            // Calculate a(i): avg distance to points in same cluster
            let a_i = 0;
            let countSame = 0;
            
            // Calculate b(i): min avg distance to points in other clusters
            const b_vals = new Array(k).fill(0);
            const b_counts = new Array(k).fill(0);

            for (let j = 0; j < n; j++) {
                if (i === j) continue;
                
                const dist = this.euclideanDistance(vectors[i], vectors[j]);
                
                if (clusters[j] === clusterI) {
                    a_i += dist;
                    countSame++;
                } else {
                    b_vals[clusters[j]] += dist;
                    b_counts[clusters[j]]++;
                }
            }

            a_i = countSame > 0 ? a_i / countSame : 0;

            let b_i = Infinity;
            for (let c = 0; c < k; c++) {
                if (c === clusterI || b_counts[c] === 0) continue;
                const avgDistToC = b_vals[c] / b_counts[c];
                if (avgDistToC < b_i) b_i = avgDistToC;
            }

            if (countSame === 0) {
                // Singleton cluster: convention is to set silhouette to 0
                totalS += 0;
            } else if (b_i === Infinity) {
                totalS += 0;
            } else {
                totalS += (b_i - a_i) / Math.max(a_i, b_i);
            }
        }

        return totalS / n;
    }

    static euclideanDistance(v1, v2) {
        let sum = 0;
        for (let i = 0; i < v1.length; i++) {
            sum += Math.pow(v1[i] - v2[i], 2);
        }
        return Math.sqrt(sum);
    }
}

module.exports = ClusterService;
