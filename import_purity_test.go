package geppetto

import (
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// TestProductionFilesImportOnlyTheStandardLibrary keeps the decision core
// independently importable: transport and application dependencies belong
// outside this package.
//
// The directory is listed and each production file is parsed on its own.
// parser.ParseDir has been deprecated since Go 1.25 and it ignores build
// constraints, so a file the compiler would skip could still fail this
// boundary. build.Default.MatchFile applies the constraints the build uses.
// scanned records that at least one production file was examined; an empty
// scan fails instead of passing in silence.
func TestProductionFilesImportOnlyTheStandardLibrary(t *testing.T) {
	directory, files := productionFiles(t)
	scanned := false
	for _, fileName := range files {
		scanned = true
		for _, importPath := range importPaths(t, directory, fileName) {
			assertStandardLibraryImport(t, directory, fileName, importPath)
		}
	}
	if !scanned {
		t.Fatal("production geppetto package was not found")
	}
}

// productionFiles lists non-test Go files in this package that match the
// default build constraints.
func productionFiles(t *testing.T) (string, []string) {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate this test file")
	}
	directory := filepath.Dir(thisFile)
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("list production engine files: %v", err)
	}

	var files []string
	for _, entry := range entries {
		fileName := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(fileName, ".go") || strings.HasSuffix(fileName, "_test.go") {
			continue
		}
		matches, err := build.Default.MatchFile(directory, fileName)
		if err != nil {
			t.Fatalf("evaluate build constraints for %s: %v", fileName, err)
		}
		if !matches {
			continue
		}
		files = append(files, fileName)
	}
	return directory, files
}

// importPaths parses one production file and returns its import paths.
func importPaths(t *testing.T, directory, fileName string) []string {
	t.Helper()
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, filepath.Join(directory, fileName), nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse production engine files: %v", err)
	}
	paths := make([]string, 0, len(file.Imports))
	for _, importSpec := range file.Imports {
		importPath, err := strconv.Unquote(importSpec.Path.Value)
		if err != nil {
			t.Fatalf("unquote import in %s: %v", fileName, err)
		}
		paths = append(paths, importPath)
	}
	return paths
}

// assertStandardLibraryImport fails when an import does not resolve inside GOROOT.
func assertStandardLibraryImport(t *testing.T, directory, fileName, importPath string) {
	t.Helper()
	importedPackage, err := build.Default.Import(importPath, directory, build.FindOnly)
	if err != nil || !importedPackage.Goroot {
		t.Errorf("%s imports non-standard-library package %q", filepath.Base(fileName), importPath)
	}
}
