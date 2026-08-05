package capability

import (
	"testing"
)

func TestAllIsDeterministic(t *testing.T) {
	all := All()
	expected := []Capability{
		Network,
		Filesystem,
		Process,
		Environment,
		Database,
	}
	if len(all) != len(expected) {
		t.Fatalf("All() returned %d capabilities, expected %d", len(all), len(expected))
	}
	for i, cap := range all {
		if cap != expected[i] {
			t.Errorf("All()[%d] = %q, expected %q", i, cap, expected[i])
		}
	}
}

func TestForConstruct_Known(t *testing.T) {
	cases := []struct {
		construct string
		expected  Capability
	}{
		{"db_connect", Database},
		{"fetch", Network},
		{"read_file", Filesystem},
		{"exec", Process},
		{"env", Environment},
	}
	for _, tc := range cases {
		if got := ForConstruct(tc.construct); got != tc.expected {
			t.Errorf("ForConstruct(%q) = %q, want %q", tc.construct, got, tc.expected)
		}
	}
}

func TestForConstruct_Unknown(t *testing.T) {
	if got := ForConstruct("not_a_real_construct"); got != None {
		t.Errorf("ForConstruct(\"not_a_real_construct\") = %q, want %q", got, None)
	}
}
