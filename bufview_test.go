package main

import (
	"bufio"
	"strings"
	"testing"

	u "github.com/akavel/up/testutil"
	"github.com/gdamore/tcell"
	"github.com/kylelemons/godebug/diff"
)

func TestBufView_DrawTo(t *testing.T) {
	type W2 = u.Wide2
	type PadEOL = u.Endline
	EOL := PadEOL{0}

	tests := []struct {
		note string
		v    BufView
		want u.Screen
	}{{
		note: "long line trimmed on the right",
		v:    newView(`123456789_123`),
		want: u.Screen{
			u.Raw("123456789»"), u.Endline{},
		},
	}, {
		note: "long lines trimmed on left & right",
		v: linesView(
			"123456789_123",
			"吃3456789_123",
			"喝茶56789_123",
			"1茶456789_123",
			"1喝茶6789_123").
			scrolled(2, 0),
		want: u.Screen{
			u.Raw("«456789_1»"), EOL,
			u.Raw("«456789_1»"), EOL,
			u.Raw("««56789_1»"), EOL,
			u.Raw("«456789_1»"), EOL,
			u.Raw("«"), W2('茶'), u.Raw("6789_1»"), EOL,
		},
	}, {
		note: "issue #51 Chinese characters",
		v: linesView(
			"吃饭",
			"喝茶",
			"睡觉"),
		want: u.Screen{
			W2('吃'), W2('饭'), PadEOL{6},
			W2('喝'), W2('茶'), PadEOL{6},
			W2('睡'), W2('觉'), PadEOL{6},
		},
	}, {
		note: "Chinese characters trimmed half-way on the left",
		v: linesView(
			"吃3456789_123",
			"喝茶56789_123",
			"1吃456789_123",
			"1喝茶6789_123").
			scrolled(1, 0),
		want: u.Screen{
			u.Raw("«3456789_»"), EOL,
			u.Raw("«"), W2('茶'), u.Raw("56789_»"), EOL,
			u.Raw("««"), u.Raw("456789_»"), EOL,
			u.Raw("««"), W2('茶'), u.Raw("6789_»"), EOL,
		},
	}, {
		note: "Chinese characters trimmed half-way on the right",
		v: linesView(
			"1234567890喝茶bc",
			"123456789喝茶abc",
			"12345678喝茶zabc",
			"1234567喝茶yzabc",
			"123456喝茶xyzabc",
			"12345喝茶0xyzabc",
			"1234喝茶90xyzabc"),
		want: u.Screen{
			u.Raw("123456789»"), EOL,
			u.Raw("123456789»"), EOL,
			u.Raw("12345678»»"), EOL,
			u.Raw("1234567"), W2('喝'), u.Raw("»"), EOL,
			u.Raw("123456"), W2('喝'), u.Raw("»»"), EOL,
			u.Raw("12345"), W2('喝'), W2('茶'), u.Raw("»"), EOL,
			u.Raw("1234"), W2('喝'), W2('茶'), u.Raw("9»"), EOL,
		},
	}, {
		note: "single tabulations",
		v: linesView(
			"\tA",
			"1\tB",
			"1234567\tC",
			"喝\tD"),
		want: u.Screen{
			u.Raw("        A"), PadEOL{1},
			u.Raw("1       B"), PadEOL{1},
			u.Raw("1234567 C"), PadEOL{1},
			W2('喝'), u.Raw("      D"), PadEOL{1},
		},
	}, {
		note: "left-trimmed single tabulations",
		v: linesView(
			"\tA",
			"1\tB",
			"1234567\tC",
			"喝\tD").
			scrolled(3, 0),
		want: u.Screen{
			u.Raw("«    A"), PadEOL{4},
			u.Raw("«    B"), PadEOL{4},
			u.Raw("«567 C"), PadEOL{4},
			u.Raw("«    D"), PadEOL{4},
		},
	}}

	// Initialize simulated tcell.Screen
	sim := tcell.NewSimulationScreen("")
	err := sim.Init()
	if err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()

	reg := Region{
		W: 10, H: 10,
		SetCell: func(dx, dy int, style tcell.Style, ch rune) {
			sim.SetCell(dx, dy, style, ch)
		},
	}

	for _, tt := range tests {
		// Act
		sim.SetSize(reg.W, reg.H)
		sim.Clear()
		tt.v.DrawTo(reg)
		sim.Sync()

		// Assert
		have := u.CellsToString(sim)
		want := padLinesBelow(tt.want.String(), reg)
		if have != want {
			t.Errorf("bad %q:\n%s", tt.note, diff.Diff(have, want))
		}
	}
}

func linesView(lines ...string) BufView {
	return newView(strings.Join(lines, "\n"))
}

func newView(text string) BufView {
	v := BufView{Buf: NewBuf(1000)}
	v.Buf.bytes = []byte(text)
	v.Buf.n = len(text)
	return v
}

func (v BufView) scrolled(x, y int) BufView {
	v.X = x
	v.Y = y
	return v
}

func padLinesBelow(screen string, reg Region) string {
	var (
		n        = strings.Count(screen, "\n")
		emptyRow = strings.Repeat(" ", reg.W) + "\n"
		padding  = strings.Repeat(emptyRow, reg.H-n)
	)
	return screen + padding
}

func Test_tabExpander(t *testing.T) {
	lines := func(s ...string) string { return strings.Join(s, "\n") }
	tests := []struct {
		in   string
		want string
	}{{
		in:   `abc`,
		want: `abc`,
	}, {
		in: lines(
			"\ta\tb",
			"\tc"),
		want: lines(
			"        a       b",
			"        c"),
	}, {
		in:   "\t\ta\tb",
		want: "                a       b",
	}, {
		in:   "abc\ndef",
		want: "abc\ndef",
	}, {
		in:   "abc\ndef\n",
		want: "abc\ndef\n",
	}}

	for _, tt := range tests {
		r := tabExpander{r: bufio.NewReader(strings.NewReader(tt.in))}
		out := []string{}
		for {
			ch, _, err := r.ReadRune()
			if err != nil {
				break
			}
			out = append(out, string(ch))
		}
		have := strings.Join(out, "")
		if have != tt.want {
			t.Errorf("bad output\nIN: %q\nHAVE: %q\nWANT: %q",
				tt.in, have, tt.want)
		}
	}
}
