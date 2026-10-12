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
	var stock map[string]int
	var moves []struct {
		SKU   string
		Delta int
	}
	readJSON("in/stock.json", &stock)
	readJSON("in/moves.json", &moves)
	for _, m := range moves {
		stock[m.SKU] += m.Delta
	}
	enc, err := json.MarshalIndent(stock, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(enc, '\n'), 0o644); err != nil {
		panic(err)
	}
}
