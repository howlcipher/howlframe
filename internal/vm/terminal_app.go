package vm

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
)

// terminal_app (improvements.md #117): the runtime owns the terminal, timers
// and child processes and calls the program's handler once per event:
//
//	(terminal_app "on_event" initial_state)
//	(defun on_event (state event) ... (return (dict ("state" s) ("view" lines) ("prompt" p) ("effects" fx))))
//
// Each call runs in a fresh child VM with no grants and its own instruction
// and memory budget, like an HTTP route handler. State crosses calls only as
// JSON, bounded by MaxStateBytes. Effects come from a closed table that the
// runtime executes under the run's own grants, so the handler decides and
// the runtime holds authority.

const (
	termMaxEffectsPerEvent = 64
	termMaxTimers          = 64
	termMaxLineBytes       = 64 * 1024
	termMaxTailBytes       = 1024 * 1024
	termMaxLogLineBytes    = 64 * 1024
	termMaxTimerMillis     = 24 * 60 * 60 * 1000
	termEventQueue         = 256
)

type termEvent map[string]any

type termProc struct {
	cancel  context.CancelFunc
	signals chan os.Signal
	done    chan struct{}
}

type termTimer struct {
	t   *time.Timer
	gen int64
}

type terminalApp struct {
	vm      *BCVM
	ip      int
	handler *bytecode.BCFunction
	state   any
	events  chan termEvent
	pending []termEvent

	out      io.Writer
	inFD     uintptr
	raw      bool
	restore  func()
	paintMu  sync.Mutex
	edit     []rune
	cols     int
	rows     int
	view     []string
	prompt   string
	painted  bool
	lastText string

	procs    map[string]*termProc
	timers   map[string]*termTimer
	timerGen int64

	quit bool
	code int
}

func (vm *BCVM) runTerminalApp(handlerName string, initial any, ip int) int {
	fn, ok := vm.prog.Functions[handlerName]
	if !ok || len(fn.Params) != 2 {
		panic(NewRuntimeError("RUNTIME_ERROR", "main", ip, bytecode.OpTerminalApp, "terminal_app handler %q must be a defun with two parameters (state event)", handlerName))
	}
	if !vm.Limits.processSlots().claimTerminal() {
		panic(NewRuntimeError("RUNTIME_ERROR", "main", ip, bytecode.OpTerminalApp, "terminal_app can run only once per program"))
	}
	app := &terminalApp{vm: vm, ip: ip, handler: fn, events: make(chan termEvent, termEventQueue), out: vm.Out,
		procs: map[string]*termProc{}, timers: map[string]*termTimer{}, cols: 80, rows: 24}
	state, err := app.boundState(initial)
	if err != nil {
		panic(NewRuntimeError("LIMIT_EXCEEDED", "main", ip, bytecode.OpTerminalApp, "terminal_app initial state: %v", err))
	}
	app.state = state
	return app.loop()
}

func (a *terminalApp) loop() int {
	vm := a.vm
	if f, ok := vm.In.(*os.File); ok && vm.Out == os.Stdout && isTerminal(f.Fd()) && isTerminal(os.Stdout.Fd()) {
		a.inFD = f.Fd()
		if restore, err := makeRaw(a.inFD); err == nil {
			a.raw, a.restore = true, restore
			a.cols, a.rows = terminalSize(os.Stdout.Fd())
			// The alternate screen keeps the operator's scrollback intact.
			fmt.Fprint(a.out, "\x1b[?1049h")
		}
	}
	defer a.shutdown()
	sigs := make(chan os.Signal, 8)
	signal.Notify(sigs, terminalSignals...)
	defer signal.Stop(sigs)
	go a.readInput()

	a.pending = append(a.pending, termEvent{"type": "start", "tty": a.raw, "cols": int64(a.cols), "rows": int64(a.rows)})
	for !a.quit {
		if len(a.pending) > 0 {
			ev := a.pending[0]
			a.pending = a.pending[1:]
			a.dispatch(ev)
			continue
		}
		select {
		case ev := <-a.events:
			if ev["type"] == "process_exit" {
				id, _ := ev["id"].(string)
				delete(a.procs, id)
			}
			if a.fired(ev) {
				a.dispatch(ev)
			}
		case sig := <-sigs:
			a.onSignal(sig)
		case <-vm.ctx.Done():
			return 0 // the caller reports the deadline
		}
	}
	return a.code
}

