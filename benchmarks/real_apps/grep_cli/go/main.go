package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) < 2 || args[0] == "" || args[1] == "" {
		fmt.Println("Usage: grep_cli <pattern> <file>")
		return 2
	}
	pat, path := args[0], args[1]
	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("error=%v\n", err)
		return 2
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		fmt.Printf("error=%v\n", err)
		return 2
	}
	rows := strings.Split(string(b), "\n")
	if len(rows) > 0 && rows[len(rows)-1] == "" {
		rows = rows[:len(rows)-1]
	}
	n := 0
	for _, row := range rows {
		if re.MatchString(row) {
			n++
			fmt.Println(row)
		}
	}
	fmt.Printf("hits=%d\n", n)
	if n > 0 {
		return 0
	}
	return 1
}
