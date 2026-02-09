const { Command } = require('commander');
const neo4jService = require('./Neo4jService');
const { syncGraph } = require('./sync_graph');
const ClusterService = require('./services/ClusterService');

const queries = {
    'suggest-seams': async (session, params) => {
        const module = params.module || '.*';
        const targetK = params.k ? parseInt(params.k, 10) : undefined;

        // 1. Fetch functions and their embeddings
        const result = await session.run(`
            MATCH (f:Function)-[:DEFINED_IN]->(file:File)
            WHERE file.file =~ $pattern AND f.embedding IS NOT NULL
            RETURN f.label as name, f.embedding as embedding, file.file as file
        `, { pattern: `.*${module}.*` });

        if (result.records.length === 0) {
            return { error: 'No functions with embeddings found for pattern: ' + module };
        }

        const data = result.records.map(r => ({
            name: r.get('name'),
            embedding: r.get('embedding'),
            file: r.get('file')
        }));

        // 2. Perform Clustering
        const vectors = data.map(d => d.embedding);
        const clusterResult = ClusterService.cluster(vectors, targetK);

        // 3. Group results
        const clusters = {};
        for (let i = 0; i < data.length; i++) {
            const clusterId = clusterResult.clusters[i];
            if (!clusters[clusterId]) clusters[clusterId] = [];
            clusters[clusterId].push({
                name: data[i].name,
                file: data[i].file
            });
        }

        return {
            pattern: module,
            k: clusterResult.k,
            silhouette_score: clusterResult.score,
            clusters: Object.entries(clusters).map(([id, members]) => ({
                id: parseInt(id, 10),
                member_count: members.length,
                members: members.slice(0, 10), // Limit members in output for readability
                representative_members: members.slice(0, 3).map(m => m.name)
            }))
        };
    },
    'debug-files': async (session, params) => {
        const result = await session.run(`
            MATCH (f:Function)-[:DEFINED_IN]->(file:File)
            RETURN file.file as file, count(f) as total, sum(CASE WHEN f.embedding IS NOT NULL THEN 1 ELSE 0 END) as embedded
            ORDER BY embedded DESC
            LIMIT 20
        `);
        return result.records.map(r => r.toObject());
    },
    'ui-contamination': async (session, params) => {
        const module = params.module || '.*';
        const result = await session.run(`
            MATCH (f:Function)-[:DEFINED_IN]->(file:File)
            WHERE file.file =~ $pattern
            RETURN 
                count(f) as total,
                sum(CASE WHEN f.ui_contaminated THEN 1 ELSE 0 END) as contaminated,
                sum(CASE WHEN f.pure_business_logic THEN 1 ELSE 0 END) as pure
        `, { pattern: `.*${module}.*` });
        return result.records[0].toObject();
    },
    'globals': async (session, params) => {
        const module = params.module || '.*';
        const result = await session.run(`
            MATCH (f:Function)-[:DEFINED_IN]->(file:File)
            WHERE file.file =~ $pattern
            MATCH (f)-[r:USES_GLOBAL]->(g:Global)
            RETURN f.label as function, type(r) as access, g.label as global, file.file as file
            LIMIT 100
        `, { pattern: `.*${module}.*` });
        return result.records.map(r => r.toObject());
    },
    'seams': async (session, params) => {
        const module = params.module || '.*';
        const result = await session.run(`
            MATCH (caller:Function {ui_contaminated: true})-[:CALLS]->(f:Function {ui_contaminated: false})-[:DEFINED_IN]->(file:File)
            WHERE file.file =~ $pattern
            RETURN DISTINCT f.label as seam, file.file as file, f.risk_score as risk
            ORDER BY f.risk_score DESC
            LIMIT 20
        `, { pattern: `.*${module}.*` });
        return result.records.map(r => r.toObject());
    },
    'hotspots': async (session, params) => {
        const module = params.module || '.*';
        const result = await session.run(`
            MATCH (f:Function)-[:DEFINED_IN]->(file:File)
            WHERE file.file =~ $pattern AND f.risk_score IS NOT NULL
            RETURN f.label as name, f.risk_score as risk, file.file as file
            ORDER BY f.risk_score DESC
            LIMIT 20
        `, { pattern: `.*${module}.*` });
        return result.records.map(r => r.toObject());
    },
    'co-change': async (session, params) => {
        const file = params.file;
        if (!file) return { error: 'Missing file parameter' };
        const result = await session.run(`
            MATCH (f1:File {file: $file})-[r:CO_CHANGED_WITH]-(f2:File)
            RETURN f2.file as co_changed_file, r.count as count
            ORDER BY r.count DESC
        `, { file });
        return result.records.map(r => r.toObject());
    },
    'impact': async (session, params) => {
        const func = params.function;
        if (!func) return { error: 'Missing function parameter' };
        const result = await session.run(`
            MATCH (caller:Function)-[:CALLS*1..3]->(f:Function {label: $func})
            RETURN DISTINCT caller.label as caller, caller.ui_contaminated as contaminated
        `, { func });
        return result.records.map(r => r.toObject());
    },
    'test-context': async (session, params) => {
        const func = params.function;
        if (!func) return { error: 'Missing function parameter' };

        // Check existence first
        const check = await session.run(`MATCH (f:Function {label: $func}) RETURN f`, { func });
        if (check.records.length === 0) {
            console.error(`Function '${func}' not found in graph.`);
            return [];
        }

        const result = await session.run(`
            MATCH (f:Function {label: $func})-[:CALLS|USES_GLOBAL]->(dep)
            RETURN dep.label as dependency, dep.type as type, labels(dep) as labels
        `, { func });
        return result.records.map(r => r.toObject());
    },
    'hybrid-context': async (session, params) => {
        const func = params.function;
        if (!func) return { error: 'Missing function parameter' };

        // 1. Structural Dependencies
        const structuralResult = await session.run(`
            MATCH (f:Function {label: $func})
            OPTIONAL MATCH (f)-[:CALLS]->(callee:Function)
            OPTIONAL MATCH (caller:Function)-[:CALLS]->(f)
            RETURN 
                collect(DISTINCT callee.label) as callees,
                collect(DISTINCT caller.label) as callers
        `, { func });
        
        const structural = structuralResult.records.length > 0 
            ? structuralResult.records[0].toObject() 
            : { callees: [], callers: [] };

        // 2. Semantic Neighbors
        // We first find the target embedding, then query for similar nodes
        const semanticResult = await session.run(`
            MATCH (target:Function {label: $func})
            WHERE target.embedding IS NOT NULL
            CALL db.index.vector.queryNodes('function_embeddings', 5, target.embedding)
            YIELD node, score
            WHERE node.label <> $func
            RETURN node.label as name, score, node.file as file
        `, { func });
        
        const semantic = semanticResult.records.map(r => ({
            name: r.get('name'),
            score: r.get('score'),
            file: r.get('file')
        }));

        return {
            function: func,
            structural_dependencies: structural,
            semantic_related: semantic
        };
    },
    'extract-service': async (session, params) => {
        const module = params.module || '.*';
        const result = await session.run(`
            MATCH (f:Function {ui_contaminated: false})-[:DEFINED_IN]->(file:File)
            WHERE file.file =~ $pattern
            OPTIONAL MATCH (caller)-[:CALLS]->(f)
            WITH f, file, count(caller) as call_count
            RETURN f.label as name, file.file as file, f.risk_score as risk, call_count
            ORDER BY call_count DESC, f.risk_score DESC
            LIMIT 20
        `, { pattern: `.*${module}.*` });
        return result.records.map(r => r.toObject());
    },
    'progress': async (session, params) => {
        const result = await session.run(`
            MATCH (f:Function)
            RETURN 
                count(f) as total,
                sum(CASE WHEN f.ui_contaminated THEN 1 ELSE 0 END) as contaminated,
                sum(CASE WHEN f.pure_business_logic THEN 1 ELSE 0 END) as pure,
                sum(CASE WHEN f.risk_score > 1000 THEN 1 ELSE 0 END) as high_risk
        `);
        return result.records[0].toObject();
    },
    'function': async (session, params) => {
        const func = params.function;
        if (!func) return { error: 'Missing function parameter' };
        const result = await session.run(`
            MATCH (f:Function {label: $func})
            OPTIONAL MATCH (f)-[:DEFINED_IN]->(file:File)
            RETURN f as function, file.file as file
        `, { func });
        return result.records.map(r => {
            const props = r.get('function').properties;
            const { embedding, ...cleanProps } = props;
            return {
                ...cleanProps,
                file: r.get('file')
            };
        });
    },
    'analyze-state': async (session, params) => {
        const func = params.function;
        if (!func) return { error: 'Missing function parameter' };
        
        // Return usages of globals
        const result = await session.run(`
            MATCH (f:Function {label: $func})
            OPTIONAL MATCH (f)-[:USES_GLOBAL]->(g:Global)
            RETURN g.label as name, g.type as type, g.file as defined_in
        `, { func });
        
        const state = {
            Globals: [],
            FileStatics: [], // Placeholders for future refinement
            Constants: []
        };
        
        result.records.forEach(r => {
            const name = r.get('name');
            if (name) {
                state.Globals.push({ name, file: r.get('defined_in') });
            }
        });
        
        return state;
    }
};

