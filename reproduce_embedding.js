const VectorService = require('./.gemini/skills/graphdb/scripts/services/VectorService');

async function run() {
    const vectorService = new VectorService();
    console.log(`Model: ${vectorService.modelName}`);
    console.log(`Dimensions requested: ${vectorService.dimensions}`);

    try {
        const vectors = await vectorService.embedDocuments(["test string"]);
        if (vectors && vectors[0]) {
            console.log(`Generated vector length: ${vectors[0].length}`);
        } else {
            console.log("No vector generated");
        }
    } catch (e) {
        console.error("Error:", e);
    }
}

run();