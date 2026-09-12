// Cross-language contract test: proves the vendored howl.* JSON Schema is
// genuinely useful to a consumer that has never seen HowlDream's Python
// implementation. This test imports no Python, no `howldream` package, and
// makes no network calls — only the local vendored schema files and this
// pure-Go JSON Schema validator.
package howl

import (
	"encoding/json"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// validExplorationResult is a representative, schema-conformant
// howl.exploration_result/v1 envelope, including a nested candidate.
const validExplorationResult = `{
  "schema_version": "howl.exploration_result/v1",
  "exploration_id": "exp-go-001",
  "parent_request_id": "req-go-001",
  "objective": "Explore alternative diagnostic strategies",
  "originating_component": "howlplane",
  "authority": {"type": "ADVISORY", "executable": false},
  "candidates": [
    {
      "schema_version": "howl.candidate/v1",
      "candidate_id": "cand-go-001",
      "source_run_id": "run-go-001",
      "parent_request_id": "req-go-001",
      "objective": "Explore alternative diagnostic strategies",
      "text": "IDEA: add a bounded retry with jitter",
      "trust": "UNVERIFIED",
      "status": "GENERATED",
      "authority": {"type": "ADVISORY", "executable": false},
      "provenance": {"run_id": "run-go-001", "producer_component": "howldream"}
    }
  ],
  "verification_status": "UNVERIFIED",
  "recommended_disposition": "DEFER",
  "provenance": {"run_id": "run-go-001", "producer_component": "howldream"}
}`

func compileExplorationResult(t *testing.T) *jsonschema.Schema {
	t.Helper()
	schema, err := Compile(ExplorationResult)
	if err != nil {
		t.Fatalf("compile %s: %v", ExplorationResult, err)
	}
	return schema
}

// mutate returns a deep copy of validExplorationResult with the given
// mutator applied to its decoded form, re-encoded to JSON.
func mutate(t *testing.T, mutator func(env map[string]interface{})) []byte {
	t.Helper()
	var env map[string]interface{}
	if err := json.Unmarshal([]byte(validExplorationResult), &env); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	mutator(env)
	out, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal mutated fixture: %v", err)
	}
	return out
}

func TestValidEnvelopeValidates(t *testing.T) {
	schema := compileExplorationResult(t)
	if err := ValidateJSON(schema, []byte(validExplorationResult)); err != nil {
		t.Fatalf("expected valid envelope to validate, got: %v", err)
	}
}

func TestMissingRequiredFieldFails(t *testing.T) {
	schema := compileExplorationResult(t)
	bad := mutate(t, func(env map[string]interface{}) {
		delete(env, "objective")
	})
	if err := ValidateJSON(schema, bad); err == nil {
		t.Fatal("expected missing required field 'objective' to fail validation")
	}
}

func TestUnsupportedSchemaVersionFails(t *testing.T) {
	schema := compileExplorationResult(t)
	bad := mutate(t, func(env map[string]interface{}) {
		env["schema_version"] = "howl.bogus/v99"
	})
	if err := ValidateJSON(schema, bad); err == nil {
		t.Fatal("expected forged schema_version to fail validation")
	}
}

func TestInvalidEnumFails(t *testing.T) {
	schema := compileExplorationResult(t)
	bad := mutate(t, func(env map[string]interface{}) {
		env["verification_status"] = "VERIFIED"
	})
	if err := ValidateJSON(schema, bad); err == nil {
		t.Fatal("expected out-of-enum verification_status to fail validation")
	}
}

func TestForgedAuthorityExecutableFails(t *testing.T) {
	schema := compileExplorationResult(t)
	bad := mutate(t, func(env map[string]interface{}) {
		env["authority"] = map[string]interface{}{"type": "ADVISORY", "executable": true}
	})
	if err := ValidateJSON(schema, bad); err == nil {
		t.Fatal("expected authority.executable=true to fail validation")
	}
}

func TestInjectedPrivilegedFieldFails(t *testing.T) {
	schema := compileExplorationResult(t)
	for _, field := range []string{"executor", "execution_capability", "approved", "bypass_review"} {
		field := field
		t.Run(field, func(t *testing.T) {
			bad := mutate(t, func(env map[string]interface{}) {
				env[field] = true
			})
			if err := ValidateJSON(schema, bad); err == nil {
				t.Fatalf("expected injected privileged field %q to fail validation (additionalProperties: false)", field)
			}
		})
	}
}

func TestNestedCandidateForgedTrustFails(t *testing.T) {
	schema := compileExplorationResult(t)
	bad := mutate(t, func(env map[string]interface{}) {
		candidates := env["candidates"].([]interface{})
		cand := candidates[0].(map[string]interface{})
		cand["trust"] = "VERIFIED"
	})
	if err := ValidateJSON(schema, bad); err == nil {
		t.Fatal("expected nested candidate with forged trust='VERIFIED' to fail validation")
	}
}

func TestAllFiveVendoredSchemasCompile(t *testing.T) {
	for _, name := range []string{Exploration, Candidate, Assessment, DevelopmentResult, ExplorationResult} {
		name := name
		t.Run(name, func(t *testing.T) {
			if _, err := Compile(name); err != nil {
				t.Fatalf("expected vendored schema %s to compile, got: %v", name, err)
			}
		})
	}
}
