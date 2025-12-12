# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Run

```bash
make build   # Build the binary
make run     # Run directly (reads JSON-RPC from stdin)
make clean   # Remove built binary
```

No external dependencies - uses only Go standard library.

## Architecture

This is an MCP (Model Context Protocol) server that provides a single tool: `index_go_symbols`. It indexes Go source files to extract top-level declarations (functions, structs, interfaces, constants, variables).

**Protocol**: JSON-RPC 2.0 over stdin/stdout. Each request/response is a single JSON line.

**Supported MCP methods**:
- `initialize` - Returns server capabilities and info
- `notifications/initialized` - No-op acknowledgment
- `tools/list` - Returns available tools (just `index_go_symbols`)
- `tools/call` - Executes a tool

**Indexing behavior**:
- Recursively walks directories starting from specified path (defaults to cwd)
- Skips: hidden directories, `vendor/`, `node_modules/`, `*_test.go`, `*.pb.go`
- Uses `go/parser` to extract declarations from each file
- Returns JSON with file paths, package names, and all top-level symbols with their signatures
