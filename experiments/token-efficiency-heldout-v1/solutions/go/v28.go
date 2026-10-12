//go:build ignore

package main

import (
	"encoding/json"
	"os"
	"slices"
)

func main() {
	data, err := os.ReadFile("in/orders.json")
	if err != nil {
		panic(err)
	}
	var orders []struct {
		ID    string `json:"id"`
		Cust  string `json:"cust"`
		Total int    `json:"total"`
	}
	if err := json.Unmarshal(data, &orders); err != nil {
		panic(err)
	}
	custs := []string{}
	for _, order := range orders {
		if !slices.Contains(custs, order.Cust) {
			custs = append(custs, order.Cust)
		}
	}
	out, err := json.MarshalIndent(custs, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(out, '\n'), 0o644); err != nil {
		panic(err)
	}
}
