const VectorService = require('./.gemini/skills/graphdb/scripts/services/VectorService');

async function testWithConfig() {
    const vectorService = new VectorService();
    console.log(`Model: ${vectorService.modelName}`);
    console.log(`Dimensions requested (via config): 768`);

    try {
        const result = await vectorService.client.models.embedContent({
            model: vectorService.modelName,
            contents: [{ parts: [{ text: "test string" }] }],
            config: {
                outputDimensionality: 768
            }
        });
        
        let vector = null;
        if (result && result.embedding && result.embedding.values) {
            vector = result.embedding.values;
        } else if (result && result.embeddings && result.embeddings[0] && result.embeddings[0].values) {
             vector = result.embeddings[0].values;
        }

        if (vector) {
            console.log(`Generated vector length: ${vector.length}`);
        } else {
            console.log("No vector generated");
        }
    } catch (e) {
        console.error("Error:", e);
    }
}

testWithConfig();