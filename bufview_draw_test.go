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
			comment: "long line trimmed on the right",
			v:       newBufView(0, 0, `1234567890xyz`),
			want: TestScreen{
				Raw("123456789"), Raw("»"), Endline{0},
				Rows{W: 10, H: 9},
			},
		},
		{
			comment: "long lines trimmed on left & right",
			v: newBufView(2, 0, ""+
				"1234567890xyz\n"+
				"吃34567890xyz\n"+
				"喝茶567890xyz\n"+
				"1茶4567890xyz\n"+
				"1喝茶67890xyz"),
			want: TestScreen{
				Raw("«"), Raw("4567890x"), Raw("»"), Endline{0},
				Raw("«"), Raw("4567890x"), Raw("»"), Endline{0},
				Raw("««"), Raw("567890x"), Raw("»"), Endline{0},
				Raw("«"), Raw("4567890x"), Raw("»"), Endline{0},
				Raw("«"), Wide{'茶', 2}, Raw("67890x"), Raw("»"), Endline{0},
				Rows{W: 10, H: 5},
			},
		},
		{
			comment: "Chinese characters - issue #51",
			v: newBufView(0, 0, `吃饭
喝茶
睡觉`),
			want: TestScreen{
				Wide{'吃', 2}, Wide{'饭', 2}, Endline{6},
				Wide{'喝', 2}, Wide{'茶', 2}, Endline{6},
				Wide{'睡', 2}, Wide{'觉', 2}, Endline{6},
				Rows{W: 10, H: 7},
			},
		},
		{
			comment: "Chinese characters trimmed half-way on the left",
			v: newBufView(1, 0, ""+
				"吃34567890xyz\n"+
				"喝茶567890xyz\n"+
				"1吃4567890xyz\n"+
				"1喝茶67890xyz"),
			want: TestScreen{
				Raw("«"), Raw("34567890"), Raw("»"), Endline{0},
				Raw("«"), Wide{'茶', 2}, Raw("567890"), Raw("»"), Endline{0},
				Raw("««"), Raw("4567890"), Raw("»"), Endline{0},
				Raw("««"), Wide{'茶', 2}, Raw("67890"), Raw("»"), Endline{0},
				Rows{W: 10, H: 6},
			},
		},
		{
			comment: "Chinese characters trimmed half-way on the right",
			v: newBufView(0, 0, ""+
				"1234567890喝茶bc\n"+
				"123456789喝茶abc\n"+
				"12345678喝茶zabc\n"+
				"1234567喝茶yzabc\n"+
				"123456喝茶xyzabc\n"+
				"12345喝茶0xyzabc\n"+
				"1234喝茶90xyzabc\n"),
			want: TestScreen{
				Raw("123456789"), Raw("»"), Endline{0},
				Raw("123456789"), Raw("»"), Endline{0},
				Raw("12345678"), Raw("»»"), Endline{0},
				Raw("1234567"), Wide{'喝', 2}, Raw("»"), Endline{0},
				Raw("123456"), Wide{'喝', 2}, Raw("»»"), Endline{0},
				Raw("12345"), Wide{'喝', 2}, Wide{'茶', 2}, Raw("»"), Endline{0},
				Raw("1234"), Wide{'喝', 2}, Wide{'茶', 2}, Raw("9»"), Endline{0},
				Rows{W: 10, H: 3},
			},
		},
		{
			comment: "single tabulations",
			v: newBufView(0, 0, ""+
				"\tA\n"+
				"1\tB\n"+
				"1234567\tC\n"+
				"喝\tD"),
			want: TestScreen{
				Raw("        A"), Endline{1},
				Raw("1       B"), Endline{1},
				Raw("1234567 C"), Endline{1},
				Wide{'喝', 2}, Raw("      D"), Endline{1},
				Rows{W: 10, H: 6},
			},
		},
		{
			comment: "left-trimmed single tabulations",
			v: newBufView(3, 0, ""+
				"\tA\n"+
				"1\tB\n"+
				"1234567\tC"),
			want: TestScreen{
				Raw("«"), Raw("    A"), Endline{4},
				Raw("«"), Raw("    B"), Endline{4},
				Raw("«"), Raw("567 C"), Endline{4},
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
		SetContent: func(x, y int, mainc rune, combc []rune, style tcell.Style) {
			scr.SetContent(x, y, mainc, combc, style)
		},
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

func newBufView(x, y int, text string) BufView {
	v := BufView{X: x, Y: y, Buf: NewBuf(1000)}
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
