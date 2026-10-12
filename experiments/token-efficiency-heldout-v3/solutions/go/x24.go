//go:build ignore

package main

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

func read(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func writeJSON(v any) {
	enc, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(enc, '\n'), 0o644); err != nil {
		panic(err)
	}
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func main() {
	g, l := 0, 1
	for _, s := range strings.Split(read("in/vals.txt"), ",") {
		n, err := strconv.Atoi(s)
		if err != nil {
			panic(err)
		}
		g = gcd(g, n)
		l = l / gcd(l, n) * n
	}
	writeJSON(map[string]int{"gcd": g, "lcm": l})
}
