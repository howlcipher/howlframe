package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) < 1 || args[0] == "" {
		fmt.Println("Usage: fetch_cli <url> [method]")
		return 2
	}
	method := "GET"
	if len(args) > 1 && args[1] != "" {
		method = args[1]
	}
	req, err := http.NewRequest(method, args[0], nil)
	if err != nil {
		fmt.Println(err)
		return 2
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println(err)
		return 2
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	fmt.Printf("status=%d\n", resp.StatusCode)
	fmt.Print(string(b))
	return 0
}
