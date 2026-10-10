package main

import (
	"fmt"
	"os"
	"strconv"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) < 2 || args[0] == "" || args[1] == "" {
		fmt.Println("Usage: minmax_cli <a> <b>")
		return 2
	}
	a, err1 := strconv.Atoi(args[0])
	b, err2 := strconv.Atoi(args[1])
	if err1 != nil || err2 != nil {
		fmt.Println("Usage: minmax_cli <a> <b>")
		return 2
	}
	lo, hi := a, b
	if b < a {
		lo, hi = b, a
	}
	fmt.Printf("min=%d\n", lo)
	fmt.Printf("max=%d\n", hi)
	return 0
}
