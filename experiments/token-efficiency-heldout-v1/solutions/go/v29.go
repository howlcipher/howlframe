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
	cheapest := orders[0]
	for _, order := range orders[1:] {
		if order.Total < cheapest.Total {
			cheapest = order
		}
	}
	if err := os.WriteFile("out/result.txt", []byte(cheapest.ID+"\n"), 0o644); err != nil {
		panic(err)
	}
}
