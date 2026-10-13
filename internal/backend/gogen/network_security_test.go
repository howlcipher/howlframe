package gogen

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGoFetchSecurity(t *testing.T) {
	// Compile the exact helper emitted by the authoritative generator into a
	// temporary consumer package, and run the same socket tests as core.
	dir, err := os.MkdirTemp(filepath.Join(moduleRoot(t), "internal"), ".netscope-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	source := `package consumer
import (
 "context"; "errors"; "fmt"; "io"; "net"; "net/http"; "net/netip"; "net/url"; "os"; "os/exec"; "strconv"; "strings"; "syscall"; "time"; "testing"
 "github.com/howlcipher/howlframe/internal/nettest"
)
` + fetchHelperSource() + execHelperSource() + `
func TestSecurity(t *testing.T) {
 nettest.Run(t,func(grants ...string)*http.Client {
  t.Setenv("HOWLFRAME_ALLOW_CAPS",strings.Join(grants,","))
  c:=*howlFrameFetchClient
  c.Transport=howlFrameFetchClient.Transport.(*http.Transport).Clone()
  return &c
 },func(f nettest.Lookup)func(){
  old:=howlFrameFetchLookupIP;howlFrameFetchLookupIP=f;return func(){howlFrameFetchLookupIP=old}
 })
}
func TestCodeLoadingEnv(t *testing.T) {
 for _,name:=range []string{"PYTHONPATH","NODE_OPTIONS","BASH_ENV","ENV","PERL5LIB","PERL5OPT","RUBYOPT","RUBYLIB","PYTHONSTARTUP","NODE_PATH"} {
  for _,raw:=range []string{name,strings.ToLower(name)} {
   t.Setenv(raw,"injected")
   t.Setenv("HOWLFRAME_ALLOW_CAPS","process:env="+raw)
   if env:=howlFrameExecEnv();len(env)!=0 {t.Fatalf("code-loading variable passed: %v",env)}
  }
 }
}
`
	if err := os.WriteFile(filepath.Join(dir, "security_test.go"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"test", "-count=1", "-v", dir}
	// Optional filter is used by the mutation runner to select one assertion.
	if filter := os.Getenv("HF_NET_MUTATION_FILTER"); filter != "" {
		args = append(args, "-run", filter)
	}
	cmd := exec.Command("go", args...)
	cmd.Dir = moduleRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("emitted helper conformance: %v\n%s", err, out)
	}
	t.Log(string(out))
}
