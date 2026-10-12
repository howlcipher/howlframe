//go:build ignore

package main

import (
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

func main() {
	var out []string
	for _, item := range strings.Split(read("in/ranges.txt"), ",") {
		a, b, ok := strings.Cut(item, "-")
		if !ok {
			b = a
		}
		lo, err1 := strconv.Atoi(a)
		hi, err2 := strconv.Atoi(b)
		if err1 != nil || err2 != nil {
			panic(item)
		}
		for i := lo; i <= hi; i++ {
			out = append(out, strconv.Itoa(i))
		}
	}
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(out, ",")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
