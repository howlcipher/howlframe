package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func path() string {
	uri := os.Getenv("NOTES_STORE")
	if uri == "" {
		uri = "file://notes.store"
	}
	return strings.TrimPrefix(uri, "file://")
}

func load() map[string]string {
	b, err := os.ReadFile(path())
	if err != nil {
		return map[string]string{}
	}
	out := map[string]string{}
	_ = json.Unmarshal(b, &out)
	return out
}

func save(store map[string]string) {
	b, _ := json.Marshal(store)
	_ = os.WriteFile(path(), b, 0o644)
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	store := load()
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "add":
			i++
			title := args[i]
			n := 1
			if v, ok := store["next_id"]; ok {
				n, _ = strconv.Atoi(v)
			}
			id := strconv.Itoa(n)
			store[id] = title
			store["next_id"] = strconv.Itoa(n + 1)
			fmt.Printf("ADDED %s %s\n", id, title)
		case "list":
			for k, v := range store {
				if k != "next_id" {
					fmt.Printf("%s %s\n", k, v)
				}
			}
		case "get":
			i++
			k := args[i]
			v, ok := store[k]
			if !ok {
				fmt.Println("NOT FOUND")
				return 1
			}
			fmt.Printf("%s %s\n", k, v)
		case "delete":
			i++
			k := args[i]
			if _, ok := store[k]; !ok {
				fmt.Println("NOT FOUND")
				return 1
			}
			delete(store, k)
			fmt.Printf("DELETED %s\n", k)
		default:
			fmt.Println("UNKNOWN")
			return 1
		}
	}
	save(store)
	return 0
}
