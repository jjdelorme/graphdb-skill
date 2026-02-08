const fs = require('fs');
const path = require('path');

// Configure log file path
const LOG_FILE = path.join(__dirname, '../execution-trace.jsonl');

// Helper to write to log
function logEvent(payload) {
    const entry = {
        timestamp: new Date().toISOString(),
        event: payload.hook_name,
        // Capture context based on event type
        data: payload
    };

    // Append single-line JSON to the log file
    fs.appendFileSync(LOG_FILE, JSON.stringify(entry) + '\n');
}

// 1. Buffer Stdin (The Hook Input)
let inputData = '';
process.stdin.setEncoding('utf8');

process.stdin.on('data', chunk => {
    inputData += chunk;
});

// 2. Process on End
process.stdin.on('end', () => {
    try {
        if (inputData.trim()) {
            const payload = JSON.parse(inputData);
            logEvent(payload);
        }
    } catch (err) {
        // Fallback logging for errors to avoid crashing the hook silently
        fs.appendFileSync(LOG_FILE, JSON.stringify({
            timestamp: new Date().toISOString(),
            event: 'ERROR',
            error: err.message,
            rawInput: inputData
        }) + '\n');
    }

    // 3. Passthrough to Stdout (Critical for Hook Chain)
    // The CLI expects the input to be echoed back (or modified) to continue execution.
    if (inputData.trim()) {
        process.stdout.write(inputData);
    }
});