func (a *terminalApp) onSignal(sig os.Signal) {
	switch {
	case isResizeSignal(sig):
		if a.raw {
			a.cols, a.rows = terminalSize(os.Stdout.Fd())
		}
		a.dispatch(termEvent{"type": "resize", "cols": int64(a.cols), "rows": int64(a.rows)})
		a.paint(true)
	case sig == os.Interrupt:
		a.dispatch(termEvent{"type": "interrupt"})
	default:
		// Terminate or hang up: one event to react, then the runtime stops.
		a.dispatch(termEvent{"type": "terminate"})
		if !a.quit {
			a.quit, a.code = true, 143
		}
	}
}

// dispatch runs the handler for one event and applies its result.
func (a *terminalApp) dispatch(ev termEvent) {
	result, failure := a.call(ev)
	if failure != nil {
		kind, _ := ev["type"].(string)
		if kind == "interrupt" {
			// A handler that cannot answer Ctrl-C must not trap the operator.
			a.quit, a.code = true, 130
			return
		}
		if kind != "handler_failed" {
			a.pending = append(a.pending, termEvent{"type": "handler_failed", "event_type": kind, "error": errorObject(failure)})
		}
		return
	}
	a.apply(result)
}

func (a *terminalApp) call(ev termEvent) (map[string]any, *VMError) {
	vm := a.vm
	limits := vm.Limits
	limits.MaxInstructions = positiveOr(vm.Limits.MaxEventInstructions, DefaultMaxEventInstructions)
	limits.MaxMemoryBytes = positiveOr(vm.Limits.MaxEventMemoryBytes, DefaultMaxEventMemoryBytes)
	child := &BCVM{ctx: vm.ctx, allocations: &allocationBudget{}, receipt: vm.receipt, prog: vm.prog,
		env: NewBcEnv(nil), stores: vm.stores, Limits: limits, AllowedCaps: nil, Out: io.Discard, ErrOut: io.Discard, callDepth: 1}
	event, err := jsonCopy(ev)
	if err != nil {
		return nil, NewRuntimeError("RUNTIME_ERROR", "main", a.ip, bytecode.OpTerminalApp, "terminal_app event: %v", err)
	}
	// The handler works on a copy: a failed event must leave the state as it was.
	state, err := jsonCopy(a.state)
	if err != nil {
		return nil, NewRuntimeError("RUNTIME_ERROR", "main", a.ip, bytecode.OpTerminalApp, "terminal_app state: %v", err)
	}
	var result any
	var failure *VMError
	func() {
		defer func() {
			r := recover()
			if r == nil {
				return
			}
			switch x := r.(type) {
			case VmReturn:
				result = x.val
			case *VMError:
				failure = x
			case VmExit:
				failure = NewRuntimeError("RUNTIME_ERROR", "main", child.ip, bytecode.OpExit, "exit is not allowed in a terminal_app handler; return a quit effect")
			default:
				failure = NewRuntimeError("RUNTIME_ERROR", "main", child.ip, bytecode.OpTerminalApp, "terminal_app handler failed: %v", r)
			}
		}()
		size := len(mustJSON(state)) + len(mustJSON(event))
		child.chargeAlloc(size, a.ip, bytecode.OpTerminalApp)
		callEnv := NewBcEnv(nil)
		callEnv.vars[a.handler.Params[0]] = state
		callEnv.vars[a.handler.Params[1]] = event
		child.run(a.handler.Instructions, callEnv)
	}()
	if failure != nil {
		return nil, failure
	}
	doc, ok := result.(map[string]any)
	if !ok {
		return nil, NewRuntimeError("TYPE_ERROR", "main", a.ip, bytecode.OpTerminalApp, "terminal_app handler must return a dict with state, view, prompt and effects, got %T", result)
	}
	next, err := a.boundState(doc["state"])
	if err != nil {
		return nil, NewRuntimeError("LIMIT_EXCEEDED", "main", a.ip, bytecode.OpTerminalApp, "terminal_app state: %v", err)
	}
	doc["state"] = next
	return doc, nil
}

