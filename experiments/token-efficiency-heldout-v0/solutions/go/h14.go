//go:build ignore

package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/app.log")
	if err != nil {
		panic(err)
	}
	lines := strings.Split(string(data), "\n")
	count := 0
	for _, line := range lines {
		if strings.Contains(line, "ERROR") {
			count++
		}
	}
	if err := os.WriteFile("out/result.txt", []byte(fmt.Sprintf("%d\n", count)), 0o644); err != nil {
		panic(err)
	}
}
