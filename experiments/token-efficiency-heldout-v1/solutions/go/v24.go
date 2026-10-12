//go:build ignore

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/version.txt")
	if err != nil {
		panic(err)
	}
	parts := strings.Split(strings.TrimSpace(string(data)), ".")
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.txt", []byte(fmt.Sprintf("%s.%d.0", parts[0], minor+1)+"\n"), 0o644); err != nil {
		panic(err)
	}
}
