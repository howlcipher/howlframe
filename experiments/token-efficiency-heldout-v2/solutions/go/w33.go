//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/board.json")
	if err != nil {
		panic(err)
	}
	var board []struct {
		Name  string
		Score int
	}
	if err := json.Unmarshal(data, &board); err != nil {
		panic(err)
	}
	sort.Slice(board, func(i, j int) bool {
		if board[i].Score != board[j].Score {
			return board[i].Score > board[j].Score
		}
		return board[i].Name < board[j].Name
	})
	var out []string
	rank := 0
	for i, e := range board {
		if i == 0 || e.Score != board[i-1].Score {
			rank++
		}
		out = append(out, fmt.Sprintf("%d %s %d", rank, e.Name, e.Score))
	}
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(out, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
