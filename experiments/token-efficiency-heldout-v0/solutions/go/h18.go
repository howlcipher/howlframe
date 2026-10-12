//go:build ignore

package main

import (
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/emails.txt")
	if err != nil {
		panic(err)
	}
	lines := strings.Split(string(data), "\n")
	var domains []string
	for _, email := range lines {
		domains = append(domains, strings.Split(email, "@")[1])
	}
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(domains, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
