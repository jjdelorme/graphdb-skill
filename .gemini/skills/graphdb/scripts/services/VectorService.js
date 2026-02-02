const { GoogleGenAI } = require("@google/genai");

class VectorService {
    constructor(config = {}) {
        if (!process.env.GOOGLE_CLOUD_PROJECT || !process.env.GOOGLE_CLOUD_LOCATION) {
            console.warn("Missing Google Cloud configuration (GOOGLE_CLOUD_PROJECT or GOOGLE_CLOUD_LOCATION).");
        }
        
        const project = process.env.GOOGLE_CLOUD_PROJECT;
        const location = process.env.GOOGLE_CLOUD_LOCATION;
        this.modelName = process.env.GEMINI_EMBEDDING_MODEL || "models/gemini-embedding-001";

        if (config.client) {
            this.client = config.client;
        } else {
            this.client = new GoogleGenAI({ vertexAI: { project, location } });
        }
    }

    async embedDocuments(texts) {
        if (!texts || texts.length === 0) return [];

        const embeddings = [];
        
        for (const text of texts) {
            let attempt = 0;
            let vector = null;
            const maxRetries = 3;

            while (attempt < maxRetries) {
                try {
                    const result = await this.client.models.embedContent({
                        model: this.modelName,
                        content: { parts: [{ text: text }] }
                    });
                    
                    if (result && result.embedding && result.embedding.values) {
                        vector = result.embedding.values;
                    } else if (result && result.embeddings && result.embeddings[0] && result.embeddings[0].values) {
                         vector = result.embeddings[0].values;
                    }

                    if (vector) {
                        break;
                    } else {
                        console.warn("Unexpected embedding response structure", JSON.stringify(result));
                        break;
                    }
                } catch (e) {
                    attempt++;
                    const isRateLimit = e.status === 429 || e.code === 429 || e.message?.includes("Quota exceeded");
                    
                    if (isRateLimit && attempt < maxRetries) {
                        const delay = Math.pow(2, attempt) * 1000;
                        await (this.sleep ? this.sleep(delay) : new Promise(resolve => setTimeout(resolve, delay)));
                        continue;
                    }
                    
                    console.error("Error generating embedding:", e.message);
                    break;
                }
            }
            embeddings.push(vector);
        }
        return embeddings;
    }
}

module.exports = VectorService;
