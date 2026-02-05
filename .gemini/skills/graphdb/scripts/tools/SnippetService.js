const fs = require('fs');
const readline = require('readline');

class SnippetService {
  /**
   * Reads specific lines from a file efficiently.
   * @param {string} filePath - Path to the file.
   * @param {number} startLine - 1-based start line.
   * @param {number} endLine - 1-based end line.
   * @returns {Promise<string>} - The content of the lines.
   */
  static async sliceFile(filePath, startLine, endLine) {
    if (!fs.existsSync(filePath)) {
      throw new Error(`File not found: ${filePath}`);
    }

    const fileStream = fs.createReadStream(filePath);
    const rl = readline.createInterface({
      input: fileStream,
      crlfDelay: Infinity
    });

    let currentLine = 0;
    const lines = [];

    for await (const line of rl) {
      currentLine++;
      if (currentLine >= startLine) {
        lines.push(line);
      }
      if (currentLine >= endLine) {
        rl.close();
        fileStream.destroy();
        break;
      }
    }

    return lines.join('\n');
  }

  /**
   * Finds a pattern within a text block and returns context.
   * @param {string} content - The text content to search.
   * @param {string} pattern - The string or regex to search for.
   * @param {number} contextLines - Number of lines of context before and after.
   * @param {number} startOffset - The 1-based line number corresponding to the first line of content.
   * @returns {Array<{lines: Array<{number: number, content: string}>}>}
   */
  static findPatternInScope(content, pattern, contextLines = 0, startOffset = 1) {
    const lines = content.split('\n');
    const matches = [];

    for (let i = 0; i < lines.length; i++) {
      if (lines[i].includes(pattern)) {
        const matchBlock = {
            lines: []
        };

        const startContext = Math.max(0, i - contextLines);
        const endContext = Math.min(lines.length - 1, i + contextLines);

        for (let j = startContext; j <= endContext; j++) {
            matchBlock.lines.push({
                number: startOffset + j,
                content: lines[j]
            });
        }
        matches.push(matchBlock);
      }
    }
    
    return matches;
  }
}

module.exports = SnippetService;