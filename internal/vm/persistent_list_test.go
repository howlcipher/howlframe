package vm

import (
	"bytes"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

func TestPersistentListBoundariesAndImmutability(t *testing.T) {
	lengths := []int{0, 1, 2, 15, 16, 17, 31, 32, 33, 255, 256, 257, 272, 273, 513}
	for _, n := range lengths {
		items := make([]any, n)
		seq := emptyVMList
		totalSlots := 0
		for i := 0; i < n; i++ {
			items[i] = int64(i)
			var slots int
			seq, slots = seq.Append(int64(i))
			totalSlots += slots
		}
		built, builtSlots := newVMList(items)
		if seq.Len() != n || built.Len() != n {
			t.Fatalf("len=%d: got seq=%d built=%d", n, seq.Len(), built.Len())
		}
		if n > 0 && totalSlots > 12*n {
			t.Fatalf("len=%d: total allocated slots=%d exceeds O(n) bound %d", n, totalSlots, 12*n)
		}
		if builtSlots < n {
			t.Fatalf("len=%d: builtSlots=%d < %d", n, builtSlots, n)
		}
		for i := 0; i < n; i++ {
			if got := seq.At(i); got != int64(i) {
				t.Fatalf("len=%d seq.At(%d)=%v", n, i, got)
			}
			if got := built.At(i); got != int64(i) {
				t.Fatalf("len=%d built.At(%d)=%v", n, i, got)
			}
		}
		// Ensure mutating original input slice or branching subsequent versions leaves built unchanged.
		if n > 0 {
			items[0] = "mutated"
			if built.At(0) != int64(0) {
				t.Fatalf("len=%d: input slice mutation leaked into vmList", n)
			}
			extended, _ := built.Append("extra")
			if built.Len() != n || extended.Len() != n+1 {
				t.Fatalf("len=%d: branch mutated receiver len", n)
			}
		}
	}
}

func TestListAppendAliasingAndNestedParity(t *testing.T) {
	programs := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "alias stays unchanged",
			src: `(cli_app
  (let (a (list 1 2))
    (let (b a)
      (do
        (append a 3)
        (append a 4)
        (print b)
        (print a)
        (print (list_len b) (list_len a))))))`,
			want: "[1 2]\n[1 2 3 4]\n2 4\n",
		},
		{
			name: "nested list snapshot",
			src: `(cli_app
  (let (snapshots (list) xs (list))
    (let (i 0)
      (do
        (while (< i 20)
          (do
            (append xs i)
            (append snapshots xs)
            (set i (+ i 1))))
        (let (first (list_get snapshots 0) last (list_get snapshots 19))
          (print (list_len first) (list_len last) (list_len xs)))))))`,
			want: "1 20 20\n",
		},
		{
			name: "parse_json list then append",
			src: `(cli_app
  (let (raw "{\"items\":[\"a\",\"b\"]}")
    (let (d (parse_json Doc raw))
      (let (xs (map_get d "items"))
        (do
          (append xs "c")
          (print (encode_json xs))
          (print (list_len (map_get d "items"))))))))`,
			want: "[\"a\",\"b\",\"c\"]\n2\n",
		},
	}

	for _, tc := range programs {
		t.Run(tc.name, func(t *testing.T) {
			root := parser.NewParser(lexer.NewLexer(tc.src), "test.howl").ParseExpression()
			var interpOut, interpErr bytes.Buffer
			if code := Interpret(root, nil, nil, strings.NewReader(""), &interpOut, &interpErr); code != 0 {
				t.Fatalf("interpreter failed (%d): %s", code, interpErr.String())
			}
			if interpOut.String() != tc.want {
				t.Fatalf("interpreter stdout = %q, want %q", interpOut.String(), tc.want)
			}

			_, prog := parseAndCompile(t, tc.src)
			var vmOut, vmErr bytes.Buffer
			ev := RunBytecodeWithEvidence(prog, nil, DefaultExecutionPolicy(), nil, strings.NewReader(""), &vmOut, &vmErr, 0)
			if ev.ExitCode != 0 || ev.RuntimeFailure != nil {
				t.Fatalf("vm failed (exit=%d failure=%#v): %s", ev.ExitCode, ev.RuntimeFailure, vmErr.String())
			}
			if vmOut.String() != tc.want {
				t.Fatalf("vm stdout = %q, want %q", vmOut.String(), tc.want)
			}
		})
	}
}

func TestVMAppend100kWithinDefaultMemoryBudget(t *testing.T) {
	src := `(cli_app
  (let (xs (list) i 0)
    (do
      (while (< i 100000)
        (do
          (append xs i)
          (set i (+ i 1))))
      (print (list_len xs) (list_get xs 0) (list_get xs 99999)))))`
	_, prog := parseAndCompile(t, src)
	policy := DefaultExecutionPolicy()
	policy.Limits.MaxInstructions = 3_000_000
	var out, errOut bytes.Buffer
	ev := RunBytecodeWithEvidence(prog, nil, policy, nil, nil, &out, &errOut, 0)
	if ev.ExitCode != 0 {
		t.Fatalf("100k append failed (%d): %s (failure=%#v)", ev.ExitCode, errOut.String(), ev.RuntimeFailure)
	}
	if got, want := out.String(), "100000 0 99999\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestVMAppendMemoryAccountingIsLinear(t *testing.T) {
	measureThreshold := func(n int) int {
		src := `(cli_app (let (xs (list) i 0) (while (< i ` + stringInt(n) + `) (do (append xs i) (set i (+ i 1))))))`
		_, prog := parseAndCompile(t, src)
		lo, hi := 0, 4_000_000
		for lo < hi {
			mid := (lo + hi) / 2
			policy := DefaultExecutionPolicy()
			policy.Limits.MaxInstructions = 1_000_000
			policy.Limits.MaxMemoryBytes = mid
			var out, errOut bytes.Buffer
			ev := RunBytecodeWithEvidence(prog, nil, policy, nil, nil, &out, &errOut, 0)
			if ev.ExitCode == 0 && ev.RuntimeFailure == nil {
				hi = mid
			} else {
				lo = mid + 1
			}
		}
		return lo
	}
	b10k := measureThreshold(10_000)
	b20k := measureThreshold(20_000)
	if b10k <= 0 || b20k > b10k*2+65536 {
		t.Fatalf("expected linear memory growth: 10k=%d bytes, 20k=%d bytes", b10k, b20k)
	}
	if b20k > 2_000_000 {
		t.Fatalf("expected O(n) bytes for 20k appends, got %d bytes", b20k)
	}
}

func stringInt(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
