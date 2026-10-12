//go:build ignore

package main

import (
	"encoding/json"
	"os"
)

func main() {
	data, err := os.ReadFile("in/members.json")
	if err != nil {
		panic(err)
	}
	var members []struct {
		Team   string `json:"team"`
		Member string `json:"member"`
	}
	if err := json.Unmarshal(data, &members); err != nil {
		panic(err)
	}
	teams := map[string][]string{}
	for _, m := range members {
		teams[m.Team] = append(teams[m.Team], m.Member)
	}
	out, err := json.MarshalIndent(teams, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(out, '\n'), 0o644); err != nil {
		panic(err)
	}
}
