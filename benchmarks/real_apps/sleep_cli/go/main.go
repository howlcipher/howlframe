package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) < 1 || args[0] == "" {
		fmt.Println("Usage: sleep_cli <ms>")
		return 2
	}
	ms, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Printf("error=%v\n", err)
		return 2
	}
	time.Sleep(time.Duration(ms) * time.Millisecond)
	fmt.Println("OK")
	return 0
}
