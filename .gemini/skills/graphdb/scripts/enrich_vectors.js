async function main() {
    console.log("Enrichment is now handled automatically by the Go binary during the 'ingest' phase.");
    console.log("To regenerate embeddings, run 'node .gemini/skills/graphdb/extraction/extract_graph.js'.");
    process.exit(0);
}

if (require.main === module) {
    main().catch(console.error);
}

module.exports = { main };
