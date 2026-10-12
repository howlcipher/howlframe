//go:build ignore

package main

import (
	"encoding/json"
	"os"
	"slices"
	"strconv"
)

func main() {
	data, err := os.ReadFile("in/queue.json")
	if err != nil {
		panic(err)
	}
	var queue []string
	if err := json.Unmarshal(data, &queue); err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.txt", []byte(strconv.Itoa(slices.Index(queue, "build"))+"\n"), 0o644); err != nil {
		panic(err)
	}
}
