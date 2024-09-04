package main

import (
	"strings"
	"testing"

	u "github.com/akavel/up/testutil"
	"github.com/gdamore/tcell"
	"github.com/kylelemons/godebug/diff"
)

func TestBufView_DrawTo(t *testing.T) {
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
			"123456789.xyz").
			scrolled(2, 0),
		want: u.Screen{
			u.Raw("«456789_1»"), u.Endline{},
			u.Raw("«456789.x»"), u.Endline{},
		},
	}, {
		note: "single tabulations",
		v: linesView(
			"\tA",
			"1\tB",
			"1234567\tC"),
		want: u.Screen{
			u.Raw("        A"), u.Endline{1},
			u.Raw("1       B"), u.Endline{1},
			u.Raw("1234567 C"), u.Endline{1},
		},
	}, {
		note: "left-trimmed single tabulations",
		v: linesView(
			"\tA",
			"1\tB",
			"1234567\tC").
			scrolled(3, 0),
		want: u.Screen{
			u.Raw("«    A"), u.Endline{4},
			u.Raw("«    B"), u.Endline{4},
			u.Raw("«567 C"), u.Endline{4},
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
