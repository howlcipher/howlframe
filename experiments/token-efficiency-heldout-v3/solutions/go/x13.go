//go:build ignore

package main

import (
	"encoding/json"
	"os"
	"strings"
)

func read(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func writeJSON(v any) {
	enc, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(enc, '\n'), 0o644); err != nil {
		panic(err)
	}
}

func main() {
	out := map[string]int{}
	for _, line := range strings.Split(read("in/requests.jsonl"), "\n") {
		var r struct {
			Level string
			Ms    int
		}
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			panic(err)
		}
		if cur, ok := out[r.Level]; !ok || r.Ms > cur {
			out[r.Level] = r.Ms
		}
	}
	writeJSON(out)
}
