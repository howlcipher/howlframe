//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
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

func writeJSON(v any) {
	enc, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(enc, '\n'), 0o644); err != nil {
		panic(err)
	}
}

func main() {
	var prices map[string]int
	readJSON("in/prices.json", &prices)
	out := map[string]string{}
	for k, v := range prices {
		out[k] = fmt.Sprintf("$%d.%02d", v/100, v%100)
	}
	writeJSON(out)
}
