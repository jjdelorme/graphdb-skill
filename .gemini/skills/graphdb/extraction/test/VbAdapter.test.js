const assert = require('node:assert');
const { test, describe } = require('node:test');
const VbAdapter = require('../adapters/VbAdapter');

describe('VbAdapter', () => {
  const adapter = new VbAdapter();

  const sourceCode = `
Imports System

Public Class MyClass
    Inherits BaseClass

    Public Sub DoWork()
        Dim x As Integer = 10
        Dim y As Integer = GlobalConfig.Timeout
        Helper.Process(x)
        Process(y)
    End Sub

    Public Function Calculate(input As Integer) As Integer
        If input > 0 Then
            Return input * 2
        Else
            Return 0
        End If
    End Function
End Class

Module MyModule
    Public Sub GlobalSub()
        Console.WriteLine("Hello")
    End Sub
End Module
`;

  test('should parse source code', async () => {
    await adapter.init();
    const tree = adapter.parse(sourceCode);
    assert.ok(tree);
    assert.strictEqual(tree.rootNode.type, 'source_file');
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

  test('should scan definitions (Modules)', async () => {
    await adapter.init();
    const tree = adapter.parse(sourceCode);
    const definitions = adapter.scanDefinitions(tree);

    // Modules often treated as Classes in some parsers or distinct
    // The adapter logic for 'module_block' sets type to 'Class' currently in VbAdapter code I read
    const mod = definitions.find(d => d.name === 'MyModule');
    assert.ok(mod, 'Should find MyModule');
    assert.strictEqual(mod.type, 'Class'); 
  });

  test('should scan definitions (Methods)', async () => {
    await adapter.init();
    const tree = adapter.parse(sourceCode);
    const definitions = adapter.scanDefinitions(tree);

    const sub = definitions.find(d => d.name === 'DoWork');
    assert.ok(sub, 'Should find DoWork');
    assert.strictEqual(sub.type, 'Function');

    const func = definitions.find(d => d.name === 'Calculate');
    assert.ok(func, 'Should find Calculate');
    assert.strictEqual(func.type, 'Function');
    assert.ok(func.complexity > 1, 'Calculate should have complexity > 1 (due to If)');
  });

  test('should scan references (Calls)', async () => {
    await adapter.init();
    const tree = adapter.parse(sourceCode);
    const knownGlobals = new Set(['GlobalConfig']);
    const refs = adapter.scanReferences(tree, knownGlobals);

    // Helper.Process(x) -> Call to Process
    // Note: VbAdapter logic "if target.type === member_access ... calleeName = member.text"
    // So 'Helper.Process' -> 'Process'
    const processCall = refs.find(r => r.target === 'Process' && r.source === 'DoWork');
    assert.ok(processCall, 'Should find call to Process');
    assert.strictEqual(processCall.type, 'Call');
  });

  test('should scan references (Global Usage)', async () => {
    await adapter.init();
    const tree = adapter.parse(sourceCode);
    const knownGlobals = new Set(['GlobalConfig']);
    const refs = adapter.scanReferences(tree, knownGlobals);

    // Dim y As Integer = GlobalConfig.Timeout
    // Usage of GlobalConfig
    const globalUsage = refs.find(r => r.target === 'GlobalConfig' && r.source === 'DoWork');
    assert.ok(globalUsage, 'Should find usage of GlobalConfig');
    assert.strictEqual(globalUsage.type, 'Usage');
  });
});
