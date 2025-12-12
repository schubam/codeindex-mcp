# codeindex-mcp

A lightweight MCP server that gives LLM agents the ability to index Go symbols in your codebase. Drop it into any project to provide code navigation capabilities to Claude, Cursor, or other MCP-compatible tools.

## Installation

```bash
go install github.com/schubam/codeindex-mcp@latest
```

Make sure `~/go/bin` is in your `PATH`.

## Configuration

### Claude Code

Add to `~/.claude/settings.json`:

```json
{
  "mcpServers": {
    "codeindex": {
      "command": "codeindex-mcp"
    }
  }
}
```

### Claude Desktop

Add to `~/.claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "codeindex": {
      "command": "codeindex-mcp"
    }
  }
}
```

### Building from source

```bash
git clone https://github.com/schubam/codeindex-mcp.git
cd codeindex-mcp
make build
```

## What It Does

Provides a single tool `index_go_symbols` that extracts:

- Functions (with signatures, receivers, params, return types, line numbers)
- Structs and interfaces
- Constants and variables
- Documentation comments

This gives LLM agents a map of your codebase so they can navigate and understand code structure without reading every file.

## Tool Schema

**`index_go_symbols`**

| Parameter | Type | Description |
|-----------|------|-------------|
| `directory` | string | Path to index (defaults to cwd) |

Returns JSON with all top-level declarations organized by file.

## Indexing Behavior

- Recursively walks from the specified directory
- Skips: hidden dirs, `vendor/`, `node_modules/`, `*_test.go`, `*.pb.go`
- Uses Go's `go/parser` for accurate AST extraction

## Requirements

Go 1.21+ (no external dependencies)

## License

MIT
