const assert = require('node:assert');
const { test, describe } = require('node:test');
const CsharpAdapter = require('../adapters/CsharpAdapter');

describe('CsharpAdapter', () => {
  const adapter = new CsharpAdapter();

  const sourceCode = `
using System;

public class BaseClass {}

public class MyClass : BaseClass {
    public static int Timeout = 100;

    public void DoWork(int x) {
        Process(x);
        int local = Timeout;
    }

    private void Process(int val) {
        if (val > 0) {
            Console.WriteLine(val);
        }
    }
}
`;

  test('should parse source code', async () => {
    await adapter.init();
    const tree = adapter.parse(sourceCode);
    assert.ok(tree);
    assert.strictEqual(tree.rootNode.type, 'compilation_unit');
  });

  test('should scan definitions (Classes)', async () => {
    await adapter.init();
    const tree = adapter.parse(sourceCode);
    const definitions = adapter.scanDefinitions(tree);

    const cls = definitions.find(d => d.name === 'MyClass');
    assert.ok(cls, 'Should find MyClass');
    assert.strictEqual(cls.type, 'Class');
    assert.ok(cls.inherits.includes('BaseClass'), 'Should inherit BaseClass');
  });

  test('should scan definitions (Methods)', async () => {
    await adapter.init();
    const tree = adapter.parse(sourceCode);
    const definitions = adapter.scanDefinitions(tree);

    const func = definitions.find(d => d.name === 'DoWork');
    assert.ok(func, 'Should find DoWork');
    assert.strictEqual(func.type, 'Function');

    const proc = definitions.find(d => d.name === 'Process');
    assert.ok(proc, 'Should find Process');
    assert.strictEqual(proc.type, 'Function');
    assert.ok(proc.complexity > 1, 'Process should have complexity > 1');
  });

  test('should scan definitions (Globals/Static Fields)', async () => {
    await adapter.init();
    const tree = adapter.parse(sourceCode);
    const definitions = adapter.scanDefinitions(tree);

    const stat = definitions.find(d => d.name === 'Timeout');
    assert.ok(stat, 'Should find static Timeout field');
    assert.strictEqual(stat.type, 'Global');
  });

  test('should scan references (Calls)', async () => {
    await adapter.init();
    const tree = adapter.parse(sourceCode);
    const knownGlobals = new Set(['Timeout']);
    const refs = adapter.scanReferences(tree, knownGlobals);

    const call = refs.find(r => r.target === 'Process' && r.source === 'DoWork');
    assert.ok(call, 'Should find call to Process from DoWork');
    assert.strictEqual(call.type, 'Call');
  });

  test('should scan references (Global Usage)', async () => {
    await adapter.init();
    const tree = adapter.parse(sourceCode);
    const knownGlobals = new Set(['Timeout']);
    const refs = adapter.scanReferences(tree, knownGlobals);

    const usage = refs.find(r => r.target === 'Timeout' && r.source === 'DoWork');
    assert.ok(usage, 'Should find usage of Timeout in DoWork');
    assert.strictEqual(usage.type, 'Usage');
  });
});
