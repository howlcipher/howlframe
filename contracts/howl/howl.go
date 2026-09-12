// Package howl loads the vendored howl.* ecosystem contract schemas and
// compiles them for validation. The schema files in this directory are
// vendored, generated copies from HowlDream — see SOURCE.md. This package
// never imports or shells out to howldream; it only reads the local JSON
// Schema files.
package howl

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

//go:embed *.schema.json
var schemaFS embed.FS

// Names of the vendored schema files, keyed by their howl.* family slug.
const (
	Exploration       = "howl.exploration.v1.schema.json"
	Candidate         = "howl.candidate.v1.schema.json"
	Assessment        = "howl.assessment.v1.schema.json"
	DevelopmentResult = "howl.development_result.v1.schema.json"
	ExplorationResult = "howl.exploration_result.v1.schema.json"
)

// Compile loads and compiles the named vendored schema file (one of the
// constants above). It registers the schema under its own declared $id so
// internal $ref resolution works with no network access.
func Compile(fileName string) (*jsonschema.Schema, error) {
	raw, err := schemaFS.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("read vendored schema %s: %w", fileName, err)
	}

	var doc struct {
		ID string `json:"$id"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse $id from %s: %w", fileName, err)
	}
	if doc.ID == "" {
		return nil, fmt.Errorf("schema %s has no $id", fileName)
	}

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(doc.ID, bytes.NewReader(raw)); err != nil {
		return nil, fmt.Errorf("register schema resource %s: %w", doc.ID, err)
	}
	schema, err := compiler.Compile(doc.ID)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", doc.ID, err)
	}
	return schema, nil
}

// ValidateJSON decodes raw JSON and validates it against the compiled schema.
func ValidateJSON(schema *jsonschema.Schema, raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var v interface{}
	if err := decoder.Decode(&v); err != nil {
		return fmt.Errorf("decode envelope JSON: %w", err)
	}
	return schema.Validate(v)
}