func (a *terminalApp) boundState(v any) (any, error) {
	text, err := encodeJSONText(normalizeForJSON(v))
	if err != nil {
		return nil, err
	}
	if max := positiveOr(a.vm.Limits.MaxStateBytes, DefaultMaxStateBytes); len(text) > max {
		return nil, fmt.Errorf("%d bytes exceeds --max-state-bytes of %d", len(text), max)
	}
	return decodeJSONValue(text)
}

func (a *terminalApp) apply(doc map[string]any) {
	a.state = doc["state"]
	if lines, ok := doc["view"].([]any); ok {
		view := make([]string, len(lines))
		for i, l := range lines {
			view[i] = TextOf(l)
		}
		a.view = view
	}
	if p, ok := doc["prompt"].(string); ok {
		a.prompt = p
	}
	a.paint(false)
	effects, _ := doc["effects"].([]any)
	if len(effects) > termMaxEffectsPerEvent {
		a.pending = append(a.pending, termEvent{"type": "effect_failed", "kind": "", "id": "", "error": a.effectError("LIMIT_EXCEEDED", "more than %d effects in one event", termMaxEffectsPerEvent)})
		effects = effects[:termMaxEffectsPerEvent]
	}
	for _, e := range effects {
		if a.quit {
			return
		}
		a.effect(e)
	}
}

func (a *terminalApp) effectError(code, msg string, args ...any) map[string]any {
	return errorObject(NewRuntimeError(code, "main", a.ip, bytecode.OpTerminalApp, msg, args...))
}

// effect executes one entry of the closed effect table.
func (a *terminalApp) effect(raw any) {
	e, _ := raw.(map[string]any)
	kind, _ := e["kind"].(string)
	id, _ := e["id"].(string)
	fail := func(err map[string]any) {
		a.pending = append(a.pending, termEvent{"type": "effect_failed", "kind": kind, "id": id, "error": err})
	}
	defer func() {
		if r := recover(); r != nil {
			if vmErr, ok := r.(*VMError); ok {
				fail(errorObject(vmErr))
				return
			}
			fail(a.effectError("RUNTIME_ERROR", "%v", r))
		}
	}()
	switch kind {
	case "quit":
		a.quit, a.code = true, int(intOf(e["code"]))
	case "timer":
		a.startTimer(id, intOf(e["ms"]), fail)
	case "exec_start":
		a.startProcess(id, e["argv"], fail)
	case "exec_signal":
		a.signalProcess(id, e["signal"], fail)
	case "tail":
		a.tail(id, e, fail)
	case "append_log":
		a.appendLog(e, fail)
	default:
		fail(a.effectError("UNKNOWN_EFFECT", "unknown terminal_app effect %q", kind))
	}
}

func (a *terminalApp) startTimer(id string, ms int64, fail func(map[string]any)) {
	if id == "" || ms < 0 || ms > termMaxTimerMillis {
		fail(a.effectError("RUNTIME_ERROR", "timer needs an id and 0..%d ms", termMaxTimerMillis))
		return
	}
	if old, ok := a.timers[id]; ok {
		old.t.Stop()
	} else if len(a.timers) >= termMaxTimers {
		fail(a.effectError("LIMIT_EXCEEDED", "more than %d live timers", termMaxTimers))
		return
	}
	a.timerGen++
	gen := a.timerGen
	a.timers[id] = &termTimer{gen: gen, t: time.AfterFunc(time.Duration(ms)*time.Millisecond, func() {
		a.events <- termEvent{"type": "timer", "id": id, timerGenKey: gen}
	})}
}

// timerGenKey marks which arming of a timer fired; a replaced timer's late
// event is dropped. The key never reaches the handler.
const timerGenKey = "\x00gen"

