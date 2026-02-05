const VectorService = require('./.gemini/skills/graphdb/scripts/services/VectorService');

async function run() {
    process.env.GEMINI_EMBEDDING_MODEL = "text-embedding-004";
    const vectorService = new VectorService();
    console.log(`Using model: ${vectorService.modelName}`);

    try {
        const vectors = await vectorService.embedDocuments(["test string"]);
        if (vectors && vectors[0]) {
            console.log(`Generated vector length: ${vectors[0].length}`);
        }
    } catch (e) {
        console.error("Error:", e);
    }
}

run();