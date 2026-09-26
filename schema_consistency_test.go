package geppetto

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSchemaEnumsMatchEngineVocabulary pins the JSON Schema enum lists to the
// engine's enum constants. The Go constants in types.go are the single source
// of the configuration vocabulary; the schema is checked against them by
// parsing the source itself, so no hand-maintained list can drift silently.
func TestSchemaEnumsMatchEngineVocabulary(t *testing.T) {
	for _, tc := range []struct {
		name     string
		typeName string
		def      string
		property string
	}{
		{"updater kinds", "UpdaterKind", "updater", "kind"},
		{"response curve kinds", "ResponseCurveKind", "responseCurve", "kind"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fromSource := enumValuesFromSource(t, tc.typeName)
			fromSchema := enumValuesFromSchema(t, tc.def, tc.property)
			require.NotEmpty(t, fromSource)
			require.NotEmpty(t, fromSchema)
			assert.Equal(t, fromSource, fromSchema)
		})
	}
}

// Aggregated event kinds are an open, game-defined vocabulary — like
// consideration IDs, not a closed enum. This guards that openness: if anyone
// turns the schema's kind field into a hand-maintained enum list, it becomes
// a second place that can diverge, and this test fails.
func TestSchemaAggregatedEventKindStaysAnOpenVocabulary(t *testing.T) {
	assert.Empty(t, enumValuesFromSchema(t, "aggregatedEventEffect", "kind"))
}

// enumValuesFromSource parses types.go and returns the sorted string values
// of every const declared with the given enum type.
func enumValuesFromSource(t *testing.T, typeName string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "types.go", nil, 0)
	require.NoError(t, err)

	var values []string
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.CONST {
			continue
		}
		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			require.True(t, ok)
			typeIdent, ok := valueSpec.Type.(*ast.Ident)
			if !ok || typeIdent.Name != typeName {
				continue
			}
			for _, value := range valueSpec.Values {
				literal, ok := value.(*ast.BasicLit)
				require.True(t, ok, "enum const of %s must be a string literal", typeName)
				unquoted, err := strconv.Unquote(literal.Value)
				require.NoError(t, err)
				values = append(values, unquoted)
			}
		}
	}
	sort.Strings(values)
	return values
}

// enumValuesFromSchema returns the sorted enum list declared at
// $defs/<def>/properties/<property> in the profile schema, or nil when the
// property is not enum-constrained.
func enumValuesFromSchema(t *testing.T, def, property string) []string {
	t.Helper()
	data, err := os.ReadFile("configs/schema/profile.schema.json")
	require.NoError(t, err)

	var schema struct {
		Defs map[string]struct {
			Properties map[string]struct {
				Enum []string `json:"enum"`
			} `json:"properties"`
		} `json:"$defs"`
	}
	require.NoError(t, json.Unmarshal(data, &schema))

	values := schema.Defs[def].Properties[property].Enum
	sort.Strings(values)
	return values
}
