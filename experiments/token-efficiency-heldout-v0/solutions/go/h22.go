//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/people.json")
	if err != nil {
		panic(err)
	}
	var people []struct {
		ID  string `json:"id"`
		Age int    `json:"age"`
	}
	if err := json.Unmarshal(data, &people); err != nil {
		panic(err)
	}
	var lines []string
	for _, p := range people {
		status := "invalid"
		if p.Age >= 0 && p.Age <= 150 {
			status = "valid"
		}
		lines = append(lines, fmt.Sprintf("%s %s", p.ID, status))
	}
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
