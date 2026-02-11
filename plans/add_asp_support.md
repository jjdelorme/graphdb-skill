# Feature Implementation Plan: Add Classic ASP Support

## 📋 Todo Checklist
- [x] ~~Create `internal/analysis/asp.go` with `AspParser` struct.~~ ✅ Implemented
- [x] ~~Implement `detectLanguage` logic (File extension + Directive check).~~ ✅ Implemented
- [x] ~~Implement `maskHtml` logic (Preserve code blocks, mask others with whitespace).~~ ✅ Implemented
- [x] ~~Implement `Parse` method with delegation to C# or VB.NET parsers.~~ ✅ Implemented
- [x] ~~Register `AspParser` for `.asp`, `.aspx`, `.ascx` in `internal/analysis/parser.go`.~~ ✅ Implemented
- [x] ~~Add unit tests in `internal/analysis/asp_test.go` covering mixed content.~~ ✅ Implemented
- [x] ~~Verify parity with Node.js `AspAdapter.js`.~~ ✅ Verified

## 🔍 Analysis & Investigation

### Current Architecture
*   **Parser Interface:** defined in `internal/analysis/parser.go`: `Parse(filePath string, content []byte) ([]*graph.Node, []*graph.Edge, error)`.
*   **Delegates:** `CSharpParser` (TreeSitter) and `VBNetParser` (Regex) exist and are registered.
*   **Node.js Parity:** The existing `AspAdapter.js` uses a "Whitespace Masking" strategy to preserve line numbers while hiding HTML from the language parsers. It also handles language detection via directives (`<%@ Language="..." %>`).

### Strategy: Whitespace Masking
To avoid writing a full ASP parser, we will:
1.  **Identify Code Blocks:** `<% ... %>` and `<script runat="server"> ... </script>`.
2.  **Mask HTML:** Replace all characters *outside* these blocks with spaces, preserving newlines (`
`). This ensures that line 10 in the code block remains line 10 in the masked content.
3.  **Delegate:** Pass the masked content to the underlying C# or VB.NET parser.

### Language Detection
*   **Default:** `.asp` -> VBScript, `.aspx`/`.ascx` -> C#.
*   **Override:** Check for `<%@ Page Language="C#" %>` or `<%@ Language="VBScript" %>`.

### VB.NET Specifics
*   The Node.js adapter wraps VB code in `Module AspWrapper ... End Module` to satisfy its parser.
*   The current Go `VBNetParser` is regex-based and doesn't strictly enforce this, but for parity and future-proofing (if we switch to TreeSitter for VB), we should replicate this wrapping.
*   **Line Offset:** Wrapping shifts code by 1 line. We must adjust the returned Node line numbers by `-1` if we wrap.

## 📝 Implementation Plan

### Prerequisites
*   None. Dependencies (TreeSitter for C#) are already present.

### Step-by-Step Implementation

#### Phase 1: Core Logic Implementation
1.  **Step 1.A (The Harness):** Create `internal/analysis/asp_test.go`.
    *   *Action:* Define a test case with a sample `.aspx` file containing C# code mixed with HTML.
    *   *Goal:* Assert that `AspParser` returns nodes for the C# methods with correct line numbers.
2.  **Step 1.B (The Implementation):** Create `internal/analysis/asp.go`.
    *   *Action:* Implement `AspParser` struct.
    *   *Logic:*
        *   `detectLanguage(content, ext)`: Returns `.cs` or `.vb`.
        *   `maskHtml(content)`: Returns masked byte slice.
            *   Use regex `(?s)<%.*?%>` and `(?si)<script.*?runat="server".*?>.*?</script>` to find code.
            *   Careful to exclude directives `<%@` from "Code" (treat as HTML/Whitespace) to match Node logic, OR keep them if they contain relevant info. Node masks them.
        *   `Parse(...)`:
            *   Call `detectLanguage`.
            *   Call `maskHtml`.
            *   If VB, prepend `Module AspWrapper
` and append `
End Module`.
            *   Call `GetParser(lang).Parse(...)`.
            *   If VB, iterate returned nodes and decrement `line` property by 1.
            *   Return results.
3.  **Step 1.C (The Verification):** Run tests.
    *   *Action:* `go test ./internal/analysis/...`
    *   *Success:* The new test passes.

#### Phase 2: Integration
1.  **Step 2.A (Registration):** Update `internal/analysis/parser.go`.
    *   *Action:* In `init()` (or a new `init` in `asp.go`), register `.asp`, `.aspx`, `.ascx` to `AspParser`.
    *   *Note:* It's cleaner to have `func init()` in `asp.go` calling `RegisterParser`.
2.  **Step 2.B (Validation):** Verify full flow.
    *   *Action:* Create a test file `test/fixtures/asp/sample.aspx` (if not exists) and run a CLI extraction test if possible, or extend unit tests to cover multiple extensions.

### Testing Strategy
*   **Unit Tests:**
    *   **C# in ASPX:** Verify methods are extracted and line numbers match the original file (not the masked one, which should be same).
    *   **VB in ASP:** Verify simple VBScript functions are found.
    *   **Language Override:** Test a `.asp` file with `<%@ Language="C#" %>` (rare but possible) or `.aspx` with VB.
*   **Masking Accuracy:** Ensure `<%@ ... %>` directives don't break parsing but are ignored as code.

## 🎯 Success Criteria
*   `AspParser` correctly delegates to `CSharpParser` or `VBNetParser`.
*   Line numbers for extracted code symbols are accurate relative to the original source file.
*   HTML content is ignored and causes no parse errors.
