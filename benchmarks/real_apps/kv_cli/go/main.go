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
	store := map[string]string{}
	for i := 0; i < len(args); i++ {
		cmd := args[i]
		switch cmd {
		case "set":
			i++
			k := args[i]
			i++
			v := args[i]
			store[k] = v
			fmt.Println("OK")
		case "get":
			i++
			k := args[i]
			v, ok := store[k]
			if !ok {
				fmt.Println("NOT FOUND")
			} else {
				fmt.Println(v)
			}
		case "delete":
			i++
			k := args[i]
			delete(store, k)
			fmt.Println("OK")
		case "increment":
			i++
			k := args[i]
			current := 0
			if v, ok := store[k]; ok {
				current, _ = strconv.Atoi(v)
			}
			next := current + 1
			store[k] = strconv.Itoa(next)
			fmt.Println(next)
		default:
			fmt.Println("UNKNOWN CMD:", cmd)
			return 1
		}
	}
	return 0
}
