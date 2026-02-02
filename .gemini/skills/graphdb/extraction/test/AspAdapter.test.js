const assert = require('node:assert');
const { test, describe, before } = require('node:test');
const AspAdapter = require('../adapters/AspAdapter');
const CsharpAdapter = require('../adapters/CsharpAdapter');
const VbAdapter = require('../adapters/VbAdapter');

describe('AspAdapter', () => {
  const csharpAdapter = new CsharpAdapter();
  const vbAdapter = new VbAdapter();
  const adapter = new AspAdapter({ csharp: csharpAdapter, vb: vbAdapter });

  before(async () => {
    await Promise.all([csharpAdapter.init(), vbAdapter.init()]);
  });

  const csHtmlSource = `
<div>
    <h1>Welcome</h1>
    <%
        var message = "Hello World";
        ProcessMessage(message);
    %>
    <script runat="server">
        public void ProcessMessage(string msg) {
            Console.WriteLine(msg);
        }
    </script>
</div>
`;

  const vbAspxSource = `
<%@ Page Language="VB" %>
<html>
<body>
    <%
        Dim name As String = "User"
        SayHello(name)
    %>
    <script runat="server">
        Sub SayHello(n As String)
            Response.Write("Hello " & n)
        End Sub
    </script>
</body>
</html>
`;

  test('should detect C# (default) and extract definitions', async () => {
    const tree = adapter.parse(csHtmlSource);
    assert.ok(tree, 'Should return a tree');
    assert.strictEqual(tree.userData.adapter, csharpAdapter, 'Should select CsharpAdapter');

    const definitions = adapter.scanDefinitions(tree);
    const method = definitions.find(d => d.name === 'ProcessMessage');
    assert.ok(method, 'Should find ProcessMessage method');
    assert.strictEqual(method.type, 'Function');
  });

  test('should detect VB (via directive) and extract definitions', async () => {
    const tree = adapter.parse(vbAspxSource);
    assert.ok(tree, 'Should return a tree');
    assert.strictEqual(tree.userData.adapter, vbAdapter, 'Should select VbAdapter');

    const definitions = adapter.scanDefinitions(tree);
    const method = definitions.find(d => d.name === 'SayHello');
    assert.ok(method, 'Should find SayHello sub');
    assert.strictEqual(method.type, 'Function'); // VbAdapter maps Sub to Function type
  });

  test('should detect VB in legacy ASP file (via extension or heuristic)', async () => {
    const legacyAspSource = `
<%
    Dim x
    x = 10
    Sub LegacySub()
        Response.Write x
    End Sub
%>
`;
    // Simulate passing a filename if we change the API, 
    // OR rely on content heuristics if we don't.
    // For now, let's assume we pass the filename as a second argument 
    // (which we will implement).
    const tree = adapter.parse(legacyAspSource, 'legacy.asp');
    
    assert.strictEqual(tree.userData.adapter, vbAdapter, 'Should select VbAdapter for .asp');
    
    const definitions = adapter.scanDefinitions(tree);
    const sub = definitions.find(d => d.name === 'LegacySub');
    assert.ok(sub, 'Should find LegacySub');
  });

  test('should mask HTML content', () => {
      // Accessing private method via prototype or just testing via behavior
      // Since it's _maskHtml, it's technically private but likely accessible in JS
      // We'll trust the parse/scan test above implicitly tests masking, 
      // but let's verify line numbers roughly match if possible.
      
      const tree = adapter.parse(csHtmlSource);
      const definitions = adapter.scanDefinitions(tree);
      const method = definitions.find(d => d.name === 'ProcessMessage');
      
      // "public void ProcessMessage" is on line 8 of csHtmlSource
      // 1: (empty line)
      // 2: <div>
      // 3:     <h1>...
      // 4:     <%
      // 5:         var...
      // 6:         Process...
      // 7:     %>
      // 8:     <script...
      // 9:         public void ProcessMessage...
      
      // Tree-sitter uses 0-based row usually, but adapter often converts to 1-based.
      // CsharpAdapter: line: node.startPosition.row + 1
      assert.strictEqual(method.line, 9, 'Line number should match original source');
  });
});