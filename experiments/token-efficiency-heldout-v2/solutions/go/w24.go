//go:build ignore

package main

import (
	"encoding/json"
	"os"
)

func main() {
	data, err := os.ReadFile("in/carts.json")
	if err != nil {
		panic(err)
	}
	var carts []struct {
		Items []struct {
			SKU string
			Qty int
		}
	}
	if err := json.Unmarshal(data, &carts); err != nil {
		panic(err)
	}
	out := map[string]int{}
	for _, c := range carts {
		for _, item := range c.Items {
			out[item.SKU] += item.Qty
		}
	}
	enc, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(enc, '\n'), 0o644); err != nil {
		panic(err)
	}
}
