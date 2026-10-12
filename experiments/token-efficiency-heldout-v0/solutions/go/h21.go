//go:build ignore

package main

import (
	"encoding/json"
	"os"
)

func main() {
	data, err := os.ReadFile("in/users.json")
	if err != nil {
		panic(err)
	}
	var users []map[string]any
	if err := json.Unmarshal(data, &users); err != nil {
		panic(err)
	}
	missing := []any{}
	for _, user := range users {
		if _, ok := user["email"]; !ok {
			missing = append(missing, user["id"])
		}
	}
	out, err := json.MarshalIndent(missing, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(out, '\n'), 0o644); err != nil {
		panic(err)
	}
}
