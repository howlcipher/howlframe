//go:build ignore

package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/cmds.txt")
	if err != nil {
		panic(err)
	}
	moves := map[[2]string]string{
		{"closed", "open"}: "opened", {"opened", "close"}: "closed",
		{"closed", "lock"}: "locked", {"locked", "unlock"}: "closed",
	}
	state := "closed"
	result := ""
	for i, cmd := range strings.Split(string(data), "\n") {
		next, ok := moves[[2]string{state, cmd}]
		if !ok {
			result = fmt.Sprintf("error line %d: cannot %s when %s", i+1, cmd, state)
			break
		}
		state = next
	}
	if result == "" {
		result = state
	}
	if err := os.WriteFile("out/result.txt", []byte(result+"\n"), 0o644); err != nil {
		panic(err)
	}
}
