//go:build ignore

package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	var paths []string
	err := filepath.WalkDir("in/docs", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() && strings.HasSuffix(path, ".txt") {
			rel, err := filepath.Rel("in/docs", path)
			if err != nil {
				return err
			}
			paths = append(paths, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	sort.Strings(paths)
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(paths, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
