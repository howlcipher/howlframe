//go:build ignore

package main

import (
	"encoding/json"
	"os"
	"strings"
)

func readJSON(path string, v any) {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		panic(err)
	}
}

func writeLines(lines []string) {
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}

func main() {
	var accounts []struct {
		Login string
		User  struct {
			Profile struct{ City *string }
		}
	}
	readJSON("in/accounts.json", &accounts)
	var out []string
	for _, a := range accounts {
		city := "unknown"
		if a.User.Profile.City != nil {
			city = *a.User.Profile.City
		}
		out = append(out, a.Login+": "+city)
	}
	writeLines(out)
}
