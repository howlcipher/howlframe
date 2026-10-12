//go:build ignore

package main

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/ring.json")
	if err != nil {
		panic(err)
	}
	var ring []string
	if err := json.Unmarshal(data, &ring); err != nil {
		panic(err)
	}
	kd, err := os.ReadFile("in/k.txt")
	if err != nil {
		panic(err)
	}
	k, err := strconv.Atoi(strings.TrimSpace(string(kd)))
	if err != nil {
		panic(err)
	}
	k %= len(ring)
	out := append(append([]string{}, ring[k:]...), ring[:k]...)
	enc, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(enc, '\n'), 0o644); err != nil {
		panic(err)
	}
}
