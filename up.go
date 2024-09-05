// Copyright 2018-2024 The up AUTHORS
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// up is the Ultimate Plumber, a tool for writing Linux pipes in a
// terminal-based UI interactively, with instant live preview of command
// results.
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"slices"
	"sync"
	"unicode"

	"github.com/gdamore/tcell"
	"github.com/gdamore/tcell/terminfo"
	"github.com/mattn/go-isatty"
	"github.com/mattn/go-runewidth"
	"github.com/spf13/pflag"
)

const version = "0.4 (2020-10-29)"

// TODO: in case of error, show it in red (bg?), then below show again initial normal output (see also #4)
// TODO: F1 should display help, and it should be multi-line, and scrolling licensing credits
// TODO: some key shortcut to increase stdin capture buffer size (unless EOF already reached)
// TODO: show status infos:
//  - red fg + "up: process returned with error code %d" -- when subprocess returned an error
//  - yellow fg -- when process is still not finished
// TODO: on github: add issues, incl. up-for-grabs / help-wanted
// TODO: [LATER] make it work on Windows; maybe with mattn/go-shellwords ?
// TODO: [LATER] Ctrl-O shows input via `less` or $PAGER
// TODO: properly show all licenses of dependencies on --version
// TODO: [LATER] on ^X (?), leave TUI and run the command through buffered input, then unpause rest of input
// TODO: [LATER] allow adding more elements of pipeline (initially, just writing `foo | bar` should work)
// TODO: [LATER] allow invocation with partial command, like: `up grep -i` (see also #11)
// TODO: [LATER][MAYBE] allow reading upN.sh scripts (see also #11)
// TODO: [MUCH LATER] readline-like rich editing support? and completion? (see also #28)
// TODO: [MUCH LATER] integration with fzf? and pindexis/marker?
// TODO: [LATER] forking and unforking pipelines (see also #4)
// TODO: [LATER] capture output of a running process (see: https://stackoverflow.com/q/19584825/98528)
// TODO: [LATER] richer TUI:
// - show # of read lines & kbytes
// - show status (errorlevel) of process, or that it's still running (also with background colors)
// - allow copying and pasting to/from command line
// TODO: [LATER] allow connecting external editor (become server/engine via e.g. socket)
// TODO: [LATER] become pluggable into http://luna-lang.org
// TODO: [LATER][MAYBE] allow "plugins" ("combos" - commands with default options) e.g. for Lua `lua -e`+auto-quote, etc.
// TODO: [LATER] make it more friendly to infrequent Linux users by providing "descriptive" commands like "search" etc.
// TODO: [LATER] advertise on some reddits for data exploration / data science
// TODO: [LATER] undo/redo - history of commands (see also #4)
// TODO: [LATER] jump between buffers saved from earlier pipe fragments; OR: allow saving/recalling "snapshots" of (cmd, results) pairs (see also #4)
// TODO: [LATER] ^-, U -- to switch to "unsafe mode"? -u to switch back? + some visual marker

