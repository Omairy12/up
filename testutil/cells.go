package testutil

import (
	"strings"

	"github.com/gdamore/tcell"
)

type SimCellsGetter interface {
	GetContents() (cells []tcell.SimCell, width, height int)
}

func CellsToString(sim SimCellsGetter) string {
	cells, w, _ := sim.GetContents()
	s := ""
	for len(cells) > 0 {
		n := min(w, len(cells))
		row, rest := cells[:w], cells[w:]
		cells = rest
		// Trim empty cells at the end of the row
		for n > 0 && len(row[n-1].Bytes) == 0 {
			n--
		}
		row = row[:n]
		// Convert row to string
		for _, c := range row {
			s += string(c.Bytes)
		}
		s += "\n"
	}
	return strings.TrimRight(s, "\n") + "\n"
}
