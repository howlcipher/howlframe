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
	if len(args) < 1 || args[0] == "" {
		fmt.Println("Usage: trim_cli <text>")
		return 2
	}
	fmt.Println(strings.TrimSpace(args[0]))
	return 0
}
