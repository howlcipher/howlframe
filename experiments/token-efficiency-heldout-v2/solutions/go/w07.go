//go:build ignore

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/lines.csv")
	if err != nil {
		panic(err)
	}
	lines := strings.Split(string(data), "\n")
	out := []string{lines[0] + ",total"}
	for _, line := range lines[1:] {
		r := strings.Split(line, ",")
		qty, err := strconv.Atoi(r[1])
		if err != nil {
			panic(err)
		}
		unit, err := strconv.Atoi(r[2])
		if err != nil {
			panic(err)
		}
		out = append(out, fmt.Sprintf("%s,%d", line, qty*unit))
	}
	if err := os.WriteFile("out/result.csv", []byte(strings.Join(out, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
