//go:build ignore

package main

import (
	"encoding/json"
	"os"
)

func main() {
	data, err := os.ReadFile("in/codes.json")
	if err != nil {
		panic(err)
	}
	var codes map[string]string
	if err := json.Unmarshal(data, &codes); err != nil {
		panic(err)
	}
	inverted := map[string]string{}
	for key, value := range codes {
		inverted[value] = key
	}
	out, err := json.MarshalIndent(inverted, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(out, '\n'), 0o644); err != nil {
		panic(err)
	}
}
