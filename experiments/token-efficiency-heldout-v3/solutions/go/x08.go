//go:build ignore

package main

import (
	"encoding/json"
	"os"
)

func readJSON(path string, v any) {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		panic(err)
	}
}

func main() {
	var doc any
	readJSON("in/doc.json", &doc)
	enc, err := json.Marshal(doc) // map keys are marshalled in sorted order
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.txt", append(enc, '\n'), 0o644); err != nil {
		panic(err)
	}
}
