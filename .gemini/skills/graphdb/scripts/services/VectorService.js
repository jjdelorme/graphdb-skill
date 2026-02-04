const { PredictionServiceClient, helpers } = require('@google-cloud/aiplatform');

class VectorService {
    constructor(config = {}) {
        const project = process.env.GOOGLE_CLOUD_PROJECT || "jasondel-cloudrun10";
        let location = process.env.GOOGLE_CLOUD_LOCATION || "us-central1";
        if (location === "global") location = "us-central1";
        
        this.project = project;
        this.location = location;
        this.modelName = process.env.GEMINI_EMBEDDING_MODEL || "gemini-embedding-001";
        
        // Sanitize model name for Vertex AI
        if (this.modelName.startsWith("models/")) {
            this.modelName = this.modelName.replace("models/", "");
        }

        this.client = config.client || new PredictionServiceClient({
            apiEndpoint: `${this.location}-aiplatform.googleapis.com`
        });
    }

    async embedDocuments(texts) {
        if (!texts || texts.length === 0) return [];

        const embeddings = [];
        const endpoint = `projects/${this.project}/locations/${this.location}/publishers/google/models/${this.modelName}`;

        for (const text of texts) {
            let attempt = 0;
            let vector = null;
            const maxRetries = 3;

            while (attempt < maxRetries) {
                try {
                    const instance = helpers.toValue({ content: text });
                    const [response] = await this.client.predict({
                        endpoint,
                        instances: [instance],
                    });

                    // Parse response
                    if (response && response.predictions && response.predictions[0]) {
                        // Some versions of the library return plain objects, others return Protobuf Structs
                        const prediction = response.predictions[0].structValue 
                            ? helpers.fromValue(response.predictions[0])
                            : response.predictions[0];
                            
                        if (prediction.embeddings && prediction.embeddings.values) {
                            vector = prediction.embeddings.values;
                        }
                    }

                    if (vector) break;
                    else {
                        console.warn("Unexpected embedding response structure", JSON.stringify(response));
                        break;
                    }
                } catch (e) {
                    attempt++;
                    const isRateLimit = e.code === 8 || e.message?.includes("Quota exceeded") || e.message?.includes("429");
                    
                    if (isRateLimit && attempt < maxRetries) {
                        const delay = Math.pow(2, attempt) * 1000;
                        await (this.sleep ? this.sleep(delay) : new Promise(resolve => setTimeout(resolve, delay)));
                        continue;
                    }
                    
                    console.error(`Error generating embedding (Model: ${this.modelName}, Project: ${this.project}, Location: ${this.location}):`, e.message);
                    break;
                }
            }
            embeddings.push(vector);
        }
        return embeddings;
    }
}

module.exports = VectorService;