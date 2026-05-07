# mcpx-test

**Category: Local LLM Router**

Test generation and quality analysis server. Generates unit and integration tests from source code using local Ollama, analyzes test coverage gaps, and triages failing test output.

## Domain

Testing lifecycle:
- Generate test cases for a function or file
- Identify untested code paths and edge cases
- Analyze test failure output (pairs with mcpx-debug for runtime errors)
- Measure and report coverage gaps without running the test suite

## Tools

| Tool | Description |
|---|---|
| `test_generate` | Generate unit tests for a function or file. Returns test code ready to write to disk. |
| `test_gaps` | Identify untested branches and edge cases in a function. Returns a list of missing test cases (no code, just descriptions). |
| `test_analyze` | Parse test failure output and explain what failed and why. |
| `test_coverage` | Parse a coverage report (go cover, pytest-cov, lcov) and surface the highest-value uncovered lines. |
| `test_mock` | Generate mock/stub code for an interface or dependency. |

Total: 5 tools.

## Data flow

```
caller → test_generate(path, symbol, lang, repo_path?)
           │
           ├─ extract target function: signature + body
           ├─ extract dependencies: types used, interfaces implemented
           ├─ detect existing test patterns in *_test.go / test_*.py (style matching)
           ├─ build prompt (<1200 tokens):
           │     "Write table-driven unit tests for this Go function.
           │      Match the style of the existing tests. Return only test code."
           │
           └─ POST /api/generate → Ollama → strip markdown fences → return raw code

caller → test_gaps(path, symbol, repo_path?)
           │
           ├─ extract function body
           ├─ static analysis: identify branches (if/else, switch, error returns, nil checks)
           ├─ cross-reference with existing tests (grep for function name in *_test.go)
           └─ return: untested branch descriptions (no LLM needed for simple functions)
                      OR Ollama call for complex logic
```

## Style matching

`test_generate` reads existing test files in the same package before generating. It extracts:
- Test function naming convention (`TestFoo_case` vs `Test_foo`)
- Use of table-driven tests vs individual test functions
- Assertion library (testify, stdlib `t.Errorf`, etc.)

The generated tests match the repo's existing style.

## Language support

| Language | Test generation | Coverage parsing |
|---|---|---|
| Go | full (table-driven, testify) | `go cover -html` profile |
| Python | full (pytest) | `pytest-cov` XML |
| TypeScript | partial (jest) | lcov |

## Output format

`test_generate` returns raw code only (no explanation, no markdown fences):
```go
func TestGateway_Init_ServerUnreachable(t *testing.T) {
    cfg := &config.Config{
        Servers: []config.ServerConfig{{Name: "bad", Binary: "/nonexistent"}},
    }
    gw := gateway.New(cfg, metrics.New())
    err := gw.Init(context.Background())
    require.Error(t, err)
}
```

`test_gaps` returns a plain list:
```
1. nil config.Servers slice (empty server list)
2. server binary not found on disk
3. Init called twice (idempotency)
4. context cancellation mid-init
```

`test_coverage` returns a LEAN table sorted by impact:
```
file                          uncovered_lines  impact
gateway/gateway.go            84-102           high    (error path)
transport/stdio.go            45-51            medium  (fallback branch)
config/config.go              28               low     (default value)
```

## Ollama integration

`test_generate` and `test_mock` always call Ollama. `test_gaps` and `test_coverage` use static analysis and only call Ollama for complex logic (>50 branches).

```
OLLAMA_URL      http://localhost:11434
OLLAMA_MODEL    qwen2.5-coder:7b
OLLAMA_NUM_CTX  8192
```

## Binary

`mcpx-test` — pure Go.
Transport: stdio (default) or HTTP (`MCP_TRANSPORT=http`, `MCP_PORT=8085`).

## Status

**Planned** — not yet implemented.
