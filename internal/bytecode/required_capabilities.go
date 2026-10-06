package bytecode

import (
	"github.com/howlcipher/howlframe/internal/capability"
	"sort"
)

// Report is a deterministic, conservative artifact capability inventory.
type Report struct {
	Capabilities []string `json:"capabilities"`
	Sites        []Site   `json:"sites"`
}

// Site identifies an instruction and explains one reported requirement.
type Site struct {
	Function   string `json:"function"`
	Index      int    `json:"index"`
	Opcode     string `json:"opcode"`
	Capability string `json:"capability"`
	Reason     string `json:"reason"`
}

// RequiredCapabilities conservatively reports effects without executing code.
// Embedded try, agent and route bodies occupy the same instruction slices.
func RequiredCapabilities(prog *BCProgram) Report {
	r := Report{Capabilities: []string{}, Sites: []Site{}}
	if prog == nil {
		return r
	}
	caps := map[string]bool{}
	scan := func(name string, instructions []BCInstruction) {
		for i, inst := range instructions {
			spec := Registry[inst.Op]
			requirements := map[capability.Capability]string{}
			if spec.Capability != capability.None {
				requirements[spec.Capability] = "opcode registry"
			}
			if inst.Op == OpStoreOpen {
				required, valid := capability.StoreRequirements(inst.StringOperand2)
				for _, c := range required {
					requirements[c] = "store URI"
				}
				if !valid {
					requirements[capability.Filesystem] = "unknown store URI; conservatively assumes filesystem"
				}
			}
			if inst.Op == OpCall {
				fn, known := prog.Functions[inst.StringOperand]
				if !known || fn == nil || fn.LazySynthesize {
					// Synthesis executes arbitrary newly compiled instructions, not just a request.
					for _, c := range capability.All() {
						requirements[c] = "lazy or unknown call; synthesized body may require any capability"
					}
				}
			}
			ordered := []string{}
			for c := range requirements {
				ordered = append(ordered, string(c))
			}
			sort.Strings(ordered)
			for _, c := range ordered {
				caps[c] = true
				r.Sites = append(r.Sites, Site{name, i, spec.Name, c, requirements[capability.Capability(c)]})
			}
		}
	}
	// Store handles are private VM values created only by STORE_OPEN in this
	// program (or synthesized code covered by CALL above); aliases cannot create them.
	scan("main", prog.Main)
	names := []string{}
	for name := range prog.Functions {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if fn := prog.Functions[name]; fn != nil {
			scan(name, fn.Instructions)
		}
	}
	for c := range caps {
		r.Capabilities = append(r.Capabilities, c)
	}
	sort.Strings(r.Capabilities)
	return r
}
