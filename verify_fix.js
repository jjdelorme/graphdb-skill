const VectorService = require('./.gemini/skills/graphdb/scripts/services/VectorService');

async function verifyFix() {
    const vectorService = new VectorService();
    console.log(`Testing with Model: ${vectorService.modelName}`);
    console.log(`Expected Dimensions: 768`);

    try {
        const vectors = await vectorService.embedDocuments(["verification test"]);
        if (vectors && vectors[0]) {
            console.log(`Actual Vector Length: ${vectors[0].length}`);
            if (vectors[0].length === 768) {
                console.log("SUCCESS: Dimensions match.");
            } else {
                console.error("FAILURE: Dimensions mismatch.");
                process.exit(1);
            }
        } else {
            console.error("FAILURE: No vector generated.");
            process.exit(1);
        }
    } catch (e) {
        console.error("Error during verification:", e);
        process.exit(1);
    }
}

verifyFix();