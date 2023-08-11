package main

import "strings"

type TestScreen []renderer

type renderer interface {
	render() string
}

type Wide struct {
	r rune
	w int
}

func (x Wide) render() string {
	// for multi-width runes, tcell seems to render them as the contents of
	// the first cell, followed by 'X' for each covered cell.
	return string(x.r) + strings.Repeat("X", x.w-1)
}

type Empty struct{ w int }

func (x Empty) render() string { return strings.Repeat(" ", x.w) }

type Rows struct{ W, H int }

func (x Rows) render() string {
	return strings.Repeat(strings.Repeat(" ", x.W)+"\n", x.H)
}
