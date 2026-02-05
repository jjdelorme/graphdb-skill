const VectorService = require('./.gemini/skills/graphdb/scripts/services/VectorService');

async function run() {
    const vectorService = new VectorService();
    // monkey patch client to debug
    const originalEmbed = vectorService.client.models.embedContent.bind(vectorService.client.models);
    
    vectorService.client.models.embedContent = async (args) => {
        console.log("Calling embedContent with:", JSON.stringify(args, null, 2));
        const res = await originalEmbed(args);
        // console.log("Result:", JSON.stringify(res, null, 2)); 
        return res;
    }

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