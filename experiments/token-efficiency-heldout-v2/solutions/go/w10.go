//go:build ignore

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/seconds.txt")
	if err != nil {
		panic(err)
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		s, err := strconv.Atoi(line)
		if err != nil {
			panic(err)
		}
		out = append(out, fmt.Sprintf("%d:%02d:%02d", s/3600, s%3600/60, s%60))
	}
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(out, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
