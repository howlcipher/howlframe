//go:build ignore

package main

import (
	"os"
	"strconv"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/n.txt")
	if err != nil {
		panic(err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		panic(err)
	}
	var out []string
	for i := 1; i <= n; i++ {
		switch {
		case i%15 == 0:
			out = append(out, "FizzBuzz")
		case i%3 == 0:
			out = append(out, "Fizz")
		case i%5 == 0:
			out = append(out, "Buzz")
		default:
			out = append(out, strconv.Itoa(i))
		}
	}
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(out, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
