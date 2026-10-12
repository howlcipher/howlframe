//go:build ignore

package main

import (
	"encoding/json"
	"os"
)

func main() {
	data, err := os.ReadFile("in/schema.json")
	if err != nil {
		panic(err)
	}
	var schema []string
	if err := json.Unmarshal(data, &schema); err != nil {
		panic(err)
	}
	configData, err := os.ReadFile("in/config.json")
	if err != nil {
		panic(err)
	}
	var config map[string]any
	if err := json.Unmarshal(configData, &config); err != nil {
		panic(err)
	}
	missing := []string{}
	for _, key := range schema {
		if _, ok := config[key]; !ok {
			missing = append(missing, key)
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
