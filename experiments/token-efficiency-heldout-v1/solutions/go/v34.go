//go:build ignore

package main

import (
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/edges.txt")
	if err != nil {
		panic(err)
	}
	next := map[string]string{}
	targets := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		from, to, _ := strings.Cut(line, ">")
		next[from] = to
		targets[to] = true
	}
	node := ""
	for from := range next {
		if !targets[from] {
			node = from
		}
	}
	chain := []string{node}
	for next[node] != "" {
		node = next[node]
		chain = append(chain, node)
	}
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(chain, ">")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
