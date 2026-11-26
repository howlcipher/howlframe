package vm

import (
	"bufio"
	"strconv"
	"strings"
)

// The raw-mode input side of terminal_app (improvements.md #119): key
// decoding, the line editor with session history, bracketed paste, and the
// pure layout helpers the painter uses. Nothing here reaches the program
// except the submitted line and whether any of it was pasted.

const termHistoryMax = 200

type keyKind int

const (
	keyNone keyKind = iota
	keyRune
	keyEnter
	keyBackspace
	keyDelete
	keyLeft
	keyRight
	keyUp
	keyDown
	keyHome
	keyEnd
	keyPageUp
	keyPageDown
	keyPasteStart
	keyPasteEnd
	keyInterrupt
	keyEOFOrDelete
	keyKillToStart
	keyKillToEnd
	keyDeleteWord
	keyRedraw
)

type termKey struct {
	kind keyKind
	r    rune
}

// readKey decodes one key from the terminal. Escape sequences it does not
// know are consumed whole and returned as keyNone, so they never reach the
// line as text.
func readKey(in *bufio.Reader) (termKey, error) {
	r, _, err := in.ReadRune()
	if err != nil {
		return termKey{}, err
	}
	switch r {
	case 0x01:
		return termKey{kind: keyHome}, nil
	case 0x02:
		return termKey{kind: keyLeft}, nil
	case 0x03:
		return termKey{kind: keyInterrupt}, nil
	case 0x04:
		return termKey{kind: keyEOFOrDelete}, nil
	case 0x05:
		return termKey{kind: keyEnd}, nil
	case 0x06:
		return termKey{kind: keyRight}, nil
	case 0x08, 0x7f:
		return termKey{kind: keyBackspace}, nil
	case 0x0b:
		return termKey{kind: keyKillToEnd}, nil
	case 0x0c:
		return termKey{kind: keyRedraw}, nil
	case '\r', '\n':
		return termKey{kind: keyEnter}, nil
	case 0x0e:
		return termKey{kind: keyDown}, nil
	case 0x10:
		return termKey{kind: keyUp}, nil
	case 0x15:
		return termKey{kind: keyKillToStart}, nil
	case 0x17:
		return termKey{kind: keyDeleteWord}, nil
	case 0x1b:
		return readEscape(in)
	}
	if r < 0x20 || r == 0xfffd {
		return termKey{}, nil
	}
	return termKey{kind: keyRune, r: r}, nil
}

// readEscape decodes the rest of an escape sequence (CSI "ESC [" or SS3
// "ESC O"); any other byte after ESC (an Alt chord) is dropped.
func readEscape(in *bufio.Reader) (termKey, error) {
	next, _, err := in.ReadRune()
	if err != nil {
		return termKey{}, err
	}
	switch next {
	case 'O':
		b, err := in.ReadByte()
		if err != nil {
			return termKey{}, err
		}
		return termKey{kind: finalKey(b, "")}, nil
	case '[':
		var params strings.Builder
		for {
			b, err := in.ReadByte()
			if err != nil {
				return termKey{}, err
			}
			if b >= 0x40 && b <= 0x7e {
				return termKey{kind: finalKey(b, params.String())}, nil
			}
			params.WriteByte(b)
		}
	}
	return termKey{}, nil
}

// finalKey maps a CSI or SS3 final byte and its parameters to a key;
// modifier parameters ("1;5") are ignored.
func finalKey(final byte, params string) keyKind {
	switch final {
	case 'A':
		return keyUp
	case 'B':
		return keyDown
	case 'C':
		return keyRight
	case 'D':
		return keyLeft
	case 'H':
		return keyHome
	case 'F':
		return keyEnd
	case '~':
		first := params
		if i := strings.IndexByte(params, ';'); i >= 0 {
			first = params[:i]
		}
		switch n, _ := strconv.Atoi(first); n {
		case 1, 7:
			return keyHome
		case 4, 8:
			return keyEnd
		case 3:
			return keyDelete
		case 5:
			return keyPageUp
		case 6:
			return keyPageDown
		case 200:
			return keyPasteStart
		case 201:
			return keyPasteEnd
		}
	}
	return keyNone
}

// readPaste collects bracketed-paste text up to the end marker. Line breaks
// and tabs become spaces and other control characters are dropped, so a
// paste can never submit a line or carry a control sequence.
func readPaste(in *bufio.Reader, max int) ([]rune, error) {
	var out []rune
	for {
		r, _, err := in.ReadRune()
		if err != nil {
			return out, err
		}
		if r == 0x1b {
			k, err := readEscape(in)
			if err != nil {
				return out, err
			}
			if k.kind == keyPasteEnd {
				return out, nil
			}
			continue
		}
		switch {
		case r == '\r' || r == '\n' || r == '\t':
			r = ' '
		case r < 0x20 || r == 0x7f || r == 0xfffd:
			continue
		}
		if len(out) < max {
			out = append(out, r)
		}
	}
}

type historyEntry struct {
	text   []rune
	pasted bool
}

// lineEditor is the raw-mode input line. History lives only in memory, for
// this session: objectives can carry anything, so nothing is written down.
type lineEditor struct {
	buf         []rune
	cursor      int
	pasted      bool
	max         int
	history     []historyEntry
	histPos     int
	draft       []rune
	draftPasted bool
}

func newLineEditor(max int) *lineEditor { return &lineEditor{max: max} }

func (e *lineEditor) text() string { return string(e.buf) }

