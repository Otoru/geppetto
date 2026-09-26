package engine

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// TestProductionFilesImportOnlyTheStandardLibrary keeps the decision core
// independently importable: transport and application dependencies belong
// outside this package.
func TestProductionFilesImportOnlyTheStandardLibrary(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate this test file")
	}
	directory := filepath.Dir(thisFile)
	fileSet := token.NewFileSet()
	packages, err := parser.ParseDir(fileSet, directory, func(info fs.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse production engine files: %v", err)
	}

	enginePackage, ok := packages["engine"]
	if !ok {
		t.Fatal("production engine package was not found")
	}
	var productionFiles map[string]*ast.File = enginePackage.Files
	for fileName, file := range productionFiles {
		for _, importSpec := range file.Imports {
			importPath, err := strconv.Unquote(importSpec.Path.Value)
			if err != nil {
				t.Fatalf("unquote import in %s: %v", fileName, err)
			}
			importedPackage, err := build.Default.Import(importPath, directory, build.FindOnly)
			if err != nil || !importedPackage.Goroot {
				t.Errorf("%s imports non-standard-library package %q", filepath.Base(fileName), importPath)
			}
		}
	}
}
