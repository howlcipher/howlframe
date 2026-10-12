//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/prices.json")
	if err != nil {
		panic(err)
	}
	var prices []int
	if err := json.Unmarshal(data, &prices); err != nil {
		panic(err)
	}
	td, err := os.ReadFile("in/target.txt")
	if err != nil {
		panic(err)
	}
	target, err := strconv.Atoi(strings.TrimSpace(string(td)))
	if err != nil {
		panic(err)
	}
	result := "none"
search:
	for j := 1; j < len(prices); j++ {
		for i := 0; i < j; i++ {
			if prices[i]+prices[j] == target {
				result = fmt.Sprintf("%d %d", i, j)
				break search
			}
		}
	}
	if err := os.WriteFile("out/result.txt", []byte(result+"\n"), 0o644); err != nil {
		panic(err)
	}
}