func init() {
	pflag.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: COMMAND | up [OPTIONS]

up is the Ultimate Plumber, a tool for writing Linux pipes in a terminal-based
UI interactively, with instant live preview of command results.

To start using up, redirect any text-emitting command (or pipeline) into it -
for example:

    $ lshw |& ./up

Ultimate Plumber then opens a full-screen terminal app. The top line of the
screen can be edited in order to interactively build a pipeline. Every time you
hit [Enter], the bottom of the screen will display the results of passing the
up's standard input through the pipeline (executed using your default $SHELL).

If a tilde '~' is visible in top-left corner, it indicates that Ultimate
Plumber did not yet fully consume its input. Some pipelines may not finish with
incomplete input; use Ctrl-S to freeze reading the input and to inject fake
EOF; use Ctrl-Q to unfreeze back and continue reading.

If a plus '+' is visible in top-left corner, the internal buffer limit
(default: 40MB) was reached and Ultimate Plumber won't read more input.

KEYS

- alphanumeric & symbol keys, Left, Right, Ctrl-A/E/B/F/K/Y/W
                      - navigate and edit the pipeline command
- Enter   - execute the pipeline command, updating the pipeline output panel
- Up, Dn, PgUp, PgDn, Ctrl-Left, Ctrl-Right
                      - navigate (scroll) the pipeline output panel
- Ctrl-X  - exit and write the pipeline to up1.sh (or if it exists then to
            up2.sh, etc. till up1000.sh)
- Ctrl-C  - quit without saving and emit the pipeline on standard output
- Ctrl-S  - temporarily freeze a long-running input to Ultimate Plumber,
            injecting a fake EOF into the buffer (shows '#' indicator in
            top-left corner)
- Ctrl-Q  - unfreeze back after Ctrl-S (disables '#' indicator)

OPTIONS
`)
		pflag.PrintDefaults()
		fmt.Fprint(os.Stderr, `
HOMEPAGE: https://github.com/akavel/up
VERSION: `+version+`
`)
	}
	pflag.ErrHelp = errors.New("") // TODO: or something else?
}

var (
	// TODO: dangerous? immediate? raw? unsafe? ...
	// FIXME(akavel): mark the unsafe mode vs. safe mode with some colour or status; also inform/mark what command's results are displayed...
	unsafeMode   = pflag.Bool("unsafe-full-throttle", false, "enable mode in which pipeline is executed immediately after any change (without pressing Enter)")
	outputScript = pflag.StringP("output-script", "o", "", "save the command to specified `file` if Ctrl-X is pressed (default: up<N>.sh)")
	debugMode    = pflag.Bool("debug", false, "debug mode")
	noColors     = pflag.Bool("no-colors", false, "disable interface colors")
	shellFlag    = pflag.StringArrayP("exec", "e", nil, "`command` to run pipeline with; repeat multiple times to pass multi-word command; defaults to '-e=$SHELL -e=-c'")
	initialCmd   = pflag.StringP("pipeline", "c", "", "initial `commands` to use as pipeline (default empty)")
	bufsize      = pflag.Int("buf", 40, "input buffer size & pipeline buffer sizes in `megabytes` (MiB)")
	noinput      = pflag.Bool("noinput", false, "start with empty buffer regardless if any input was provided")
)

func main() {
	// Handle command-line flags
	pflag.Parse()

	log.SetOutput(io.Discard)
	if *debugMode {
		debug, err := os.Create("up.debug")
		if err != nil {
			die(err.Error())
		}
		log.SetOutput(debug)
	}

	// Find out what is the user's preferred login shell. This also allows user
	// to choose the "engine" used for command execution.
	shell := *shellFlag
	if len(shell) == 0 {
		log.Println("checking $SHELL...")
		sh := os.Getenv("SHELL")
		if sh != "" {
			goto shell_found
		}
		log.Println("checking bash...")
		sh, _ = exec.LookPath("bash")
		if sh != "" {
			goto shell_found
		}
		log.Println("checking sh...")
		sh, _ = exec.LookPath("sh")
		if sh != "" {
			goto shell_found
		}
		die("cannot find shell: no -e flag, $SHELL is empty, neither bash nor sh are in $PATH")
	shell_found:
		shell = []string{sh, "-c"}
	}
	log.Println("found shell:", shell)

	stdin := io.Reader(os.Stdin)
	if *noinput {
		stdin = bytes.NewReader(nil)
	} else if isatty.IsTerminal(os.Stdin.Fd()) {
		// TODO: Without this block, we'd hang when nothing is piped on input (see
		// github.com/peco/peco, mattn/gof, fzf, etc.)
		die("up requires some data piped on standard input, for example try: `echo hello world | up`")
	}

	// Initialize TUI infrastructure
	tui := initTUI()
	defer tui.Fini()

	// Initialize 3 main UI parts
	var (
		// The top line of the TUI is an editable command, which will be used
		// as a pipeline for data we read from stdin
		commandEditor = NewEditor("| ", *initialCmd)
		// The rest of the screen is a view of the results of the command
		commandOutput = BufView{}
		// Sometimes, a message may be displayed at the bottom of the screen, with help or other info
		message = `Enter runs  ^X exit (^C nosave)  PgUp/PgDn/Up/Dn/^</^> scroll  ^S pause (^Q end)  [Ultimate Plumber v` + version + ` by akavel et al.]`
	)

	// Initialize main data flow
	var (
		// We capture data piped to 'up' on standard input into an internal buffer
		// When some new data shows up on stdin, we raise a custom signal,
		// so that main loop will refresh the buffers and the output.
		stdinCapture = NewBuf(*bufsize*1024*1024).
				StartCapturing(stdin, func() { triggerRefresh(tui) })
		// Then, we pass this data as input to a subprocess.
		// Initially, no subprocess is running, as no command is entered yet
		commandSubprocess *Subprocess = nil
	)
	// Intially, for user's convenience, show the raw input data, as if `cat` command was typed
	commandOutput.Buf = stdinCapture

	// Main loop
	lastCommand := ""
	restart := false
	for {
		// If user edited the command, immediately run it in background, and
		// kill the previously running command.
		command := commandEditor.String()
		if restart || (*unsafeMode && command != lastCommand) {
			commandSubprocess.Kill()
			if command != "" {
				commandSubprocess = StartSubprocess(shell, command, stdinCapture, func() { triggerRefresh(tui) })
				commandOutput.Buf = commandSubprocess.Buf
			} else {
				// If command is empty, show original input data again (~ equivalent of typing `cat`)
				commandSubprocess = nil
				commandOutput.Buf = stdinCapture
			}
			restart = false
			lastCommand = command
		}

		// Draw UI
		w, h := tui.Size()
		style := whiteOnBlue
		if command == lastCommand {
			style = whiteOnDBlue
		}
		stdinCapture.DrawStatus(TuiRegion(tui, 0, 0, 1, 1), style)
		commandEditor.DrawTo(TuiRegion(tui, 1, 0, w-1, 1), style,
			func(x, y int) { tui.ShowCursor(x+1, 0) })
		commandOutput.DrawTo(TuiRegion(tui, 0, 1, w, h-1))
		drawText(TuiRegion(tui, 0, h-1, w, 1), whiteOnBlue, message)
		tui.Show()

		// Handle UI events
		switch ev := tui.PollEvent().(type) {
		// Key pressed
		case *tcell.EventKey:
			// Is it a command editor key?
			if commandEditor.HandleKey(ev) {
				message = ""
				continue
			}
			// Is it a command output view key?
			if commandOutput.HandleKey(ev, h-1) {
				message = ""
				continue
			}
			// Some other global key combinations
			switch getKey(ev) {
			case key(tcell.KeyEnter):
				restart = true
			case key(tcell.KeyCtrlUnderscore),
				ctrlKey(tcell.KeyCtrlUnderscore):
				// TODO: ask for another character to trigger command-line option, like in `less`

			case key(tcell.KeyCtrlS),
				ctrlKey(tcell.KeyCtrlS):
				stdinCapture.Pause(true)
				triggerRefresh(tui)
			case key(tcell.KeyCtrlQ),
				ctrlKey(tcell.KeyCtrlQ):
				stdinCapture.Pause(false)
				restart = true
			case key(tcell.KeyCtrlC),
				ctrlKey(tcell.KeyCtrlC),
				key(tcell.KeyCtrlD),
				ctrlKey(tcell.KeyCtrlD):
				// Quit
				tui.Fini()
				os.Stderr.WriteString("up: Ultimate Plumber v" + version + " https://github.com/akavel/up\n")
				os.Stderr.WriteString("up: | " + commandEditor.String() + "\n")
				return
			case key(tcell.KeyCtrlX),
				ctrlKey(tcell.KeyCtrlX):
				// Write script 'upN.sh' and quit
				tui.Fini()
				writeScript(shell, commandEditor.String(), tui)
				return
			}
		}
	}
}

func initTUI() tcell.Screen {
	// TODO: maybe try gocui or termbox?
	tui, err := tcell.NewScreen()
	if err == terminfo.ErrTermNotFound {
		term := os.Getenv("TERM")
		hash := sha1.Sum([]byte(term))
		// TODO: add a flag which would attempt to perform the download automatically if explicitly requested by user
		die(fmt.Sprintf(`%[1]s
Your terminal code:
	TERM=%[2]s
was not found in the database provided by tcell library. Please try checking if
a supplemental database is found for your terminal at one of the following URLs:
	https://github.com/gdamore/tcell/raw/master/terminfo/database/%.1[3]x/%.4[3]x
	https://github.com/gdamore/tcell/raw/master/terminfo/database/%.1[3]x/%.4[3]x.gz
If yes, download it and save in the following directory:
	$HOME/.tcelldb/%.1[3]x/
then try running "up" again. If that does not work for you, please first consult:
	https://github.com/akavel/up/issues/15
and if you don't see your terminal code mentioned there, please try asking on:
	https://github.com/gdamore/tcell/issues
Or, you might try changing TERM temporarily to some other value, for example by
running "up" with:
	TERM=xterm up
Good luck!`,
			err, term, hash))
	}
	if err != nil {
		die(err.Error())
	}
	err = tui.Init()
	if err != nil {
		die(err.Error())
	}
	return tui
}

func triggerRefresh(tui tcell.Screen) {
	tui.PostEvent(tcell.NewEventInterrupt(nil))
}

func die(message string) {
	os.Stderr.WriteString("error: " + message + "\n")
	os.Exit(1)
}

func NewEditor(prompt, value string) *Editor {
	v := []rune(value)
	return &Editor{
		prompt: []rune(prompt),
		value:  v,
		cursor: len(v),
		lastw:  len(v),
	}
}

type Editor struct {
	// TODO: make editor multiline. Reuse gocui or something for this?
	prompt    []rune
	value     []rune
	killspace []rune
	cursor    int
	// lastw is length of value on last Draw; we need it to know how much to erase after backspace
	lastw int
}

func (e *Editor) String() string { return string(e.value) }

func (e *Editor) DrawTo(region Region, style tcell.Style, setcursor func(x, y int)) {
	// Draw prompt & the edited value - use white letters on blue background
	for i, ch := range e.prompt {
		region.SetCell(i, 0, style, ch)
	}
	for i, ch := range e.value {
		region.SetCell(len(e.prompt)+i, 0, style, ch)
	}

	// Clear remains of last value if needed
	for i := len(e.value); i < e.lastw; i++ {
		region.SetCell(len(e.prompt)+i, 0, tcell.StyleDefault, ' ')
	}
	e.lastw = len(e.value)

	// Show cursor if requested
	if setcursor != nil {
		setcursor(len(e.prompt)+e.cursor, 0)
	}
}

func (e *Editor) HandleKey(ev *tcell.EventKey) bool {
	// If a character is entered, with no modifiers except maybe shift, then just insert it
	if ev.Key() == tcell.KeyRune && ev.Modifiers()&(^tcell.ModShift) == 0 {
		e.insert(ev.Rune())
		return true
	}
	// Handle editing & movement keys
	switch getKey(ev) {
	case key(tcell.KeyBackspace), key(tcell.KeyBackspace2):
		// See https://github.com/nsf/termbox-go/issues/145
		e.delete(-1)
	case key(tcell.KeyDelete):
		e.delete(0)
	case key(tcell.KeyLeft),
		key(tcell.KeyCtrlB),
		ctrlKey(tcell.KeyCtrlB):
		if e.cursor > 0 {
			e.cursor--
		}
	case key(tcell.KeyRight),
		key(tcell.KeyCtrlF),
		ctrlKey(tcell.KeyCtrlF):
		if e.cursor < len(e.value) {
			e.cursor++
		}
	case key(tcell.KeyCtrlA),
		ctrlKey(tcell.KeyCtrlA):
		e.cursor = 0
	case key(tcell.KeyCtrlE),
		ctrlKey(tcell.KeyCtrlE):
		e.cursor = len(e.value)
	case key(tcell.KeyCtrlK),
		ctrlKey(tcell.KeyCtrlK):
		e.kill()
	case key(tcell.KeyCtrlY),
		ctrlKey(tcell.KeyCtrlY):
		e.insert(e.killspace...)
	case key(tcell.KeyCtrlW),
		ctrlKey(tcell.KeyCtrlW):
		e.unixWordRubout()
	default:
		// Unknown key/combination, not handled
		return false
	}
	return true
}

func (e *Editor) insert(ch ...rune) {
	e.value = slices.Insert(e.value, e.cursor, ch...)
	e.cursor += len(ch)
}

func (e *Editor) delete(dx int) {
	pos := e.cursor + dx
	if pos < 0 || pos >= len(e.value) {
		return
	}
	e.value = slices.Delete(e.value, pos, pos+1)
	e.cursor = pos
}

func (e *Editor) kill() {
	if e.cursor != len(e.value) {
		e.killspace = append(e.killspace[:0], e.value[e.cursor:]...)
	}
	e.value = e.value[:e.cursor]
}

// unixWordRubout removes the part of the word on the left of the cursor. A word is
// delimited by whitespaces.
// The term `unix-word-rubout` comes from `readline` (see `man 3 readline`)
func (e *Editor) unixWordRubout() {
	if e.cursor <= 0 {
		return
	}
	pos := e.cursor - 1
	for pos != 0 && (unicode.IsSpace(e.value[pos]) || !unicode.IsSpace(e.value[pos-1])) {
		pos--
	}
	e.killspace = append(e.killspace[:0], e.value[pos:e.cursor]...)
	e.value = slices.Delete(e.value, pos, e.cursor)
	e.cursor = pos
}

type BufView struct {
	// TODO: Wrap bool
	Y   int // Y of the view in the Buf, for down/up scrolling
	X   int // X of the view in the Buf, for left/right scrolling
	Buf *Buf
}

func (v *BufView) DrawTo(region Region) {
	bufr := bufio.NewReader(v.Buf.NewReader(false))

	// PgDn/PgUp etc. support
	for y := v.Y; y > 0; y-- {
		line, err := bufr.ReadBytes('\n')
		switch err {
		case nil:
			// skip line
			continue
		case io.EOF:
			bufr = bufio.NewReader(bytes.NewReader(line))
			y = 0
			break
		default:
			panic(err)
		}
	}
	r := tabExpander{r: bufr}

	y := 0
	row := newRowView(v, y, region)
	for {
		ch, _, err := r.ReadRune()
		if y >= region.H {
			break
		} else if err == io.EOF {
			row.EndLine()
			y++
			break
		} else if err != nil {
			panic(err)
		}
		if ch == '\n' {
			row.EndLine()
			y++
			row = newRowView(v, y, region)
			continue
		}
		row.PrintCh(ch)
	}
	for ; y < region.H; y++ {
		newRowView(v, y, region).EndLine()
	}
}

func (v *BufView) HandleKey(ev *tcell.EventKey, scrollY int) bool {
	const scrollX = 8 // When user scrolls horizontally, move by this many characters
	switch getKey(ev) {
	//
	// Vertical scrolling
	//
	case key(tcell.KeyUp):
		v.Y--
		v.normalizeY()
	case key(tcell.KeyDown):
		v.Y++
		v.normalizeY()
	case key(tcell.KeyPgDn):
		// TODO: in top-right corner of Buf area, draw current line number & total # of lines
		v.Y += scrollY
		v.normalizeY()
	case key(tcell.KeyPgUp):
		v.Y -= scrollY
		v.normalizeY()
	//
	// Horizontal scrolling
	//
	case altKey(tcell.KeyLeft),
		ctrlKey(tcell.KeyLeft):
		v.X = max(0, v.X-scrollX)
	case altKey(tcell.KeyRight),
		ctrlKey(tcell.KeyRight):
		v.X += scrollX
	case altKey(tcell.KeyHome),
		ctrlKey(tcell.KeyHome):
		v.X = 0
	default:
		// Unknown key/combination, not handled
		return false
	}
	return true
}

func (v *BufView) normalizeY() {
	nlines := count(v.Buf.NewReader(false), '\n') + 1
	v.Y = max(0, min(nlines-1, v.Y))
}

func count(r io.Reader, b byte) (n int) {
	buf := [256]byte{}
	for {
		i, err := r.Read(buf[:])
		n += bytes.Count(buf[:i], []byte{b})
		if err != nil {
			return
		}
	}
}

type RowView struct {
	w     int
	putch func(x int, ch rune)

	x            int
	overflowLeft bool
	lastRuneW    int
}

func newRowView(v *BufView, y int, region Region) *RowView {
	return &RowView{
		w: region.W,
		putch: func(x int, ch rune) {
			region.SetCell(x, y, tcell.StyleDefault, ch)
		},
		x:         -v.X,
		lastRuneW: 1,
	}
}

func (r *RowView) PrintCh(ch rune) {
	w := max(runewidth.RuneWidth(ch), 1)
	switch {
	case r.overflowLeft && r.x == 0, r.x < 0 && r.x+w > 0:
		r.fill(0, '«', r.x+w)
	case r.x < 0:
		r.overflowLeft = true
	case r.x == r.w:
		r.fill(r.x-r.lastRuneW, '»', r.lastRuneW)
	case r.x < r.w && r.x+w > r.w:
		r.fill(r.x, '»', r.w-r.x)
	default:
		r.putch(r.x, ch)
	}
	r.x += w
	r.lastRuneW = w
}

func (r *RowView) EndLine() {
	xStart := max(0, r.x)
	if xStart == 0 && r.overflowLeft {
		r.putch(0, '«')
		xStart++
	}
	r.fill(xStart, ' ', r.w-xStart)
}

func (r *RowView) fill(x0 int, ch byte, w int) {
	for w > 0 {
		if x0 >= 0 && x0 < r.w {
			r.putch(x0, rune(ch))
		}
		w--
		x0++
	}
}

func NewBuf(bufsize int) *Buf {
	// TODO: make buffer size dynamic (growable by pressing a key)
	buf := &Buf{bytes: make([]byte, bufsize)}
	buf.cond = sync.NewCond(&buf.mu)
	return buf
}

type Buf struct {
	bytes []byte

	mu     sync.Mutex // guards the following fields
	cond   *sync.Cond
	status bufStatus
	n      int
}

type bufStatus int

const (
	bufReading bufStatus = iota
	bufEOF
	bufPaused
)

func (b *Buf) StartCapturing(r io.Reader, notify func()) *Buf {
	go b.capture(r, notify)
	return b
}

func (b *Buf) capture(r io.Reader, notify func()) {
	// TODO: allow stopping - take context?
	for {
		n, err := r.Read(b.bytes[b.n:])

		b.mu.Lock()
		for b.status == bufPaused {
			b.cond.Wait()
		}
		b.n += n
		if err == io.EOF {
			b.status = bufEOF
		}
		if b.n == len(b.bytes) {
			// TODO: remove this when we can grow the buffer
			err = io.EOF
		}
		b.cond.Broadcast()
		b.mu.Unlock()

		go notify()
		if err == io.EOF {
			log.Printf("capture EOF after: %q", b.bytes[:b.n]) // TODO: make sure no race here, and skipped if not debugging
			return
		} else if err != nil {
			// TODO: better handling of errors
			panic(err)
		}
	}
}

func (b *Buf) Pause(pause bool) {
	b.mu.Lock()
	if pause {
		if b.status == bufReading {
			b.status = bufPaused
			// trigger all readers to emit fake EOF
			b.cond.Broadcast()
		}
	} else {
		if b.status == bufPaused {
			b.status = bufReading
			// wake up the capture func
			b.cond.Broadcast()
		}
	}
	b.mu.Unlock()
}

func (b *Buf) DrawStatus(region Region, style tcell.Style) {
	status := '~' // default: still reading input

	b.mu.Lock()
	switch {
	case b.status == bufPaused:
		status = '#'
	case b.status == bufEOF:
		status = ' ' // all input read, nothing more to do
	case b.n == len(b.bytes):
		status = '+' // buffer full
	}
	b.mu.Unlock()

	region.SetCell(0, 0, style, status)
}

func (b *Buf) NewReader(blocking bool) io.Reader {
	i := 0
	return funcReader(func(p []byte) (n int, err error) {
		b.mu.Lock()
		end := b.n
		for blocking && end == i && b.status == bufReading && end < len(b.bytes) {
			b.cond.Wait()
			end = b.n
		}
		b.mu.Unlock()

		n = copy(p, b.bytes[i:end])
		i += n
		if n > 0 {
			return n, nil
		} else {
			if blocking {
				log.Printf("blocking reader emitting EOF after: %q", b.bytes[:end])
			}
			return 0, io.EOF
		}
	})
}

type funcReader func([]byte) (int, error)

func (f funcReader) Read(p []byte) (int, error) { return f(p) }

type Subprocess struct {
	Buf    *Buf
	cancel context.CancelFunc
}

func StartSubprocess(shell []string, command string, stdin *Buf, notify func()) *Subprocess {
	ctx, cancel := context.WithCancel(context.TODO())
	r, w := io.Pipe()
	p := &Subprocess{
		Buf:    NewBuf(len(stdin.bytes)).StartCapturing(r, notify),
		cancel: cancel,
	}

	cmd := exec.CommandContext(ctx, shell[0], append(shell[1:], command)...)
	cmd.Stdout = w
	cmd.Stderr = w
	cmd.Stdin = stdin.NewReader(true)
	err := cmd.Start()
	if err != nil {
		fmt.Fprintf(w, "up: %s", err)
		w.Close()
		return p
	}
	log.Println(cmd.Path)
	go func() {
		err = cmd.Wait()
		if err != nil {
			fmt.Fprintf(w, "up: %s", err)
			log.Printf("Wait returned error: %s", err)
		}
		w.Close()
	}()
	return p
}

func (s *Subprocess) Kill() {
	if s == nil {
		return
	}
	s.cancel()
}

type key int32

func getKey(ev *tcell.EventKey) key { return key(ev.Modifiers())<<16 + key(ev.Key()) }
func altKey(base tcell.Key) key     { return key(tcell.ModAlt)<<16 + key(base) }
func ctrlKey(base tcell.Key) key    { return key(tcell.ModCtrl)<<16 + key(base) }

func writeScript(shell []string, command string, tui tcell.Screen) {
	os.Stderr.WriteString("up: Ultimate Plumber v" + version + " https://github.com/akavel/up\n")
	var f *os.File
	var err error
	if *outputScript != "" {
		os.Stderr.WriteString("up: writing " + *outputScript)
		f, err = os.OpenFile(*outputScript, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
		if err != nil {
			goto fallback_tmp
		}
		goto try_file
	}

	os.Stderr.WriteString("up: writing: .")
	for i := 1; i < 1000; i++ {
		f, err = os.OpenFile(fmt.Sprintf("up%d.sh", i), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0755)
		switch {
		case os.IsExist(err):
			continue
		case err != nil:
			goto fallback_tmp
		default:
			os.Stderr.WriteString("/" + f.Name())
			goto try_file
		}
	}
	os.Stderr.WriteString(" - error: up1.sh-up999.sh already exist\n")
	goto fallback_tmp

try_file:
	// NOTE: currently not supporting multi-word shell in upNNN.sh unfortunately :(
	_, err = fmt.Fprintf(f, "#!%s\n%s\n", shell[0], command)
	if err != nil {
		goto fallback_tmp
	}
	err = f.Close()
	if err != nil {
		goto fallback_tmp
	}
	os.Stderr.WriteString(" - OK\n")
	return

fallback_tmp:
	// TODO: test if the fallbacks etc. protections actually work
	os.Stderr.WriteString(" - error: " + err.Error() + "\n")
	f, err = os.CreateTemp("", "up-*.sh")
	if err != nil {
		goto fallback_print
	}
	_, err = fmt.Fprintf(f, "#!%s\n%s\n", shell, command)
	if err != nil {
		goto fallback_print
	}
	err = f.Close()
	if err != nil {
		goto fallback_print
	}
	os.Stderr.WriteString("up: writing: " + f.Name() + " - OK\n")
	os.Chmod(f.Name(), 0755)
	return

fallback_print:
	fname := "TMP"
	if f != nil {
		fname = f.Name()
	}
	os.Stderr.WriteString("up: writing: " + fname + " - error: " + err.Error() + "\n")
	os.Stderr.WriteString("up: | " + command + "\n")
}

type Region struct {
	W, H    int
	SetCell func(x, y int, style tcell.Style, ch rune)
}

func TuiRegion(tui tcell.Screen, x, y, w, h int) Region {
	return Region{
		W: w, H: h,
		SetCell: func(dx, dy int, style tcell.Style, ch rune) {
			if dx >= 0 && dx < w && dy >= 0 && dy < h {
				if *noColors {
					style = tcell.StyleDefault
				}
				tui.SetCell(x+dx, y+dy, style, ch)
			}
		},
	}
}

var (
	whiteOnBlue  = tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorBlue)
	whiteOnDBlue = tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorNavy)
)

func drawText(region Region, style tcell.Style, text string) {
	for x, ch := range text {
		region.SetCell(x, 0, style, ch)
	}
}

type runeReader interface {
	ReadRune() (r rune, size int, err error)
}

type tabExpander struct {
	r runeReader
	x int // if negative, represents amount of pending spaces left
}

var _ runeReader = (*tabExpander)(nil)

func (t *tabExpander) ReadRune() (r rune, size int, err error) {
	if t.x < 0 {
		t.x++
		return ' ', 1, nil
	}

	r, size, err = t.r.ReadRune()
	if err != nil {
		return 0, 0, err
	}

	const tabWidth = 8
	switch r {
	case '\n', '\r':
		t.x = 0
		return
	case '\t':
		t.x = t.x - tabWidth
		return t.ReadRune()
	default:
		w := runewidth.RuneWidth(r)
		t.x = (t.x + w) % tabWidth
		return
	}
}