// fired reports whether a timer event is current, retiring the timer.
func (a *terminalApp) fired(ev termEvent) bool {
	gen, ok := ev[timerGenKey].(int64)
	if !ok {
		return true
	}
	delete(ev, timerGenKey)
	id, _ := ev["id"].(string)
	current, live := a.timers[id]
	if !live || current.gen != gen {
		return false
	}
	delete(a.timers, id)
	return true
}

func (a *terminalApp) startProcess(id string, rawArgv any, fail func(map[string]any)) {
	vm := a.vm
	list, _ := rawArgv.([]any)
	if id == "" || len(list) == 0 {
		fail(a.effectError("RUNTIME_ERROR", "exec_start needs an id and a non-empty argv list"))
		return
	}
	if _, live := a.procs[id]; live {
		fail(a.effectError("RUNTIME_ERROR", "a process with id %q is still running", id))
		return
	}
	argv := execArgv(list)
	vm.receiptDecision = &effectDecision{summary: filepath.Base(argv[0])}
	program := vm.requireExec(argv[0], argv[1:], bytecode.OpExec)
	max := vm.Limits.MaxExecOutputBytes
	if max <= 0 {
		max = DefaultLimits.MaxExecOutputBytes
	}
	ctx, cancel := context.WithCancel(vm.ctx)
	proc := &termProc{cancel: cancel, signals: make(chan os.Signal, 4), done: make(chan struct{})}
	a.procs[id] = proc
	go func() {
		defer close(proc.done)
		ev := termEvent{"type": "process_exit", "id": id, "exit_code": int64(0), "stdout": "", "stderr": "", "error": nil}
		func() {
			defer func() {
				if r := recover(); r != nil {
					ev["exit_code"] = int64(-1)
					if vmErr, ok := r.(*VMError); ok {
						ev["error"] = errorObject(vmErr)
					} else {
						ev["error"] = a.effectError("RUNTIME_ERROR", "%v", r)
					}
				}
			}()
			res := vm.runExec(ctx, program, argv[1:], max, true, proc.signals, a.ip, bytecode.OpExec)
			ev["stdout"], ev["stderr"] = string(res.stdout), string(res.stderr)
			if res.err != nil {
				ev["exit_code"] = int64(res.exitCode)
				if res.exitCode == 0 {
					ev["exit_code"] = int64(-1)
				}
				ev["error"] = a.effectError("IO_ERROR", "exec failed: %v", res.err)
			}
		}()
		select {
		case a.events <- ev:
		case <-ctx.Done():
			select {
			case a.events <- ev:
			default:
			}
		}
	}()
}

func (a *terminalApp) signalProcess(id string, rawSignal any, fail func(map[string]any)) {
	proc, ok := a.procs[id]
	if !ok {
		fail(a.effectError("RUNTIME_ERROR", "no running process with id %q", id))
		return
	}
	name, _ := rawSignal.(string)
	switch name {
	case "kill":
		proc.cancel()
	case "interrupt", "terminate":
		sig := processSignal(name)
		select {
		case proc.signals <- sig:
		default:
		}
	default:
		fail(a.effectError("RUNTIME_ERROR", "exec_signal signal must be interrupt, terminate or kill"))
	}
}

func (a *terminalApp) tail(id string, e map[string]any, fail func(map[string]any)) {
	vm := a.vm
	path, _ := e["path"].(string)
	offset := intOf(e["offset"])
	vm.receiptDecision = &effectDecision{summary: path}
	resolved := vm.requireFilesystem(path, false, bytecode.OpReadFile)
	data, size, err := capability.Grants(vm.AllowedCaps).ScopedReadFrom(resolved, offset, termMaxTailBytes)
	ev := termEvent{"type": "tail_lines", "id": id, "path": path, "lines": []any{}, "offset": offset, "missing": false}
	if errors.Is(err, fs.ErrNotExist) {
		ev["missing"] = true
		a.pending = append(a.pending, ev)
		return
	}
	if err != nil {
		vm.failScopedFilesystem(err, bytecode.OpReadFile)
		fail(a.effectError("IO_ERROR", "tail failed: %v", err))
		return
	}
	if offset < 0 || offset > size {
		offset = size
	}
	whole := strings.LastIndexByte(string(data), '\n') + 1
	lines := []any{}
	for _, l := range strings.Split(string(data[:whole]), "\n") {
		if l != "" {
			lines = append(lines, strings.TrimSuffix(l, "\r"))
		}
	}
	ev["lines"], ev["offset"] = lines, offset+int64(whole)
	a.pending = append(a.pending, ev)
}

