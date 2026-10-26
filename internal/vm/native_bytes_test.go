package vm

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
)

// nativeBytesOps applies every list operation to the value bound to b. The
// same operations run on read_file bytes and on the equivalent literal number
// list, so a byte list behaves exactly as the list it stands for.
const nativeBytesOps = `(do
  (print (list_len b))
  (print (list_get b 1))
  (print (list_get b 99))
  (for x b (print x))
  (print (str_join b "-"))
  (print (bytes_to_string b))
  (print (encode_json b))
  (print (encode_json (dict ("body" b))))
  (print b)
  (print (== b b))
  (print (!= b (list 1)))
  (store_open db "memory://native-bytes")
  (store_put db "k" (dict ("body" b)))
  (print (encode_json (store_get db "k")))
  (let (c b) (do (append c "z") (print c) (print (list_len b)))))`

func runNativeBytesProgram(t *testing.T, source string, policy ExecutionPolicy) (string, *bytecode.RuntimeFailure) {
	t.Helper()
	_, prog := parseAndCompile(t, source)
	var out, stderr bytes.Buffer
	caps := []capability.Capability{capability.Filesystem, capability.Database}
	ev := RunBytecodeWithEvidence(prog, nil, policy, caps, nil, &out, &stderr, 0)
	if ev.ExitCode != 0 && ev.RuntimeFailure == nil {
		t.Fatalf("exit %d: %s", ev.ExitCode, &stderr)
	}
	return out.String(), ev.RuntimeFailure
}

func TestNativeBytesBehaveLikeTheirNumberList(t *testing.T) {
	data := []byte{104, 105, 0, 255, 10}
	path := filepath.Join(t.TempDir(), "data.bin")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	literal := make([]string, len(data))
	for i, c := range data {
		literal[i] = fmt.Sprint(c)
	}
	fromFile, failure := runNativeBytesProgram(t, fmt.Sprintf(`(cli_app (let (b (read_file %q)) %s))`, path, nativeBytesOps), DefaultExecutionPolicy())
	if failure != nil {
		t.Fatalf("read_file bytes: %#v", failure)
	}
	fromList, failure := runNativeBytesProgram(t, fmt.Sprintf(`(cli_app (let (b (list %s)) %s))`, strings.Join(literal, " "), nativeBytesOps), DefaultExecutionPolicy())
	if failure != nil {
		t.Fatalf("number list: %#v", failure)
	}
	if fromFile != fromList {
		t.Fatalf("read_file bytes differ from their number list:\nbytes: %q\nlist:  %q", fromFile, fromList)
	}
	for _, want := range []string{"[104,105,0,255,10]", `{"body":[104,105,0,255,10]}`, "[104 105 0 255 10 z]\n5\n"} {
		if !strings.Contains(fromFile, want) {
			t.Fatalf("output lacks %q:\n%s", want, fromFile)
		}
	}
}

// Bytes cost one byte each until an operation needs them as list items; then
// those items are charged as list slots, as before. The budget leaves room for
// the 1000-byte string bytes_to_string makes, and was 9000 bytes short of the
// read alone when bytes were charged as list slots.
func TestNativeBytesMemoryCharge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.bin")
	if err := os.WriteFile(path, bytes.Repeat([]byte{'a'}, 1000), 0o600); err != nil {
		t.Fatal(err)
	}
	policy := DefaultExecutionPolicy()
	policy.Limits.MaxMemoryBytes = 2100
	for _, body := range []string{
		`(print (list_len b))`,
		`(print (bytes_to_string b))`,
		`(for x b (set n (+ n 1)))`,
		`(print (list_get b 999))`,
	} {
		source := fmt.Sprintf(`(cli_app (let (b (read_file %q) n 0) %s))`, path, body)
		if _, failure := runNativeBytesProgram(t, source, policy); failure != nil {
			t.Fatalf("%s within 2100 bytes: %#v", body, failure)
		}
	}
	for _, body := range []string{`(print (str_join b ""))`, `(append b 1)`} {
		source := fmt.Sprintf(`(cli_app (let (b (read_file %q)) %s))`, path, body)
		if _, failure := runNativeBytesProgram(t, source, policy); failure == nil || failure.Code != "LIMIT_EXCEEDED" {
			t.Fatalf("%s must charge the list it builds: %#v", body, failure)
		}
	}
}

// exec with a single byte-list argument passes it as the argument vector, as
// it does for any list.
func TestExecArgvFromNativeBytes(t *testing.T) {
	if got := execArgv([]any{[]byte{49, 50}}); len(got) != 2 || got[0] != "49" || got[1] != "50" {
		t.Fatalf("execArgv(bytes) = %q", got)
	}
}
