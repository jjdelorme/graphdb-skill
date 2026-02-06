const assert = require('node:assert');
const { test, describe } = require('node:test');
const TsAdapter = require('../adapters/TsAdapter');

describe('TsAdapter', () => {
  const adapter = new TsAdapter();

  const sourceCode = `
class BaseClass {}

class MyClass extends BaseClass {
    doWork(x) {
        this.process(x);
        const local = GlobalVar;
    }

    process(val) {
        if (val > 0) {
            console.log(val);
        }
    }
}

function Helper() {
    return 42;
}

const ArrowHelper = (x) => {
    return x + 1;
};
`;

  test('should parse source code', async () => {
    await adapter.init();
    const tree = adapter.parse(sourceCode);
    assert.ok(tree);
    // Tree-sitter-typescript usually produces 'program' as root
    assert.ok(tree.rootNode.type === 'program' || tree.rootNode.type === 'compilation_unit', `Actual root type: ${tree.rootNode.type}`);
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

  test('should scan definitions (Methods/Functions)', async () => {
    await adapter.init();
    const tree = adapter.parse(sourceCode);
    const definitions = adapter.scanDefinitions(tree);

    const method = definitions.find(d => d.name === 'doWork');
    assert.ok(method, 'Should find doWork method');
    assert.strictEqual(method.type, 'Function');

    const func = definitions.find(d => d.name === 'Helper');
    assert.ok(func, 'Should find Helper function');
    assert.strictEqual(func.type, 'Function');

    const arrow = definitions.find(d => d.name === 'ArrowHelper');
    assert.ok(arrow, 'Should find ArrowHelper');
    assert.strictEqual(arrow.type, 'Function');
  });

  test('should scan references (Calls)', async () => {
    await adapter.init();
    const tree = adapter.parse(sourceCode);
    const knownGlobals = new Set(['GlobalVar']);
    const refs = adapter.scanReferences(tree, knownGlobals);

    // doWork calls process
    const call = refs.find(r => r.target === 'process' && r.source === 'doWork');
    assert.ok(call, 'Should find call to process from doWork');
    assert.strictEqual(call.type, 'Call');
  });

  test('should scan references (Global Usage)', async () => {
    await adapter.init();
    const tree = adapter.parse(sourceCode);
    const knownGlobals = new Set(['GlobalVar']);
    const refs = adapter.scanReferences(tree, knownGlobals);

    const usage = refs.find(r => r.target === 'GlobalVar' && r.source === 'doWork');
    assert.ok(usage, 'Should find usage of GlobalVar in doWork');
    assert.strictEqual(usage.type, 'Usage');
  });
});