func (a *terminalApp) appendLog(e map[string]any, fail func(map[string]any)) {
	vm := a.vm
	path, _ := e["path"].(string)
	line := strings.ReplaceAll(TextOf(e["line"]), "\n", " ")
	if len(line) > termMaxLogLineBytes {
		fail(a.effectError("LIMIT_EXCEEDED", "log line exceeds %d bytes", termMaxLogLineBytes))
		return
	}
	vm.receiptDecision = &effectDecision{summary: path}
	resolved := vm.requireFilesystem(path, true, bytecode.OpWriteFile)
	if err := capability.Grants(vm.AllowedCaps).ScopedAppendFile(resolved, []byte(line+"\n"), 0o600); err != nil {
		vm.failScopedFilesystem(err, bytecode.OpWriteFile)
		fail(a.effectError("IO_ERROR", "append_log failed: %v", err))
	}
}

// readInput turns the terminal into events. In raw mode the runtime edits
// the line itself; otherwise it reads whole lines (also the test interface).
func (a *terminalApp) readInput() {
	vm := a.vm
	if !a.raw {
		reader := vm.lineReader
		if reader == nil {
			reader = bufio.NewReader(vm.In)
		}
		for {
			line, err := readBoundedLine(reader, termMaxLineBytes)
			if err != nil && line == "" {
				a.events <- termEvent{"type": "eof"}
				return
			}
			a.events <- termEvent{"type": "line", "text": line}
			if err != nil {
				a.events <- termEvent{"type": "eof"}
				return
			}
		}
	}
	in := bufio.NewReader(os.NewFile(a.inFD, "terminal"))
	for {
		r, _, err := in.ReadRune()
		if err != nil {
			a.events <- termEvent{"type": "eof"}
			return
		}
		switch {
		case r == 0x03:
			a.events <- termEvent{"type": "interrupt"}
		case r == 0x04:
			if a.editText() == "" {
				a.events <- termEvent{"type": "eof"}
			}
		case r == '\r' || r == '\n':
			text := a.takeEdit()
			a.events <- termEvent{"type": "line", "text": text}
		case r == 0x7f || r == 0x08:
			a.changeEdit(func(e []rune) []rune {
				if len(e) > 0 {
					return e[:len(e)-1]
				}
				return e
			})
		case r == 0x15:
			a.changeEdit(func([]rune) []rune { return nil })
		case r == 0x1b:
			dropEscapeSequence(in)
		case r < 0x20 || r == utf8.RuneError:
		default:
			a.changeEdit(func(e []rune) []rune {
				if len(e) < termMaxLineBytes/4 {
					return append(e, r)
				}
				return e
			})
		}
	}
}

func dropEscapeSequence(in *bufio.Reader) {
	next, _, err := in.ReadRune()
	if err != nil || (next != '[' && next != 'O') {
		return
	}
	for {
		b, err := in.ReadByte()
		if err != nil || (b >= 0x40 && b <= 0x7e) {
			return
		}
	}
}

