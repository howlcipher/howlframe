//go:build ignore

package main

import (
	"encoding/json"
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/config.kv")
	if err != nil {
		panic(err)
	}
	lines := strings.Split(string(data), "\n")
	config := map[string]string{}
	for _, line := range lines {
		key, value, _ := strings.Cut(line, "=")
		config[key] = value
	}
	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(out, '\n'), 0o644); err != nil {
		panic(err)
	}
}
