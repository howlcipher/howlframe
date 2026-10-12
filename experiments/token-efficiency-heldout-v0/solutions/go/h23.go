//go:build ignore

package main

import (
	"encoding/json"
	"os"
)

func main() {
	data, err := os.ReadFile("in/ids.json")
	if err != nil {
		panic(err)
	}
	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		panic(err)
	}
	result := "ok"
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			result = "duplicate:" + id
			break
		}
		seen[id] = true
	}
	if err := os.WriteFile("out/result.txt", []byte(result+"\n"), 0o644); err != nil {
		panic(err)
	}
}
