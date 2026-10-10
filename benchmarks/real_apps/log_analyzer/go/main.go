package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

func main() {
	os.Exit(run(os.Args))
}

func run(argv []string) int {
	if len(argv) < 2 || argv[1] == "" {
		fmt.Println("HowlFrame Log Analyzer")
		fmt.Println("Usage: log_analyzer <log-file>")
		return 2
	}
	logFile := argv[1]
	content, err := os.ReadFile(logFile)
	if err != nil {
		fmt.Println("HowlFrame Log Analyzer")
		fmt.Println("file=", logFile)
		fmt.Println("error=", err)
		return 2
	}
	lines := strings.Split(string(content), "\n")
	errors, warnings, todos := 0, 0, 0
	reErr := regexp.MustCompile(`ERROR|FATAL`)
	reWarn := regexp.MustCompile(`WARN|WARNING`)
	reTodo := regexp.MustCompile(`TODO|FIXME`)
	for _, line := range lines {
		if reErr.MatchString(line) {
			errors++
		} else if reWarn.MatchString(line) {
			warnings++
		} else if reTodo.MatchString(line) {
			todos++
		}
	}
	fmt.Println("HowlFrame Log Analyzer")
	fmt.Println("file=", logFile)
	fmt.Println("lines=", len(lines))
	fmt.Println("errors=", errors)
	fmt.Println("warnings=", warnings)
	fmt.Println("todo=", todos)
	if errors > 0 {
		fmt.Println("status=ATTENTION")
		return 1
	}
	fmt.Println("status=CLEAN")
	return 0
}
