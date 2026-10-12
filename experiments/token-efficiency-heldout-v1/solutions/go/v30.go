//go:build ignore

package main

import (
	"encoding/json"
	"os"
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
	total := 0
	for _, order := range orders {
		total += order.Total
	}
	out, err := json.MarshalIndent(map[string]int{"count": len(orders), "total": total}, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(out, '\n'), 0o644); err != nil {
		panic(err)
	}
}
