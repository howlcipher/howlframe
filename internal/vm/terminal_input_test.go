package vm

import (
	"bufio"
	"strings"
	"testing"
)

func editorAfter(keys ...func(*lineEditor)) *lineEditor {
	e := newLineEditor(64)
	for _, k := range keys {
		k(e)
	}
	return e
}

func typed(s string) func(*lineEditor) { return func(e *lineEditor) { e.insert([]rune(s)) } }

func TestLineEditorEditing(t *testing.T) {
	cases := []struct {
		name   string
		keys   []func(*lineEditor)
		text   string
		cursor int
	}{
		{"insert at the cursor", []func(*lineEditor){typed("abc"), (*lineEditor).left, (*lineEditor).left, typed("X")}, "aXbc", 2},
		{"home and end", []func(*lineEditor){typed("bc"), (*lineEditor).home, typed("a"), (*lineEditor).end, typed("d")}, "abcd", 4},
		{"backspace before the cursor", []func(*lineEditor){typed("abc"), (*lineEditor).left, (*lineEditor).backspace}, "ac", 1},
		{"delete under the cursor", []func(*lineEditor){typed("abc"), (*lineEditor).home, (*lineEditor).deleteAt}, "bc", 0},
		{"kill to start keeps the rest", []func(*lineEditor){typed("abc"), (*lineEditor).left, (*lineEditor).killToStart}, "c", 0},
		{"kill to end", []func(*lineEditor){typed("abc"), (*lineEditor).home, (*lineEditor).right, (*lineEditor).killToEnd}, "a", 1},
		{"delete word and the spaces after it", []func(*lineEditor){typed("fix the bug  "), (*lineEditor).deleteWord}, "fix the ", 8},
		{"cursor stops at the edges", []func(*lineEditor){typed("ab"), (*lineEditor).right, (*lineEditor).home, (*lineEditor).left, (*lineEditor).backspace}, "ab", 0},
	}
	for _, c := range cases {
		e := editorAfter(c.keys...)
		if e.text() != c.text || e.cursor != c.cursor {
			t.Errorf("%s: %q cursor %d, want %q cursor %d", c.name, e.text(), e.cursor, c.text, c.cursor)
		}
	}
	if e := editorAfter(typed(strings.Repeat("x", 100))); len(e.buf) != 64 {
		t.Fatalf("the line is bounded: %d runes", len(e.buf))
	}
}

func TestLineEditorPasteIsMarked(t *testing.T) {
	e := editorAfter(typed("run "))
	e.paste([]rune("y"))
	if text, pasted := e.submit(); text != "run y" || !pasted {
		t.Fatalf("submit = %q pasted=%v", text, pasted)
	}
	if text, pasted := editorAfter(typed("y")).submit(); text != "y" || pasted {
		t.Fatalf("typed submit = %q pasted=%v", text, pasted)
	}
}

func TestLineEditorHistory(t *testing.T) {
	e := newLineEditor(64)
	for _, line := range []string{"first", "second", "second", "  ", "third"} {
		e.insert([]rune(line))
		e.submit()
	}
	if len(e.history) != 3 {
		t.Fatalf("blanks and repeats are skipped: %d entries", len(e.history))
	}
	e.insert([]rune("draft"))
	e.prev()
	if e.text() != "third" || e.cursor != 5 {
		t.Fatalf("prev = %q", e.text())
	}
	e.prev()
	e.prev()
	e.prev() // stays at the oldest
	if e.text() != "first" {
		t.Fatalf("oldest = %q", e.text())
	}
	e.next()
	e.next()
	e.next()
	if e.text() != "draft" {
		t.Fatalf("the draft comes back after the newest entry: %q", e.text())
	}
	for i := 0; i < termHistoryMax+50; i++ {
		e.load([]rune(strings.Repeat("n", i%7+1)+string(rune('a'+i%26))), false)
		e.submit()
	}
	if len(e.history) != termHistoryMax {
		t.Fatalf("history is capped at %d: %d", termHistoryMax, len(e.history))
	}
}

func keysOf(t *testing.T, input string) []termKey {
	t.Helper()
	in := bufio.NewReader(strings.NewReader(input))
	var keys []termKey
	for {
		k, err := readKey(in)
		if err != nil {
			return keys
		}
		keys = append(keys, k)
	}
}

