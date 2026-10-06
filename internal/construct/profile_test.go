package construct

import "testing"

func TestProfilesRegistryContract(t *testing.T) {
	excluded := map[string]bool{}
	previous := ""
	for _, e := range GovernedExcluded {
		if e.Name <= previous || e.Reason == "" {
			t.Fatalf("exclusions must be sorted, unique and explained: %+v", e)
		}
		previous = e.Name
		excluded[e.Name] = true
		if _, ok := Lookup(e.Name); !ok {
			t.Fatalf("excluded construct %q is absent from registry", e.Name)
		}
	}
	for _, e := range Table() {
		if !Allowed(ProfileDefault, e.Name) {
			t.Fatalf("default rejects %s", e.Name)
		}
		want := e.Support != Unsupported && !excluded[e.Name]
		if Allowed(ProfileGoverned, e.Name) != want {
			t.Fatalf("governed classification mismatch: %s", e.Name)
		}
	}
	for _, name := range []string{"default", "governed"} {
		if p, err := LookupProfile(name); err != nil || string(p) != name {
			t.Fatalf("%s: %s %v", name, p, err)
		}
	}
	if _, err := LookupProfile("unknown"); err == nil {
		t.Fatal("unknown profile accepted")
	}
}
