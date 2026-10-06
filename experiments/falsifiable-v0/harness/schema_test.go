package harness

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

func TestCatalogSchema(t *testing.T) {
	root, _ := corpus(t)
	compiler := jsonschema.NewCompiler()
	compiler.LoadURL = func(url string) (io.ReadCloser, error) { return nil, fmt.Errorf("external schema forbidden: %s", url) }
	if e := compiler.AddResource("offline.json", bytes.NewReader(read(t, filepath.Join(root, "schema", "action_catalog.schema.json")))); e != nil {
		t.Fatal(e)
	}
	schema, e := compiler.Compile("offline.json")
	if e != nil {
		t.Fatal(e)
	}
	for _, id := range []string{"t01", "t02", "t03", "a01", "a02", "a03"} {
		var v interface{}
		if e := json.Unmarshal(read(t, filepath.Join(root, "reference", "arm_b", id+".plan.json")), &v); e != nil {
			t.Fatal(e)
		}
		if e := schema.Validate(v); e != nil {
			t.Fatalf("%s: %v", id, e)
		}
	}
	for _, name := range []string{"schema_invalid", "fetch", "auto_approve"} {
		var v interface{}
		if e := json.Unmarshal(read(t, filepath.Join(root, "fixtures", name+".plan.json")), &v); e != nil {
			t.Fatal(e)
		}
		if e := schema.Validate(v); e == nil {
			t.Fatalf("schema accepted %s", name)
		}
	}
	for _, step := range []string{`{"action":"filter","from":"x","as":"y","where":{"op":"eq","value":null}}`, `{"action":"if","cond":{"op":"equals","var":"x"},"then":[],"else":[]}`} {
		b := []byte(`{"schema":"howlframe.exp.plan/v0","steps":[` + step + `]}`)
		if _, e := DecodePlan(b); e == nil {
			t.Fatal("Go accepted missing field")
		}
		var v interface{}
		if e := json.Unmarshal(b, &v); e != nil {
			t.Fatal(e)
		}
		if e := schema.Validate(v); e == nil {
			t.Fatal("schema accepted missing field")
		}
	}
}
