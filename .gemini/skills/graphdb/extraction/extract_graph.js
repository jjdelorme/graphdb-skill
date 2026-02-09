const fs = require('fs');
const path = require('path');
const glob = require('glob');
const { execSync } = require('child_process');
const neo4jService = require('../scripts/Neo4jService');
const GraphBuilder = require('./core/GraphBuilder');
const CppAdapter = require('./adapters/CppAdapter');
const CsharpAdapter = require('./adapters/CsharpAdapter');
const VbAdapter = require('./adapters/VbAdapter');
const SqlAdapter = require('./adapters/SqlAdapter');
const AspAdapter = require('./adapters/AspAdapter');
const TsAdapter = require('./adapters/TsAdapter');

// Paths
const ROOT_DIR = path.resolve(__dirname, '../../../../');
const COMPILE_DB_PATH = path.join(ROOT_DIR, 'compile_commands.json');
const OUTPUT_DIR = path.join(ROOT_DIR, '.gemini/graph_data');
const CPP_WASM_PATH = path.join(__dirname, 'node_modules/tree-sitter-cpp/tree-sitter-cpp.wasm');
const CS_WASM_PATH = path.join(__dirname, 'node_modules/tree-sitter-c-sharp/tree-sitter-c_sharp.wasm');

async function main() {
    // 1. Load File List
    let fileList = [];
    const args = process.argv.slice(2);
    const fileListArgIndex = args.indexOf('--file-list');

    if (fileListArgIndex !== -1 && args[fileListArgIndex + 1]) {
        // Surgical Update Mode
        const listPath = args[fileListArgIndex + 1];
        console.log(`Using file list from: ${listPath}`);
        if (fs.existsSync(listPath)) {
            fileList = fs.readFileSync(listPath, 'utf8')
                .split('\n')
                .map(l => l.trim())
                .filter(l => l && !l.startsWith('#'))
                .map(l => path.resolve(ROOT_DIR, l));
        } else {
            console.error(`File list not found: ${listPath}`);
            process.exit(1);
        }
    } else {
        // Full Scan Mode
        console.log("Loading compilation database...");
        const uniqueFiles = new Set();
        
        // 1a. C++ Files from compile_commands.json
        if (fs.existsSync(COMPILE_DB_PATH)) {
            const compileCmds = JSON.parse(fs.readFileSync(COMPILE_DB_PATH, 'utf8'));
            compileCmds.forEach(entry => {
                let filePath = entry.file;
                if (!path.isAbsolute(filePath)) filePath = path.join(entry.directory, entry.file);
                
                uniqueFiles.add(path.resolve(filePath));
            });
        } else {
            console.warn(`Warning: Compilation database not found at ${COMPILE_DB_PATH}`);
        }

        // 1b. Other Files from glob
        console.log("Scanning for source files (CS, VB, ASP, SQL, CPP)...");
        const extensions = 'c,cc,cpp,cxx,h,hh,hpp,hxx,inl,cs,vb,asp,aspx,cshtml,razor,sql,ts,tsx';
        
        const files = glob.sync(`**/*.{${extensions}}`, { 
            cwd: ROOT_DIR, 
            absolute: true, 
            ignore: [
                '**/obj/**', '**/bin/**', '**/node_modules/**', 
                '**/Debug/**', '**/Release/**', '**/.git/**',
                '**/.gemini/**'
            ] 
        });
        console.log(`Found ${files.length} source files via scanning.`);
        files.forEach(f => uniqueFiles.add(f));

        fileList = Array.from(uniqueFiles);
    }
    
    console.log(`Total: ${fileList.length} unique files to process.`);

    // 2. Setup Adapters
    // Note: Cpp/Cs use WASM. Vb uses native. Sql uses Regex. Asp uses Composite.
    const cppAdapter = new CppAdapter(CPP_WASM_PATH);
    const csAdapter = new CsharpAdapter(CS_WASM_PATH);
    const vbAdapter = new VbAdapter(); // Native
    const sqlAdapter = new SqlAdapter(); // Regex
    const tsAdapter = new TsAdapter(); // Native (Tree-sitter bindings)
    const aspAdapter = new AspAdapter({ csharp: csAdapter, vb: vbAdapter });

    // 3. Configure Builder
    const builder = new GraphBuilder({
        root: ROOT_DIR,
        outputDir: OUTPUT_DIR,
        adapters: {
            cpp: cppAdapter,
            csharp: csAdapter,
            vb: vbAdapter,
            sql: sqlAdapter,
            asp: aspAdapter,
            ts: tsAdapter
        }
    });

        // 4. Run
        await builder.run(fileList);

        // 5. Update Graph State
        try {
            console.log("Updating Graph State...");
            const commitHash = execSync('git rev-parse HEAD', { cwd: ROOT_DIR }).toString().trim();
            await neo4jService.updateGraphState(commitHash);
            console.log(`Graph state updated to commit: ${commitHash}`);
        } catch (e) {
            console.error("Failed to update graph state:", e);
        } finally {
            await neo4jService.close();
        }
    }

    

    if (require.main === module) {

        main().catch(console.error);

    }

    

    module.exports = { main };

    