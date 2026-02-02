const assert = require('node:assert');
const { test, describe } = require('node:test');
const CppAdapter = require('../adapters/CppAdapter');

describe('CppAdapter', () => {
  const adapter = new CppAdapter();

  const sourceCode = `
#include <iostream>

int globalVar = 42;

class MyClass : public BaseClass {
public:
    void method(int x) {
        process(x);
    }
};

void process(int input) {
    if (input > 0) {
        std::cout << input << std::endl;
    } else {
        std::cout << "Zero" << std::endl;
    }
    
    int local = globalVar;
}

int main() {
    MyClass obj;
    obj.method(10);
    return 0;
}
`;

  test('should parse source code', async () => {
    await adapter.init();
    const tree = adapter.parse(sourceCode);
    assert.ok(tree);
    assert.strictEqual(tree.rootNode.type, 'translation_unit');
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

  test('should scan definitions (Functions)', async () => {
    await adapter.init();
    const tree = adapter.parse(sourceCode);
    const definitions = adapter.scanDefinitions(tree);

    const func = definitions.find(d => d.name === 'process');
    assert.ok(func, 'Should find process function');
    assert.strictEqual(func.type, 'Function');
    assert.ok(func.complexity > 1, 'Process should have complexity > 1 (due to If)');

    const method = definitions.find(d => d.name === 'method');
    assert.ok(method, 'Should find method inside class');
    // Note: CppAdapter extracts methods as top-level functions in current logic if they are definitions
    assert.strictEqual(method.type, 'Function');
  });

  test('should scan definitions (Globals)', async () => {
    await adapter.init();
    const tree = adapter.parse(sourceCode);
    const definitions = adapter.scanDefinitions(tree);

    const glob = definitions.find(d => d.name === 'globalVar');
    assert.ok(glob, 'Should find globalVar');
    assert.strictEqual(glob.type, 'Global');
  });

  test('should scan references (Calls)', async () => {
    await adapter.init();
    const tree = adapter.parse(sourceCode);
    const knownGlobals = new Set(['globalVar']);
    const refs = adapter.scanReferences(tree, knownGlobals);

    const call = refs.find(r => r.target === 'process' && r.source === 'method');
    assert.ok(call, 'Should find call to process from method');
    assert.strictEqual(call.type, 'Call');
  });

  test('should scan references (Global Usage)', async () => {
    await adapter.init();
    const tree = adapter.parse(sourceCode);
    const knownGlobals = new Set(['globalVar']);
    const refs = adapter.scanReferences(tree, knownGlobals);

    const usage = refs.find(r => r.target === 'globalVar' && r.source === 'process');
    assert.ok(usage, 'Should find usage of globalVar in process');
    assert.strictEqual(usage.type, 'Usage');
  });
});
