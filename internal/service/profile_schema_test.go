package service

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	profileloader "github.com/vitorhugo/geppetto/profile"
)

// TestProfilesValidateAgainstSchema keeps configs/schema/profile.schema.json
// honest: every profile in configs/ must validate against the schema and still
// load through the same path the server uses. If a struct tag or enum changes
// in the public model, this test fails before the schema rots.
func TestProfilesValidateAgainstSchema(t *testing.T) {
	configDir := filepath.Join("..", "..", "configs")
	schemaPath := filepath.Join(configDir, "schema", "profile.schema.json")

	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaBytes))
	if err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}

	compiler := jsonschema.NewCompiler()
	const schemaURL = "https://geppetto.local/profile.schema.json"
	if err := compiler.AddResource(schemaURL, schemaDoc); err != nil {
		t.Fatalf("add schema resource: %v", err)
	}
	schema, err := compiler.Compile(schemaURL)
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}

	paths, err := filepath.Glob(filepath.Join(configDir, "*.json"))
	if err != nil {
		t.Fatalf("glob profiles: %v", err)
	}
	if len(paths) != 4 {
		t.Fatalf("expected 4 profiles in %s, got %d: %v", configDir, len(paths), paths)
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read profile: %v", err)
			}
			doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("profile is not valid JSON: %v", err)
			}
			if err := schema.Validate(doc); err != nil {
				t.Errorf("profile does not validate against schema: %v", err)
			}
			if _, err := profileloader.Load(path); err != nil {
				t.Errorf("profile no longer loads via profile.Load: %v", err)
			}
		})
	}

	// Guards the loader contract: every *.json in configs/ must be a loadable
	// profile. The schema lives in configs/schema/ precisely because this glob
	// is not recursive and rejects nameless files.
	if _, err := LoadProfiles(configDir); err != nil {
		t.Errorf("LoadProfiles(%s) failed: %v", configDir, err)
	}
}
