package main

import (
	"flag"
	"testing"

	"github.com/howlcipher/howlframe/internal/vm"
)

func TestStartupInstructionLimitResolution(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want int
	}{
		{"no flags: server default applies", nil, vm.DefaultStartupInstructions},
		{"explicit --max-instructions keeps governing startup", []string{"--max-instructions", "5000"}, 0},
		{"explicit startup flag wins", []string{"--max-startup-instructions", "777"}, 777},
		{"both explicit: startup flag wins", []string{"--max-instructions", "5000", "--max-startup-instructions", "9000"}, 9000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fs := flag.NewFlagSet("run", flag.ContinueOnError)
			fs.Int("max-instructions", 100000, "")
			startup := fs.Int("max-startup-instructions", 0, "")
			if err := fs.Parse(tc.args); err != nil {
				t.Fatal(err)
			}
			if got := startupInstructionLimit(fs, *startup); got != tc.want {
				t.Fatalf("got %d want %d", got, tc.want)
			}
		})
	}
}
