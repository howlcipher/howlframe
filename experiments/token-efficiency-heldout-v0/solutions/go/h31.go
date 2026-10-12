//go:build ignore

package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func main() {
	count := 0
	err := filepath.WalkDir("in/docs", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			count++
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.txt", []byte(fmt.Sprintf("%d\n", count)), 0o644); err != nil {
		panic(err)
	}
}
