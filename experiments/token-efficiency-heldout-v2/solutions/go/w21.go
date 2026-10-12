//go:build ignore

package main

import (
	"encoding/json"
	"os"
)

func main() {
	data, err := os.ReadFile("in/staff.json")
	if err != nil {
		panic(err)
	}
	var staff []struct{ Name, Manager string }
	if err := json.Unmarshal(data, &staff); err != nil {
		panic(err)
	}
	out := map[string][]string{}
	for _, s := range staff {
		if s.Manager != "" {
			out[s.Manager] = append(out[s.Manager], s.Name)
		}
	}
	enc, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(enc, '\n'), 0o644); err != nil {
		panic(err)
	}
}
