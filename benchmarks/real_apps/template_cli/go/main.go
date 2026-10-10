package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) < 3 || args[0] == "" || args[1] == "" || args[2] == "" {
		fmt.Println("Usage: template_cli <file> <key> <value>")
		return 2
	}
	b, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Printf("error=%v\n", err)
		return 2
	}
	needle := "{{" + args[1] + "}}"
	parts := strings.Split(string(b), needle)
	if len(parts) < 2 {
		fmt.Println("NO PLACEHOLDER")
		return 1
	}
	fmt.Print(parts[0] + args[2] + parts[1])
	return 0
}