func readBoundedLine(r *bufio.Reader, max int) (string, error) {
	var b []byte
	for {
		chunk, err := r.ReadSlice('\n')
		if len(b) < max {
			room := max - len(b)
			if len(chunk) > room {
				chunk = chunk[:room]
			}
			b = append(b, chunk...)
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		return strings.TrimRight(string(b), "\r\n"), err
	}
}

func (a *terminalApp) editText() string {
	a.paintMu.Lock()
	defer a.paintMu.Unlock()
	return string(a.edit)
}

func (a *terminalApp) takeEdit() string {
	a.paintMu.Lock()
	text := string(a.edit)
	a.edit = nil
	a.paintMu.Unlock()
	a.repaintInput()
	return text
}

func (a *terminalApp) changeEdit(f func([]rune) []rune) {
	a.paintMu.Lock()
	a.edit = f(a.edit)
	a.paintMu.Unlock()
	a.repaintInput()
}

func (a *terminalApp) repaintInput() {
	if !a.raw {
		return
	}
	a.paintMu.Lock()
	defer a.paintMu.Unlock()
	fmt.Fprint(a.out, "\r\x1b[K"+a.inputLine())
}

// inputLine is the prompt and the edited text, keeping the tail visible.
func (a *terminalApp) inputLine() string {
	line := []rune(sanitizeTerminal(a.prompt) + sanitizeTerminal(string(a.edit)))
	if a.cols > 1 && len(line) >= a.cols {
		line = line[len(line)-a.cols+1:]
	}
	return string(line)
}

// paint draws the view and prompt. Line mode writes a frame only when it
// changed; raw mode redraws the screen.
func (a *terminalApp) paint(force bool) {
	a.paintMu.Lock()
	defer a.paintMu.Unlock()
	var lines []string
	for _, l := range a.view {
		if a.raw {
			lines = append(lines, wrapTerminal(sanitizeTerminal(l), a.cols)...)
		} else {
			lines = append(lines, sanitizeTerminal(l))
		}
	}
	if !a.raw {
		text := strings.Join(lines, "\n") + "\n" + sanitizeTerminal(a.prompt)
		if text == a.lastText && !force {
			return
		}
		lead := ""
		if a.painted {
			lead = "\n"
		}
		fmt.Fprint(a.out, lead+text)
		a.lastText, a.painted = text, true
		return
	}
	if room := a.rows - 1; room > 0 && len(lines) > room {
		lines = lines[len(lines)-room:]
	}
	var b strings.Builder
	b.WriteString("\x1b[H\x1b[2J")
	for _, l := range lines {
		b.WriteString(l + "\r\n")
	}
	b.WriteString(a.inputLine())
	fmt.Fprint(a.out, b.String())
	a.painted = true
}

// sanitizeTerminal replaces control characters, so text from a child or the
// program cannot move the cursor, change modes or inject sequences.
func sanitizeTerminal(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\t':
			return ' '
		case r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0):
			return '?'
		}
		return r
	}, s)
}

// wrapTerminal splits a line at the terminal width; continuation lines get
// a two-space hanging indent so a wrapped line reads as one.
func wrapTerminal(line string, width int) []string {
	runes := []rune(line)
	if width <= 4 || len(runes) <= width {
		return []string{line}
	}
	out := []string{string(runes[:width])}
	runes = runes[width:]
	for len(runes) > width-2 {
		out = append(out, "  "+string(runes[:width-2]))
		runes = runes[width-2:]
	}
	return append(out, "  "+string(runes))
}

func (a *terminalApp) shutdown() {
	for _, t := range a.timers {
		t.t.Stop()
	}
	for _, p := range a.procs {
		p.cancel()
	}
	for _, p := range a.procs {
		select {
		case <-p.done:
		case <-time.After(10 * time.Second):
		}
	}
	if a.restore != nil {
		a.paintMu.Lock()
		fmt.Fprint(a.out, "\r\n\x1b[?1049l")
		a.restore()
		a.paintMu.Unlock()
	} else if a.painted {
		fmt.Fprint(a.out, "\n")
	}
}

func positiveOr(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}

func intOf(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	}
	return 0
}

func jsonCopy(v any) (any, error) {
	text, err := encodeJSONText(normalizeForJSON(v))
	if err != nil {
		return nil, err
	}
	return decodeJSONValue(text)
}

func mustJSON(v any) []byte {
	b, _ := encodeJSONText(normalizeForJSON(v))
	return b
}

// errorObject is a runtime error as the versioned object a handler reads.
func errorObject(e *VMError) map[string]any {
	doc, err := decodeJSONValue([]byte(e.Error()))
	if m, ok := doc.(map[string]any); ok && err == nil {
		return m
	}
	return map[string]any{"schema": RuntimeErrorSchema, "code": e.Code, "message": e.Message}
}
