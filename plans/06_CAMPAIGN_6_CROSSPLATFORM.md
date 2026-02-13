# Campaign 6: Cross-Platform Distribution

**Goal:** Ship a single, zero-dependency binary for all major OSs (Linux, macOS, Windows) using a Zig-based cross-compilation pipeline.

## 1. Background & Motivation
The `graphdb` tool is currently built locally using `go build`. It depends on `go-tree-sitter` which uses CGO (C bindings). Standard `GOOS=... GOARCH=... go build` fails because it requires a C cross-compiler.
To support developers on different platforms (Linux, macOS, Windows), we need a robust cross-compilation strategy. Zig (`zig cc`) is the industry standard for easy C/C++ cross-compilation.

## 2. Strategy: Zig as Cross-Compiler
We will use `zig cc` as the C compiler for `go build` via `CC="zig cc -target ..."`.

### Targeted Platforms
| Platform | GOOS | GOARCH | Zig Target |
| :--- | :--- | :--- | :--- |
| **Linux x64** | `linux` | `amd64` | `x86_64-linux-musl` |
| **Linux ARM64** | `linux` | `arm64` | `aarch64-linux-musl` |
| **macOS Intel** | `darwin` | `amd64` | `x86_64-macos-none` |
| **macOS Silicon** | `darwin` | `arm64` | `aarch64-macos-none` |
| **Windows x64** | `windows` | `amd64` | `x86_64-windows-gnu` |

## 3. Implementation Plan

### Phase 1: Local Build Script (`scripts/build_release.sh`)
**Owner:** Engineer
**Tasks:**
1.  Create `scripts/build_release.sh`.
2.  Implement logic to check for `zig`.
3.  Iterate through the target platforms.
4.  Run `go build` with **properly escaped environment variables**:
    *   `CGO_ENABLED=1`
    *   `CC="zig cc -target <zig_target>"`
    *   `CXX="zig c++ -target <zig_target>"`
5.  Output binaries to `dist/`.

### Phase 2: Validation & Testing
**Owner:** Auditor
**Tasks:**
1.  Verify the binaries actually run on different platforms (if possible locally).
2.  Ensure `tree-sitter` bindings work (e.g., parse a simple file).
3.  Check binary sizes and linking (static linking preferred where possible).

## 4. Acceptance Criteria
- [x] `scripts/build_release.sh` exists and works on the developer's machine (assuming Zig is installed).
- [ ] Binaries are executable on target platforms and can parse code (Tree-sitter functional).
