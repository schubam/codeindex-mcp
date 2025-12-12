package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// JSON-RPC 2.0 types
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string     `json:"jsonrpc"`
	ID      any        `json:"id"`
	Result  any        `json:"result,omitempty"`
	Error   *RPCError  `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCP types
type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
}

type Capabilities struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

type ToolsCapability struct{}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Tool struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties,omitempty"`
	Required   []string            `json:"required,omitempty"`
}

type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

type ToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ToolCallResult struct {
	Content []ContentBlock `json:"content"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Config types
type Config struct {
	Exclude []string `json:"exclude"`
	Include []string `json:"include"`
}

// Index types (from codeindex)
type Param struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type Function struct {
	Name         string  `json:"name"`
	Line         int     `json:"line"`
	ReceiverType string  `json:"receiver_type,omitempty"`
	Params       []Param `json:"params"`
	Results      []Param `json:"results"`
	Doc          string  `json:"doc,omitempty"`
}

type FileIndex struct {
	Path       string     `json:"path"`
	Package    string     `json:"package"`
	Functions  []Function `json:"functions"`
	Structs    []string   `json:"structs"`
	Interfaces []string   `json:"interfaces"`
	Constants  []string   `json:"constants,omitempty"`
	Variables  []string   `json:"variables,omitempty"`
}

type Index struct {
	Files []FileIndex `json:"files"`
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	// Increase buffer size for large messages
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			sendError(nil, -32700, "Parse error")
			continue
		}

		handleRequest(req)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "scanner error: %v\n", err)
	}
}

func handleRequest(req JSONRPCRequest) {
	switch req.Method {
	case "initialize":
		sendResult(req.ID, InitializeResult{
			ProtocolVersion: "2024-11-05",
			Capabilities: Capabilities{
				Tools: &ToolsCapability{},
			},
			ServerInfo: ServerInfo{
				Name:    "codeindex",
				Version: "1.0.0",
			},
		})

	case "notifications/initialized":
		// No response needed for notifications

	case "tools/list":
		sendResult(req.ID, ToolsListResult{
			Tools: []Tool{
				{
					Name:        "index_go_symbols",
					Description: "Index Go symbols (functions, structs, interfaces, constants, variables) in a directory. Returns JSON with all top-level declarations.",
					InputSchema: InputSchema{
						Type: "object",
						Properties: map[string]Property{
							"directory": {
								Type:        "string",
								Description: "Path to the directory to index. Defaults to current working directory if not specified.",
							},
						},
					},
				},
			},
		})

	case "tools/call":
		var params ToolCallParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			sendError(req.ID, -32602, "Invalid params")
			return
		}

		if params.Name != "index_go_symbols" {
			sendError(req.ID, -32602, fmt.Sprintf("Unknown tool: %s", params.Name))
			return
		}

		// Parse arguments
		var args struct {
			Directory string `json:"directory"`
		}
		if len(params.Arguments) > 0 {
			json.Unmarshal(params.Arguments, &args)
		}
		if args.Directory == "" {
			args.Directory = "."
		}

		// Run indexer
		index, err := indexDirectory(args.Directory)
		if err != nil {
			sendError(req.ID, -32000, err.Error())
			return
		}

		// Serialize result
		resultJSON, err := json.Marshal(index)
		if err != nil {
			sendError(req.ID, -32000, err.Error())
			return
		}

		sendResult(req.ID, ToolCallResult{
			Content: []ContentBlock{
				{Type: "text", Text: string(resultJSON)},
			},
		})

	default:
		sendError(req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method))
	}
}

func sendResult(id any, result any) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
}

func sendError(id any, code int, message string) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &RPCError{Code: code, Message: message},
	}
	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
}

// loadConfig reads .codeindex.json from the given directory
func loadConfig(root string) *Config {
	configPath := filepath.Join(root, ".codeindex.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return &Config{}
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse .codeindex.json: %v\n", err)
		return &Config{}
	}
	return &cfg
}

// shouldSkip checks if a path should be skipped based on config
func shouldSkip(path string, root string, cfg *Config) bool {
	relPath, err := filepath.Rel(root, path)
	if err != nil {
		relPath = path
	}

	// Check if explicitly included (takes priority)
	for _, inc := range cfg.Include {
		inc = filepath.Clean(inc)
		if relPath == inc || strings.HasPrefix(relPath, inc+string(filepath.Separator)) {
			return false
		}
	}

	// Check if excluded
	for _, exc := range cfg.Exclude {
		exc = filepath.Clean(exc)
		if relPath == exc || strings.HasPrefix(relPath, exc+string(filepath.Separator)) {
			return true
		}
	}

	return false
}

// Indexing logic (from codeindex)
func indexDirectory(root string) (*Index, error) {
	index := &Index{}
	cfg := loadConfig(root)

	// Convert root to absolute path for consistent matching
	absRoot, err := filepath.Abs(root)
	if err != nil {
		absRoot = root
	}

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Convert to absolute for matching
		absPath, err := filepath.Abs(path)
		if err != nil {
			absPath = path
		}

		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			// Check config-based exclusions for directories
			if shouldSkip(absPath, absRoot, cfg) {
				return filepath.SkipDir
			}
			return nil
		}

		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		if strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, ".pb.go") {
			return nil
		}

		// Check config-based exclusions for files
		if shouldSkip(absPath, absRoot, cfg) {
			return nil
		}

		fi, err := indexFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to index %s: %v\n", path, err)
			return nil
		}
		if fi != nil {
			index.Files = append(index.Files, *fi)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return index, nil
}

func indexFile(path string) (*FileIndex, error) {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	fi := &FileIndex{
		Path:    path,
		Package: parsed.Name.Name,
	}

	for _, decl := range parsed.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			fn := Function{
				Name:    d.Name.Name,
				Line:    fset.Position(d.Pos()).Line,
				Params:  paramsFromFieldList(d.Type.Params),
				Results: paramsFromFieldList(d.Type.Results),
			}
			if d.Doc != nil {
				fn.Doc = strings.TrimSpace(d.Doc.Text())
			}
			if d.Recv != nil && len(d.Recv.List) > 0 {
				fn.ReceiverType = exprToString(d.Recv.List[0].Type)
			}
			fi.Functions = append(fi.Functions, fn)

		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					switch s.Type.(type) {
					case *ast.StructType:
						fi.Structs = append(fi.Structs, s.Name.Name)
					case *ast.InterfaceType:
						fi.Interfaces = append(fi.Interfaces, s.Name.Name)
					}
				case *ast.ValueSpec:
					for _, name := range s.Names {
						if d.Tok == token.CONST {
							fi.Constants = append(fi.Constants, name.Name)
						} else if d.Tok == token.VAR {
							fi.Variables = append(fi.Variables, name.Name)
						}
					}
				}
			}
		}
	}

	if len(fi.Functions) == 0 && len(fi.Structs) == 0 && len(fi.Interfaces) == 0 {
		return nil, nil
	}

	return fi, nil
}

func paramsFromFieldList(fl *ast.FieldList) []Param {
	if fl == nil {
		return nil
	}
	var params []Param
	for _, field := range fl.List {
		t := exprToString(field.Type)
		if len(field.Names) == 0 {
			params = append(params, Param{Name: "", Type: t})
			continue
		}
		for _, name := range field.Names {
			params = append(params, Param{Name: name.Name, Type: t})
		}
	}
	return params
}

func exprToString(expr ast.Expr) string {
	var sb strings.Builder
	if err := printer.Fprint(&sb, token.NewFileSet(), expr); err != nil {
		return ""
	}
	return sb.String()
}
