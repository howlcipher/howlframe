package fetch_cli_test

import (
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
)

func TestFetchCLI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte(`{"ok":"1"}`))
	}))
	defer srv.Close()

	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "fetch_cli.howl")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile: %v", err)
	}

	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "network:fetch="+strings.TrimPrefix(srv.URL, "http://"),
		"fetch_cli.howl.bc.bin", srv.URL)
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fetch: %v %s", err, out)
	}
	got := string(out)
	if !strings.Contains(got, "status=418") || !strings.Contains(got, `"ok":"1"`) {
		t.Fatalf("unexpected: %s", got)
	}

	// #165: the coarse network grant no longer authorizes outbound fetch.
	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "network",
		"fetch_cli.howl.bc.bin", srv.URL)
	cmd.Dir = "."
	out, _ = cmd.CombinedOutput()
	if !strings.Contains(string(out), "capability denied: network") {
		t.Fatalf("want denial, got %s", out)
	}
}
