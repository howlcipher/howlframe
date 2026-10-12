//go:build ignore

package main

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/ledger.csv")
	if err != nil {
		panic(err)
	}
	out := map[string]int{}
	for _, line := range strings.Split(string(data), "\n")[1:] {
		r := strings.Split(line, ",")
		amount, err := strconv.Atoi(r[2])
		if err != nil {
			panic(err)
		}
		out[r[1]] += amount
	}
	enc, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(enc, '\n'), 0o644); err != nil {
		panic(err)
	}
}
