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
		fmt.Println("Usage: lower_cli <text>")
		return 2
	}
	fmt.Println(strings.ToLower(args[0]))
	return 0
}
