package main

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell"
	"github.com/kylelemons/godebug/diff"
)

func Test_BufView_DrawTo(t *testing.T) {
	tests := []struct {
		comment string
		v       BufView
		want    TestScreen
	}{
		{
			comment: "long line",
			v:       newBufView(`1234567890xyz`),
			want: TestScreen{
				Raw("123456789"), Raw("»"), Endline{0},
				Rows{W: 10, H: 9},
			},
		},
		{
			comment: "Chinese characters - issue #51",
			v: newBufView(`吃饭
喝茶
睡觉`),
			want: TestScreen{
				Wide{'吃', 2}, Wide{'饭', 2}, Endline{8},
				Wide{'喝', 2}, Wide{'茶', 2}, Endline{8},
				Wide{'睡', 2}, Wide{'觉', 2}, Endline{8},
				Rows{W: 10, H: 7},
			},
		},
	}

	scr := tcell.NewSimulationScreen("")
	err := scr.Init()
	if err != nil {
		t.Fatal(err)
	}
	defer scr.Fini()

	region := Region{
		W: 10,
		H: 10,
		SetCell: func(x, y int, style tcell.Style, ch rune) {
			scr.SetCell(x, y, style, ch)
		},
		// SetContent: func(dx, dy int, mainc rune, combc []rune
	}

	for _, tt := range tests {
		scr.SetSize(region.W, region.H)
		scr.Clear()
		tt.v.DrawTo(region)
		scr.Sync()

		haveCells, haveW, _ := scr.GetContents()
		have := renderCells(haveCells, haveW)

		want := tt.want.String()
		if have != want {
			t.Errorf("bad %q:\n%s", tt.comment, diff.Diff(have, want))
		}
	}
}

func newBufView(text string) BufView {
	v := BufView{Buf: NewBuf(1000)}
	v.Buf.bytes = []byte(text)
	v.Buf.n = len(text)
	return v
}

func renderCells(cells []tcell.SimCell, w int) string {
	s := ""
	for len(cells) >= w {
		row := cells[:w]
		cells = cells[w:]
		// Trim empty cells at the end of the row
		for len(row) > 0 && len(row[len(row)-1].Bytes) == 0 {
			row = row[:len(row)-1]
		}
		for _, c := range row {
			s += string(c.Bytes)
		}
		s += "\n"
	}
	// FIXME: append any trailing cells' bytes as well
	return strings.TrimRight(s, "\n") + "\n"
}
