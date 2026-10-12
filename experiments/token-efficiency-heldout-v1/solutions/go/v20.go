//go:build ignore

package main

import (
	"encoding/json"
	"os"
)

func main() {
	data, err := os.ReadFile("in/queue.json")
	if err != nil {
		panic(err)
	}
	var queue []string
	if err := json.Unmarshal(data, &queue); err != nil {
		panic(err)
	}
	out, err := json.MarshalIndent(queue[1:len(queue)-1], "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(out, '\n'), 0o644); err != nil {
		panic(err)
	}
}
