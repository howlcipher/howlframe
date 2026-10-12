//go:build ignore

package main

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/load.txt")
	if err != nil {
		panic(err)
	}
	var a []int
	for _, x := range strings.Split(strings.TrimSpace(string(data)), ",") {
		v, err := strconv.Atoi(x)
		if err != nil {
			panic(err)
		}
		a = append(a, v)
	}
	out := []int{}
	for i := 0; i+3 <= len(a); i++ {
		out = append(out, (a[i]+a[i+1]+a[i+2])/3)
	}
	enc, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(enc, '\n'), 0o644); err != nil {
		panic(err)
	}
}
