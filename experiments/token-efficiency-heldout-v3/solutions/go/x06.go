//go:build ignore

package main

import (
	"encoding/json"
	"os"
	"slices"
	"sort"
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
	var sales []struct {
		Region, Product string
		Units           int
	}
	readJSON("in/sales.json", &sales)
	type region struct {
		Units    int      `json:"units"`
		Products []string `json:"products"`
	}
	out := map[string]*region{}
	for _, s := range sales {
		r, ok := out[s.Region]
		if !ok {
			r = &region{Products: []string{}}
			out[s.Region] = r
		}
		r.Units += s.Units
		if !slices.Contains(r.Products, s.Product) {
			r.Products = append(r.Products, s.Product)
		}
	}
	for _, r := range out {
		sort.Strings(r.Products)
	}
	writeJSON(out)
}
