//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
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

func main() {
	var users []struct {
		ID   int
		Name string
	}
	var scores []struct{ User, Points int }
	readJSON("in/users.json", &users)
	readJSON("in/scores.json", &scores)
	totals := map[int]int{}
	for _, s := range scores {
		totals[s.User] += s.Points
	}
	var out []string
	for _, u := range users {
		out = append(out, fmt.Sprintf("%s %d", u.Name, totals[u.ID]))
	}
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(out, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
