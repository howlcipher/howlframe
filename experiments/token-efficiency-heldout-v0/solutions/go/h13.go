//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	data, err := os.ReadFile("in/scores.json")
	if err != nil {
		panic(err)
	}
	var scores []int
	if err := json.Unmarshal(data, &scores); err != nil {
		panic(err)
	}
	total := 0
	for _, s := range scores {
		total += s
	}
	if err := os.WriteFile("out/result.txt", []byte(fmt.Sprintf("%d\n", total/len(scores))), 0o644); err != nil {
		panic(err)
	}
}
