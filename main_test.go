package main

import (
	"testing"
)

func TestIndexDirectory(t *testing.T) {
	// Index ourselves!
	index, err := indexDirectory(".")
	if err != nil {
		t.Fatalf("failed to index current directory: %v", err)
	}

	if len(index.Files) == 0 {
		t.Fatal("expected at least one file to be indexed")
	}

	// Find main.go in the results
	var mainFile *FileIndex
	for i := range index.Files {
		if index.Files[i].Path == "main.go" {
			mainFile = &index.Files[i]
			break
		}
	}

	if mainFile == nil {
		t.Fatal("expected main.go to be indexed")
	}

	if mainFile.Package != "main" {
		t.Errorf("expected package 'main', got %q", mainFile.Package)
	}

	// Check that we found some of our own functions
	funcNames := make(map[string]bool)
	for _, fn := range mainFile.Functions {
		funcNames[fn.Name] = true
	}

	expectedFuncs := []string{"main", "handleRequest", "indexDirectory", "indexFile", "filterIndex", "loadConfig"}
	for _, name := range expectedFuncs {
		if !funcNames[name] {
			t.Errorf("expected to find function %q", name)
		}
	}

	// Check that we found our own structs
	structNames := make(map[string]bool)
	for _, s := range mainFile.Structs {
		structNames[s] = true
	}

	expectedStructs := []string{"Config", "Index", "FileIndex", "Function", "Param"}
	for _, name := range expectedStructs {
		if !structNames[name] {
			t.Errorf("expected to find struct %q", name)
		}
	}
}

func TestFilterByExactName(t *testing.T) {
	index, err := indexDirectory(".")
	if err != nil {
		t.Fatalf("failed to index: %v", err)
	}

	filtered := filterIndex(index, "main", "", "")

	// Should find exactly one function named "main"
	count := 0
	for _, file := range filtered.Files {
		for _, fn := range file.Functions {
			if fn.Name == "main" {
				count++
			} else {
				t.Errorf("unexpected function %q in filtered results", fn.Name)
			}
		}
	}

	if count != 1 {
		t.Errorf("expected 1 'main' function, got %d", count)
	}
}

func TestFilterByNameContains(t *testing.T) {
	index, err := indexDirectory(".")
	if err != nil {
		t.Fatalf("failed to index: %v", err)
	}

	// Filter is case-sensitive: "Index" matches FileIndex, Index, filterIndex but not indexDirectory
	filtered := filterIndex(index, "", "Index", "")

	foundStructs := make(map[string]bool)
	foundFuncs := make(map[string]bool)

	for _, file := range filtered.Files {
		for _, s := range file.Structs {
			foundStructs[s] = true
		}
		for _, fn := range file.Functions {
			foundFuncs[fn.Name] = true
		}
	}

	if !foundStructs["Index"] {
		t.Error("expected to find struct 'Index'")
	}
	if !foundStructs["FileIndex"] {
		t.Error("expected to find struct 'FileIndex'")
	}
	if !foundFuncs["filterIndex"] {
		t.Error("expected to find function 'filterIndex'")
	}

	// These should NOT match because "index" != "Index" (case-sensitive)
	if foundFuncs["indexDirectory"] {
		t.Error("indexDirectory should not match 'Index' (case-sensitive)")
	}
}

func TestFilterByKind(t *testing.T) {
	index, err := indexDirectory(".")
	if err != nil {
		t.Fatalf("failed to index: %v", err)
	}

	// Filter for structs only
	filtered := filterIndex(index, "", "", "struct")

	for _, file := range filtered.Files {
		if len(file.Functions) > 0 {
			t.Errorf("expected no functions when filtering by kind=struct, got %d", len(file.Functions))
		}
		if len(file.Interfaces) > 0 {
			t.Errorf("expected no interfaces when filtering by kind=struct, got %d", len(file.Interfaces))
		}
	}

	// Should have some structs
	totalStructs := 0
	for _, file := range filtered.Files {
		totalStructs += len(file.Structs)
	}
	if totalStructs == 0 {
		t.Error("expected at least one struct")
	}
}

func TestFilterByKindAndName(t *testing.T) {
	index, err := indexDirectory(".")
	if err != nil {
		t.Fatalf("failed to index: %v", err)
	}

	// Find structs containing "Config"
	filtered := filterIndex(index, "", "Config", "struct")

	for _, file := range filtered.Files {
		for _, s := range file.Structs {
			if s != "Config" && s != "ServerConfig" && s != "ToolsCapability" {
				// Only Config should match
				if s != "Config" {
					t.Errorf("unexpected struct %q", s)
				}
			}
		}
		if len(file.Functions) > 0 {
			t.Error("expected no functions when filtering by kind=struct")
		}
	}
}

func TestMatchesFilter(t *testing.T) {
	tests := []struct {
		name         string
		symbolName   string
		exactName    string
		contains     string
		shouldMatch  bool
	}{
		{"exact match", "foo", "foo", "", true},
		{"exact no match", "foo", "bar", "", false},
		{"contains match", "fooBar", "", "Bar", true},
		{"contains no match", "fooBar", "", "baz", false},
		{"both exact and contains", "fooBar", "fooBar", "Bar", true},
		{"exact match contains fail", "fooBar", "fooBar", "baz", false},
		{"no filters", "anything", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesFilter(tt.symbolName, tt.exactName, tt.contains)
			if got != tt.shouldMatch {
				t.Errorf("matchesFilter(%q, %q, %q) = %v, want %v",
					tt.symbolName, tt.exactName, tt.contains, got, tt.shouldMatch)
			}
		})
	}
}

func TestFunctionSignatures(t *testing.T) {
	index, err := indexDirectory(".")
	if err != nil {
		t.Fatalf("failed to index: %v", err)
	}

	// Find filterIndex function and check its signature
	filtered := filterIndex(index, "filterIndex", "", "function")

	var filterFn *Function
	for _, file := range filtered.Files {
		for i := range file.Functions {
			if file.Functions[i].Name == "filterIndex" {
				filterFn = &file.Functions[i]
				break
			}
		}
	}

	if filterFn == nil {
		t.Fatal("expected to find filterIndex function")
	}

	// Check params: (index *Index, name, nameContains, kind string)
	if len(filterFn.Params) != 4 {
		t.Errorf("expected 4 params, got %d", len(filterFn.Params))
	}

	// Check return type: *Index
	if len(filterFn.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(filterFn.Results))
	}
	if filterFn.Results[0].Type != "*Index" {
		t.Errorf("expected return type '*Index', got %q", filterFn.Results[0].Type)
	}
}

func TestSkipsTestFiles(t *testing.T) {
	index, err := indexDirectory(".")
	if err != nil {
		t.Fatalf("failed to index: %v", err)
	}

	for _, file := range index.Files {
		if file.Path == "main_test.go" {
			t.Error("test files should be skipped")
		}
	}
}

func TestEmptyFilter(t *testing.T) {
	index, err := indexDirectory(".")
	if err != nil {
		t.Fatalf("failed to index: %v", err)
	}

	// Empty filter should return everything
	filtered := filterIndex(index, "", "", "")

	if len(filtered.Files) != len(index.Files) {
		t.Errorf("empty filter should return all files, got %d want %d",
			len(filtered.Files), len(index.Files))
	}
}
