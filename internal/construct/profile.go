package construct

import "fmt"

type Profile string

const (
	ProfileDefault  Profile = "default"
	ProfileGoverned Profile = "governed"
)

type Exclusion struct {
	Name   string
	Reason string
}

// GovernedExcluded lists unconditional construct exclusions in registry order.
// Conditional include/use root confinement and Go/JS target exclusions are
// checked by hfir.VerifyProfile; include itself remains allowed inside the root.
// spawn and spawn_agent are distinct
// primitives; both are excluded to prevent agent execution bypassing spawn policy.
var GovernedExcluded = []Exclusion{
	{"achieve", "model execution"}, {"confidence", "model execution"},
	{"db_connect", "external SQL connection"}, {"ephemeral_circuit", "model execution"},
	{"exec", "host process execution"}, {"lazy_synthesize", "runtime code synthesis"},
	{"llm_generate", "model execution"}, {"neural_circuit", "model execution"},
	{"spawn", "host process execution"}, {"spawn_agent", "agent execution"}, {"sql_query", "external SQL query"},
}

func LookupProfile(name string) (Profile, error) {
	switch Profile(name) {
	case ProfileDefault, ProfileGoverned:
		return Profile(name), nil
	}
	return "", fmt.Errorf("unknown profile %q", name)
}
func Allowed(profile Profile, name string) bool {
	if profile == ProfileDefault {
		return true
	}
	if profile != ProfileGoverned {
		return false
	}
	for _, e := range GovernedExcluded {
		if e.Name == name {
			return false
		}
	}
	e, ok := Lookup(name)
	return ok && e.Support != Unsupported
}
