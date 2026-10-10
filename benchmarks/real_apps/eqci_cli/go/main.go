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
	if len(args) < 2 || args[0] == "" || args[1] == "" {
		fmt.Println("Usage: eqci_cli <a> <b>")
		return 2
	}
	if strings.EqualFold(args[0], args[1]) {
		fmt.Println("YES")
		return 0
	}
	fmt.Println("NO")
	return 1
}
