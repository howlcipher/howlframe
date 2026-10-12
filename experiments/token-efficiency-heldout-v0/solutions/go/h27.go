//go:build ignore

package main

import (
	"encoding/json"
	"os"
)

func main() {
	data, err := os.ReadFile("in/profile.json")
	if err != nil {
		panic(err)
	}
	var profile struct {
		User struct {
			Name string   `json:"name"`
			Tags []string `json:"tags"`
		} `json:"user"`
	}
	if err := json.Unmarshal(data, &profile); err != nil {
		panic(err)
	}
	result := map[string]any{"name": profile.User.Name, "tag_count": len(profile.User.Tags)}
	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(out, '\n'), 0o644); err != nil {
		panic(err)
	}
}
