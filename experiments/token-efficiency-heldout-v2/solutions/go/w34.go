//go:build ignore

package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/maze.txt")
	if err != nil {
		panic(err)
	}
	grid := strings.Split(string(data), "\n")
	r, c := 0, 0
	for i, row := range grid {
		if j := strings.IndexByte(row, 'S'); j >= 0 {
			r, c = i, j
		}
	}
	walls := 0
	for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
		y, x := r+d[0], c+d[1]
		if y >= 0 && y < len(grid) && x >= 0 && x < len(grid[y]) && grid[y][x] == '#' {
			walls++
		}
	}
	if err := os.WriteFile("out/result.txt", []byte(fmt.Sprintf("%d,%d %d\n", r, c, walls)), 0o644); err != nil {
		panic(err)
	}
}
