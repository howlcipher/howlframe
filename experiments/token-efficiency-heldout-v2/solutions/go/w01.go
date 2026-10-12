//go:build ignore

package main

import (
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/para.txt")
	if err != nil {
		panic(err)
	}
	var out []string
	cur := ""
	for _, word := range strings.Fields(string(data)) {
		if cur == "" {
			cur = word
		} else if len(cur)+1+len(word) <= 20 {
			cur += " " + word
		} else {
			out = append(out, cur)
			cur = word
		}
	}
	out = append(out, cur)
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(out, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
