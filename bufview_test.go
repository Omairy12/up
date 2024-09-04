package main

import (
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
		v:    newView(0, 0, `1234567890xyz`),
		want: u.Screen{
			u.Raw("123456789"), u.Raw("»"), u.Endline{0},
			u.Rows{W: 10, H: 9},
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
		want := tt.want.String()
		if have != want {
			t.Errorf("bad %q:\n%s", tt.note, diff.Diff(have, want))
		}
	}
}

func newView(x, y int, text string) BufView {
	v := BufView{X: x, Y: y, Buf: NewBuf(1000)}
	v.Buf.bytes = []byte(text)
	v.Buf.n = len(text)
	return v
}
