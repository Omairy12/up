package main

import "strings"

type TestScreen []renderer

func (ts TestScreen) String() string {
	s := ""
	for _, r := range ts {
		s += r.render()
	}
	return s
}

type renderer interface {
	render() string
}

type Raw string

func (x Raw) render() string { return string(x) }

type Wide struct {
	r rune
	w int
}

func (x Wide) render() string {
	// for multi-width runes, tcell seems to render them as the contents of
	// the first cell, followed by 'X' for each covered cell.
	return string(x.r) + strings.Repeat("X", x.w-1)
}

type Endline struct{ w int }

func (x Endline) render() string { return Empty{x.w}.render() + "\n" }

type Empty struct{ w int }

func (x Empty) render() string { return strings.Repeat(" ", x.w) }

type Rows struct{ W, H int }

func (x Rows) render() string {
	return strings.Repeat(strings.Repeat(" ", x.W)+"\n", x.H)
}
