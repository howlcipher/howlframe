//go:build ignore

package main

import (
	"os"
	"strconv"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/years.txt")
	if err != nil {
		panic(err)
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		y, err := strconv.Atoi(line)
		if err != nil {
			panic(err)
		}
		kind := "common"
		if y%4 == 0 && (y%100 != 0 || y%400 == 0) {
			kind = "leap"
		}
		out = append(out, line+" "+kind)
	}
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(out, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
