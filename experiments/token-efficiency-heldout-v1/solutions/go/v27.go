//go:build ignore

package main

import (
	"encoding/json"
	"os"
	"strings"
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
	var ids []string
	for _, order := range orders {
		if order.Total >= 100 {
			ids = append(ids, order.ID)
		}
	}
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(ids, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
