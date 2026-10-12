//go:build ignore

package main

import (
	"encoding/json"
	"os"
)

func main() {
	data, err := os.ReadFile("in/jobs.json")
	if err != nil {
		panic(err)
	}
	var jobs []string
	if err := json.Unmarshal(data, &jobs); err != nil {
		panic(err)
	}
	out := [][]string{}
	for i := 0; i < len(jobs); i += 3 {
		out = append(out, jobs[i:min(i+3, len(jobs))])
	}
	enc, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(enc, '\n'), 0o644); err != nil {
		panic(err)
	}
}
