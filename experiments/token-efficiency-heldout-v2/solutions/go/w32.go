//go:build ignore

package main

import (
	"os"
	"strings"
)

func luhn(digits string) bool {
	total := 0
	for i := 0; i < len(digits); i++ {
		d := int(digits[len(digits)-1-i] - '0')
		if i%2 == 1 {
			d *= 2
		}
		if d > 9 {
			d -= 9
		}
		total += d
	}
	return total%10 == 0
}

func main() {
	data, err := os.ReadFile("in/ids.txt")
	if err != nil {
		panic(err)
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		if luhn(line) {
			out = append(out, line+" valid")
		} else {
			out = append(out, line+" invalid")
		}
	}
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(out, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