func TestReadKeyDecodesTerminalSequences(t *testing.T) {
	keys := keysOf(t, "a\x1b[A\x1b[B\x1b[1;5C\x1b[D\x1bOH\x1bOF\x1b[3~\x1b[5~\x1b[6~\x1b[7~\x1b[8~\x1b[200~\x1b[201~\x1b[2~\x1bx\x01\x05\x0b\x0c\x15\x17\x0e\x10\x03\x04\r\x7f")
	want := []keyKind{keyRune, keyUp, keyDown, keyRight, keyLeft, keyHome, keyEnd, keyDelete, keyPageUp, keyPageDown, keyHome, keyEnd,
		keyPasteStart, keyPasteEnd, keyNone, keyNone, keyHome, keyEnd, keyKillToEnd, keyRedraw, keyKillToStart, keyDeleteWord, keyDown, keyUp,
		keyInterrupt, keyEOFOrDelete, keyEnter, keyBackspace}
	if len(keys) != len(want) {
		t.Fatalf("%d keys, want %d: %v", len(keys), len(want), keys)
	}
	for i := range want {
		if keys[i].kind != want[i] {
			t.Errorf("key %d = %v, want %v", i, keys[i].kind, want[i])
		}
	}
}

func TestReadPasteNeverCarriesControls(t *testing.T) {
	in := bufio.NewReader(strings.NewReader("one\r\ntwo\tthree\x1b[31m\x07four\x1b[201~after"))
	text, err := readPaste(in, 64)
	if err != nil || string(text) != "one  two threefour" {
		t.Fatalf("paste = %q, %v", string(text), err)
	}
	rest, _ := in.ReadString(0)
	if rest != "after" {
		t.Fatalf("reading stops at the end marker: %q", rest)
	}
	long := bufio.NewReader(strings.NewReader(strings.Repeat("z", 100) + "\x1b[201~"))
	if text, _ := readPaste(long, 10); len(text) != 10 {
		t.Fatalf("a paste is bounded: %d", len(text))
	}
}

func TestInputViewKeepsTheCursorVisible(t *testing.T) {
	prompt := []rune("> ")
	if text, col := inputView(prompt, []rune("abc"), 1, 80); text != "> abc" || col != 3 {
		t.Fatalf("short line: %q col %d", text, col)
	}
	line := []rune(strings.Repeat("x", 30) + "END")
	text, col := inputView(prompt, line, len(line), 20)
	if !strings.HasSuffix(text, "END") || len([]rune(text)) > 19 || col != len([]rune(text)) {
		t.Fatalf("cursor at the end: %q col %d", text, col)
	}
	text, col = inputView(prompt, line, 0, 20)
	if !strings.HasPrefix(text, "> x") || col != 2 {
		t.Fatalf("cursor at the start: %q col %d", text, col)
	}
}

func TestScrollWindow(t *testing.T) {
	cases := []struct {
		total, room, scroll               int
		start, end, above, below, scroll2 int
	}{
		{10, 23, 0, 0, 10, 0, 0, 0},      // fits: no indicators
		{50, 23, 0, 28, 50, 28, 0, 0},    // newest lines, top indicator
		{50, 23, 10, 19, 40, 19, 10, 10}, // scrolled: both indicators
		{50, 23, 999, 0, 22, 0, 28, 28},  // clamped at the top line
		{50, 23, -5, 28, 50, 28, 0, 0},   // clamped at the bottom
		{50, 2, 5, 48, 50, 0, 0, 0},      // too short for indicators
	}
	for _, c := range cases {
		start, end, above, below, scroll := scrollWindow(c.total, c.room, c.scroll)
		if start != c.start || end != c.end || above != c.above || below != c.below || scroll != c.scroll2 {
			t.Errorf("scrollWindow(%d,%d,%d) = %d %d %d %d %d, want %d %d %d %d %d", c.total, c.room, c.scroll,
				start, end, above, below, scroll, c.start, c.end, c.above, c.below, c.scroll2)
		}
		rows := end - start
		if above > 0 {
			rows++
		}
		if below > 0 {
			rows++
		}
		if c.room >= 3 && rows > c.room {
			t.Errorf("scrollWindow(%d,%d,%d) uses %d rows of %d", c.total, c.room, c.scroll, rows, c.room)
		}
	}
}

func TestStylesComeFromTheFixedPalette(t *testing.T) {
	if got := styled("bad", "error", true); got != "\x1b[31mbad\x1b[0m" {
		t.Fatalf("error = %q", got)
	}
	if got := styled("bad", "error", false); got != "bad" {
		t.Fatalf("colour off = %q", got)
	}
	view := parseView([]any{"plain", map[string]any{"text": "x", "style": "ok"}, map[string]any{"text": "y", "style": "\x1b[5m"}, map[string]any{"style": "dim"}, int64(7)})
	want := []viewLine{{"plain", ""}, {"x", "ok"}, {"y", ""}, {"", "dim"}, {"7", ""}}
	for i := range want {
		if view[i] != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, view[i], want[i])
		}
	}
	for _, mode := range []string{"auto", "always", "never"} {
		t.Setenv("NO_COLOR", "1")
		if got := colourWanted(mode, true); got != (mode == "always") {
			t.Errorf("NO_COLOR with %s = %v", mode, got)
		}
	}
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm")
	if !colourWanted("auto", true) || colourWanted("auto", false) {
		t.Fatalf("auto colours only a terminal")
	}
}
