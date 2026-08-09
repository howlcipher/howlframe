package construct

import (
	"sort"
	"strings"
	"testing"
)

func TestTableIsSortedAndUnique(t *testing.T) {
	entries := Table()
	if len(entries) == 0 {
		t.Fatal("Table() is empty")
	}

	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.Name
	}
	if !sort.StringsAreSorted(names) {
		t.Errorf("Table() is not sorted by Name; got %v", names)
	}

	seen := make(map[string]bool, len(names))
	for _, name := range names {
		if seen[name] {
			t.Errorf("duplicate entry for construct %q", name)
		}
		seen[name] = true
	}
}

func TestTableIsDeterministic(t *testing.T) {
	first := Table()
	second := Table()
	if len(first) != len(second) {
		t.Fatalf("Table() returned %d then %d entries", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Errorf("Table()[%d] differs between calls: %+v vs %+v", i, first[i], second[i])
		}
	}
}

func TestTableReturnsACopy(t *testing.T) {
	entries := Table()
	original := entries[0]
	entries[0].Name = "mutated"
	if Table()[0] != original {
		t.Error("mutating Table()'s result changed the registry")
	}
}

func TestEveryEntryHasEvidence(t *testing.T) {
	for _, entry := range Table() {
		if entry.Note == "" {
			t.Errorf("construct %q has no Note explaining its classification", entry.Name)
		}
		if entry.Support < Supported || entry.Support > Unsupported {
			t.Errorf("construct %q has an out-of-range Support value %d", entry.Name, entry.Support)
		}
	}
}

// TestUnsupportedConstructsWithOwnersCiteThem keeps the diagnostic actionable:
// where a backlog item exists to close a gap, the registry must name it so the
// error message can point at it.
func TestUnsupportedConstructsWithOwnersCiteThem(t *testing.T) {
	expected := map[string]string{
		"test": "improvements.md #96",
	}
	for name, tracker := range expected {
		entry, ok := Lookup(name)
		if !ok {
			t.Errorf("construct %q is missing from the registry", name)
			continue
		}
		if entry.Support != Unsupported {
			t.Errorf("construct %q = %v, want Unsupported", name, entry.Support)
		}
		if entry.Tracker != tracker {
			t.Errorf("construct %q Tracker = %q, want %q", name, entry.Tracker, tracker)
		}
	}
}

// TestModuleConstructsAreCompileTimeOnly locks in improvements.md #95's
// classification: parser.ExpandIncludes and ast.ResolveModules fully consume
// use/export/module before checker.Check or the ZIR gate ever run
// (zero.go:74-75), so the bytecode target never needs a lowering for them.
func TestModuleConstructsAreCompileTimeOnly(t *testing.T) {
	for _, name := range []string{"use", "export", "module"} {
		entry, ok := Lookup(name)
		if !ok {
			t.Errorf("construct %q is missing from the registry", name)
			continue
		}
		if entry.Support != CompileTimeOnly {
			t.Errorf("construct %q = %v, want CompileTimeOnly", name, entry.Support)
		}
	}
}

func TestOnlyUnsupportedEntriesCarryTrackers(t *testing.T) {
	for _, entry := range Table() {
		if entry.Tracker != "" && entry.Support != Unsupported {
			t.Errorf("construct %q is %v but cites tracker %q; only Unsupported gaps have owners",
				entry.Name, entry.Support, entry.Tracker)
		}
	}
}

func TestLookupUnknownConstruct(t *testing.T) {
	if _, ok := Lookup("totally_made_up_head"); ok {
		t.Error("Lookup found an entry for an invented construct")
	}
}

func TestSubFormsAreSortedAndNotTableEntries(t *testing.T) {
	subs := SubForms()
	if len(subs) == 0 {
		t.Fatal("SubForms() is empty")
	}
	for i := 1; i < len(subs); i++ {
		prev, cur := subs[i-1], subs[i]
		if prev.Head > cur.Head || (prev.Head == cur.Head && prev.Parent > cur.Parent) {
			t.Errorf("SubForms() is not sorted at index %d: %+v then %+v", i, prev, cur)
		}
	}

	// A sub-form's parent must itself be something the scan already knows
	// about - either a registry entry, or another sub-form (a lambda's
	// parent is route, which is itself a sub-form of http_server). An
	// exemption anchored to an unknown parent could never be reached.
	subHeads := make(map[string]bool, len(subs))
	for _, sub := range subs {
		subHeads[sub.Head] = true
	}
	for _, sub := range subs {
		if _, ok := Lookup(sub.Parent); ok {
			continue
		}
		if subHeads[sub.Parent] {
			continue
		}
		t.Errorf("sub-form %q names parent %q, which is neither a registry entry nor a sub-form", sub.Head, sub.Parent)
	}

	// catch, route and lambda must NOT be registry entries: they are only
	// legal under their owning parent. Registering them would make a bare
	// (catch ...) silently acceptable.
	for _, head := range []string{"catch", "route", "lambda", "metric", "candidate", "column"} {
		if _, ok := Lookup(head); ok {
			t.Errorf("sub-form %q must not be a standalone registry entry", head)
		}
	}
}

func TestSupportedNamesMatchesTable(t *testing.T) {
	names := SupportedNames()
	if !sort.StringsAreSorted(names) {
		t.Errorf("SupportedNames() is not sorted: %v", names)
	}
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	for _, entry := range Table() {
		if entry.Support == Supported && !set[entry.Name] {
			t.Errorf("SupportedNames() is missing %q", entry.Name)
		}
		if entry.Support != Supported && set[entry.Name] {
			t.Errorf("SupportedNames() wrongly includes %v construct %q", entry.Support, entry.Name)
		}
	}
}

func TestSupportString(t *testing.T) {
	cases := map[Support]string{
		Supported:       "Supported",
		CompileTimeOnly: "CompileTimeOnly",
		Unsupported:     "Unsupported",
	}
	for support, want := range cases {
		if got := support.String(); got != want {
			t.Errorf("Support(%d).String() = %q, want %q", support, got, want)
		}
	}
}

func TestViolationMessageCitesTracker(t *testing.T) {
	withTracker := Violation{Name: "test", Tracker: "improvements.md #96", Reason: "no VM support"}
	msg := withTracker.Message()
	for _, want := range []string{`"test"`, "no VM support", "improvements.md #96"} {
		if !strings.Contains(msg, want) {
			t.Errorf("Message() = %q, missing %q", msg, want)
		}
	}

	withoutTracker := Violation{Name: "trace", Reason: "no opcode"}
	if strings.Contains(withoutTracker.Message(), "tracked by") {
		t.Errorf("Message() = %q, should not claim an owner when Tracker is empty", withoutTracker.Message())
	}
}