func (e *lineEditor) insert(rs []rune) {
	room := e.max - len(e.buf)
	if room <= 0 {
		return
	}
	if len(rs) > room {
		rs = rs[:room]
	}
	out := make([]rune, 0, len(e.buf)+len(rs))
	out = append(out, e.buf[:e.cursor]...)
	out = append(out, rs...)
	out = append(out, e.buf[e.cursor:]...)
	e.buf = out
	e.cursor += len(rs)
}

func (e *lineEditor) paste(rs []rune) {
	if len(rs) == 0 {
		return
	}
	e.insert(rs)
	e.pasted = true
}

func (e *lineEditor) backspace() {
	if e.cursor > 0 {
		e.buf = append(e.buf[:e.cursor-1], e.buf[e.cursor:]...)
		e.cursor--
	}
}

func (e *lineEditor) deleteAt() {
	if e.cursor < len(e.buf) {
		e.buf = append(e.buf[:e.cursor], e.buf[e.cursor+1:]...)
	}
}

func (e *lineEditor) left() {
	if e.cursor > 0 {
		e.cursor--
	}
}

func (e *lineEditor) right() {
	if e.cursor < len(e.buf) {
		e.cursor++
	}
}

func (e *lineEditor) home() { e.cursor = 0 }
func (e *lineEditor) end()  { e.cursor = len(e.buf) }

func (e *lineEditor) killToStart() {
	e.buf = append([]rune{}, e.buf[e.cursor:]...)
	e.cursor = 0
}

func (e *lineEditor) killToEnd() { e.buf = e.buf[:e.cursor] }

// deleteWord removes the word before the cursor and the spaces after it, as
// readline's Ctrl-W does.
func (e *lineEditor) deleteWord() {
	i := e.cursor
	for i > 0 && e.buf[i-1] == ' ' {
		i--
	}
	for i > 0 && e.buf[i-1] != ' ' {
		i--
	}
	e.buf = append(e.buf[:i], e.buf[e.cursor:]...)
	e.cursor = i
}

func (e *lineEditor) load(text []rune, pasted bool) {
	e.buf = append([]rune{}, text...)
	e.cursor = len(e.buf)
	e.pasted = pasted
}

// prev and next walk the session history; the line being edited is kept as
// a draft and comes back after the newest entry.
func (e *lineEditor) prev() {
	if e.histPos == 0 {
		return
	}
	if e.histPos == len(e.history) {
		e.draft, e.draftPasted = append([]rune{}, e.buf...), e.pasted
	}
	e.histPos--
	h := e.history[e.histPos]
	e.load(h.text, h.pasted)
}

func (e *lineEditor) next() {
	if e.histPos >= len(e.history) {
		return
	}
	e.histPos++
	if e.histPos == len(e.history) {
		e.load(e.draft, e.draftPasted)
		return
	}
	h := e.history[e.histPos]
	e.load(h.text, h.pasted)
}

// submit returns the line and whether any of it was pasted, records it in the
// history (skipping blanks and repeats) and clears the editor.
func (e *lineEditor) submit() (string, bool) {
	text, pasted := string(e.buf), e.pasted
	if strings.TrimSpace(text) != "" && (len(e.history) == 0 || string(e.history[len(e.history)-1].text) != text) {
		e.history = append(e.history, historyEntry{text: append([]rune{}, e.buf...), pasted: pasted})
		if len(e.history) > termHistoryMax {
			e.history = e.history[len(e.history)-termHistoryMax:]
		}
	}
	e.buf, e.cursor, e.pasted = nil, 0, false
	e.histPos, e.draft, e.draftPasted = len(e.history), nil, false
	return text, pasted
}

// inputView is the visible slice of prompt and line for a terminal cols wide,
// scrolled horizontally to keep the cursor in view, and the cursor's column.
func inputView(prompt, line []rune, cursor, cols int) (string, int) {
	full := append(append([]rune{}, prompt...), line...)
	pos := len(prompt) + cursor
	width := cols - 1 // the last column stays free, so the cursor never wraps
	if width < 1 || len(full) < width {
		return string(full), pos
	}
	start := pos - width + 1
	if limit := len(full) + 1 - width; start > limit {
		start = limit
	}
	if start < 0 {
		start = 0
	}
	end := start + width
	if end > len(full) {
		end = len(full)
	}
	return string(full[start:end]), pos - start
}

// scrollWindow chooses which of total lines fill room rows when the view is
// scrolled up by scroll lines, reserving a row for each "more" indicator. It
// returns lines [start, end), the counts the above and below indicators show
// (0: no indicator), and the scroll clamped to its range. A terminal too
// short for indicators shows the newest lines.
func scrollWindow(total, room, scroll int) (start, end, above, below, clamped int) {
	if total <= room || room < 3 {
		start = total - room
		if start < 0 {
			start = 0
		}
		return start, total, 0, 0, 0
	}
	maxScroll := total - (room - 1)
	if scroll < 0 {
		scroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	end = total - scroll
	avail := room
	if scroll > 0 {
		avail--
	}
	start = end - avail
	if start > 0 {
		start = end - (avail - 1)
	}
	if start < 0 {
		start = 0
	}
	return start, end, start, scroll, scroll
}

// termStyles is the fixed palette a view line may name; only the runtime
// turns a name into SGR, so a program never sends an escape of its own.
var termStyles = map[string]string{
	"plain": "", "bold": "1", "dim": "2", "heading": "1", "info": "36", "ok": "32", "warning": "33", "error": "31",
}

func styled(text, style string, colour bool) string {
	sgr := termStyles[style]
	if !colour || sgr == "" {
		return text
	}
	return "\x1b[" + sgr + "m" + text + "\x1b[0m"
}