async function main() {
    const program = new Command();

    program
        .name('query_graph')
        .description('CLI to query the Neo4j graph for code analysis')
        .version('1.0.0');

    // Global options
    program
        .option('-m, --module <pattern>', 'Module regex pattern', '.*')
        .option('-f, --function <name>', 'Function name')
        .option('-F, --file <path>', 'File path')
        .option('-k, --k <number>', 'Cluster count');

    // Argument for query type
    program
        .argument('<query_type>', 'Type of query to run')
        .action(async (queryType, options) => {
            // 0. Auto-Sync Check
            await syncGraph();

            if (!queries[queryType]) {
                console.error(`Unknown query type: ${queryType}`);
                console.error('Available queries:', Object.keys(queries).join(', '));
                process.exit(1);
            }

            const session = neo4jService.getSession();
            try {
                // Merge options into params
                const params = { ...options };
                const result = await queries[queryType](session, params);
                
                console.log(JSON.stringify(result, (key, value) => {
                    if (typeof value === 'bigint') return value.toString();
                    if (value && typeof value.toNumber === 'function') return value.toNumber();
                    return value;
                }, 2));
            } catch (error) {
                console.error('Query Error:', error);
                process.exit(1);
            } finally {
                await session.close();
                await neo4jService.close();
            }
        });

    program.parse(process.argv);
}

main().catch(console.error);