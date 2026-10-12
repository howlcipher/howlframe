//go:build ignore

package main

import (
	"encoding/json"
	"os"
)

func main() {
	data, err := os.ReadFile("in/items.json")
	if err != nil {
		panic(err)
	}
	var items []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &items); err != nil {
		panic(err)
	}
	byID := map[string]string{}
	for _, item := range items {
		byID[item.ID] = item.Name
	}
	out, err := json.MarshalIndent(byID, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(out, '\n'), 0o644); err != nil {
		panic(err)
	}
}
