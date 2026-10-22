package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAdvertisedHFBCFormatMatchesWrittenArtifact(t *testing.T) {
	binaryPath := buildHowlFrameBinaryForTest(t)
	runCLI := func(args ...string) (string, error) {
		out, err := exec.Command(binaryPath, args...).CombinedOutput()
		return string(out), err
	}

	root := t.TempDir()
	source := filepath.Join(root, "format.howl")
	artifact := filepath.Join(root, "format.hfbc")
	if err := os.WriteFile(source, []byte(`(cli_app (print "synthetic"))`), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := runCLI("build", source, "-o", artifact); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	data, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 8 || string(data[:4]) != "HFBC" {
		t.Fatal("missing artifact header")
	}
	format := fmt.Sprint(binary.LittleEndian.Uint32(data[4:8]))
	for _, args := range [][]string{{"--version"}, {"version"}} {
		out, err := runCLI(args...)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "HFBC format: "+format+"\n") {
			t.Fatalf("advertised format does not match written artifact%d: %s", binary.LittleEndian.Uint32(data[4:8]), out)
		}
	}
	out, err := runCLI("interop", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var descriptor struct {
		Format string `json:"hfbc_format"`
	}
	if err := json.Unmarshal([]byte(out), &descriptor); err != nil {
		t.Fatal(err)
	}
	if descriptor.Format != format {
		t.Fatalf("descriptor format %s differs from artifact %s", descriptor.Format, format)
	}
}
