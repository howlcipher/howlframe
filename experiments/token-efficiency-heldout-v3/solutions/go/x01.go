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

func readJSON(path string, v any) {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		panic(err)
	}
}

func writeLines(lines []string) {
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}

func main() {
	var cfg any
	readJSON("in/config.json", &cfg)
	var out []string
	for _, line := range strings.Split(read("in/paths.txt"), "\n") {
		v, ok := cfg, true
		for _, k := range strings.Split(line, ".") {
			m, isMap := v.(map[string]any)
			if v, ok = m[k]; !isMap || !ok {
				ok = false
				break
			}
		}
		if !ok {
			out = append(out, "missing")
			continue
		}
		enc, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		out = append(out, string(enc))
	}
	writeLines(out)
}
