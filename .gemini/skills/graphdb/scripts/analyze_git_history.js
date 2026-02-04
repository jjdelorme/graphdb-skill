const { execSync } = require('child_process');
const neo4jService = require('./Neo4jService');
const path = require('path');
const fs = require('fs');

async function main() {
    console.log('Starting git history analysis...');

    // The repository path is the project root (4 levels up from .gemini/skills/graphdb/scripts/)
    const repoPath = path.resolve(__dirname, '../../../../');
    
    let fileChangeCount = {};
    let filteredCoChanges = [];

    // 1. Get git log
    // Format: COMMIT:hash|author|date
    // Followed by list of files
    console.log('Extracting git log from all branches...');
    // Increase maxBuffer for large repositories
    const logOutput = execSync(`git -C "${repoPath}" log --all --name-only --pretty=format:"COMMIT:%H|%an|%ad"`, { maxBuffer: 200 * 1024 * 1024 }).toString();

    const lines = logOutput.split('\n');
    const commitGroups = []; // array of sets of files
    let currentCommitFiles = new Set();

    // Extensions supported by the skill (from extract_graph.js)
    const RELEVANT_EXTENSIONS = new Set([
        '.c', '.cc', '.cpp', '.cxx', '.h', '.hh', '.hpp', '.hxx', '.inl',
        '.cs', '.vb', '.asp', '.aspx', '.cshtml', '.razor', '.sql'
    ]);

    console.log('Parsing log output...');
    for (const line of lines) {
        if (line.startsWith('COMMIT:')) {
            // Only consider commits that touch a reasonable number of files to avoid noise from massive refactors
            if (currentCommitFiles.size > 0 && currentCommitFiles.size <= 100) {
                commitGroups.push(currentCommitFiles);
            }
            currentCommitFiles = new Set();
        } else if (line.trim() !== '') {
            const file = line.trim().replace(/\\/g, '/');
            const ext = path.extname(file).toLowerCase();
            if (RELEVANT_EXTENSIONS.has(ext)) {
                // Store path relative to repo root
                const filePath = file;
                fileChangeCount[filePath] = (fileChangeCount[filePath] || 0) + 1;
                currentCommitFiles.add(filePath);
            }
        }
    }
    if (currentCommitFiles.size > 0 && currentCommitFiles.size <= 100) {
        commitGroups.push(currentCommitFiles);
    }

    console.log(`Analyzed ${commitGroups.length} relevant commits (size <= 100).`);
    console.log(`Found ${Object.keys(fileChangeCount).length} unique relevant files.`);

    // 2. Co-change analysis
    console.log('Analyzing co-changes...');
    const coChanges = new Map();
    for (let k = 0; k < commitGroups.length; k++) {
        if (k % 1000 === 0) process.stdout.write(`\r  Processing commit ${k} / ${commitGroups.length}...`);
        const files = commitGroups[k];
        const fileList = Array.from(files).sort();
        for (let i = 0; i < fileList.length; i++) {
            for (let j = i + 1; j < fileList.length; j++) {
                const key = `${fileList[i]}|${fileList[j]}`;
                coChanges.set(key, (coChanges.get(key) || 0) + 1);
            }
        }
    }
    console.log('\nCo-change analysis complete.');

    console.log('Filtering co-changes with threshold >= 5...');
    for (const [key, count] of coChanges.entries()) {
        if (count >= 5) {
            const [file1, file2] = key.split('|');
            filteredCoChanges.push({ file1, file2, count });
        }
    }

    console.log(`Found ${filteredCoChanges.length} co-change pairs with threshold >= 5.`);

    // 3. Update Neo4j
    const session = neo4jService.getSession();

    try {
        console.log('Updating File nodes with change_frequency...');
        const fileEntries = Object.entries(fileChangeCount);
        const batchSize = 1000;
        for (let i = 0; i < fileEntries.length; i += batchSize) {
            const batch = fileEntries.slice(i, i + batchSize).map(([file, count]) => ({ file, count }));
            await session.run(`
                UNWIND $batch AS item
                MATCH (f:File {file: item.file})
                SET f.change_frequency = item.count
            `, { batch });
            process.stdout.write(`\r  Processed ${Math.min(i + batchSize, fileEntries.length)} / ${fileEntries.length} files`);
        }
        console.log('\nFile change frequencies updated.');

        console.log('Calculating risk_scores for Functions...');
        // Risk score = complexity * change_frequency (from its file)
        await session.run(`
            MATCH (f:Function)-[:DEFINED_IN]->(file:File)
            WHERE file.change_frequency IS NOT NULL
            WITH f, file
            SET f.risk_score = toInteger(f.complexity) * toInteger(file.change_frequency)
        `);

        console.log('Creating CO_CHANGED_WITH relationships...');
        for (let i = 0; i < filteredCoChanges.length; i += batchSize) {
            const batch = filteredCoChanges.slice(i, i + batchSize);
            await session.run(`
                UNWIND $batch AS item
                MATCH (f1:File {file: item.file1})
                MATCH (f2:File {file: item.file2})
                MERGE (f1)-[r:CO_CHANGED_WITH]-(f2)
                SET r.count = item.count
            `, { batch });
            process.stdout.write(`\r  Processed ${Math.min(i + batchSize, filteredCoChanges.length)} / ${filteredCoChanges.length} co-change pairs`);
        }
        console.log('\nGit history analysis complete.');

    } catch (error) {
        console.error('Error updating Neo4j:', error);
    } finally {
        await session.close();
        await neo4jService.close();
    }
}

main().catch(console.error);