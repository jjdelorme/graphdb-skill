const { test, describe, it, before, after } = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const SnippetService = require('../scripts/tools/SnippetService.js');

const TEMP_FILE = path.join(__dirname, 'temp_test_file.txt');

describe('SnippetService', () => {
  before(() => {
    const lines = Array.from({ length: 10 }, (_, i) => `Line ${i + 1}`);
    fs.writeFileSync(TEMP_FILE, lines.join('\n'));
  });

  after(() => {
    if (fs.existsSync(TEMP_FILE)) {
      fs.unlinkSync(TEMP_FILE);
    }
  });

  describe('sliceFile', () => {
    it('should read specific lines (1-based)', async () => {
      const result = await SnippetService.sliceFile(TEMP_FILE, 2, 4);
      assert.strictEqual(result.trim(), 'Line 2\nLine 3\nLine 4');
    });

    it('should handle single line range', async () => {
        const result = await SnippetService.sliceFile(TEMP_FILE, 5, 5);
        assert.strictEqual(result.trim(), 'Line 5');
    });

    it('should clamp end line if it exceeds file length', async () => {
        const result = await SnippetService.sliceFile(TEMP_FILE, 9, 20);
        assert.strictEqual(result.trim(), 'Line 9\nLine 10');
    });

    it('should throw error for non-existent file', async () => {
        await assert.rejects(async () => {
            await SnippetService.sliceFile('non_existent_file.txt', 1, 5);
        });
    });
  });

  describe('findPatternInScope', () => {
      const content = `function calculateTax(amount) {
    // This is a comment
    const rate = 0.05;
    return amount * rate;
}
// End of function`;

      it('should find pattern and return context', () => {
          const matches = SnippetService.findPatternInScope(content, 'const rate', 1, 100);
          assert.strictEqual(matches.length, 1);
          
          const match = matches[0];
          assert.ok(match.lines.some(l => l.number === 102 && l.content.includes('const rate')));
          
          assert.ok(match.lines.some(l => l.number === 101 && l.content.includes('// This is a comment')));
          assert.ok(match.lines.some(l => l.number === 103 && l.content.includes('return amount * rate')));
      });

      it('should return empty array if pattern not found', () => {
          const matches = SnippetService.findPatternInScope(content, 'missing pattern', 1);
          assert.strictEqual(matches.length, 0);
      });
  });
});