package capability

import (
	"sort"
	"testing"
)

func TestHostOperationsSorted(t *testing.T) {
	names := make([]string, len(hostOperations))
	for i, op := range hostOperations {
		names[i] = op.Name
	}
	if !sort.StringsAreSorted(names) {
		t.Fatalf("hostOperations must be sorted by name: %v", names)
	}
	for i := 1; i < len(names); i++ {
		if names[i] == names[i-1] {
			t.Fatalf("duplicate host operation %s", names[i])
		}
	}
}

// The usage texts are the ones the checker reported before the table
// existed, so its arity errors read as they did.
func TestHostOperationUsage(t *testing.T) {
	for name, want := range map[string]string{
		"event_attr":       "(event_attr event name)",
		"set_checked":      "(set_checked el bool)",
		"image_encode":     "(image_encode base64 mime max_side out_mime quality)",
		"zip_read":         "(zip_read base64 max_entries max_bytes)",
		"on_visible":       "(on_visible elements lambda)",
		"on_scroll":        "(on_scroll lambda)",
		"on_event":         "(on_event el event lambda)",
		"set_attr":         "(set_attr el name val)",
		"toggle_class":     "(toggle_class el class)",
		"url_encode":       "(url_encode string)",
		"download_file":    "(download_file name mime base64)",
		"richtext_command": "(richtext_command el command value)",
		"scroll_y":         "(scroll_y)",
	} {
		op, ok := LookupHostOperation(name)
		if !ok {
			t.Fatalf("%s is not a host operation", name)
		}
		if got := op.Usage(); got != want {
			t.Errorf("%s usage %q, want %q", name, got, want)
		}
	}
}

func TestHostOperationKinds(t *testing.T) {
	for _, op := range HostOperations() {
		for i, p := range op.Params {
			if p.Kind == HostLambda && i != len(op.Params)-1 {
				t.Errorf("%s: a lambda must be the last parameter", op.Name)
			}
			if p.Kind != HostLambda && p.Lambda != nil {
				t.Errorf("%s: only a lambda parameter has lambda parameters", op.Name)
			}
		}
		if op.Result == HostSubscription && !op.LambdaLast() {
			t.Errorf("%s returns a subscription but takes no lambda", op.Name)
		}
		if op.Capability != None {
			if _, err := ParseGrant(string(op.Capability)); err != nil {
				t.Errorf("%s needs an unknown capability %q", op.Name, op.Capability)
			}
		}
	}
}
