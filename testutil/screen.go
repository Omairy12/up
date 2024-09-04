package testutil

import "strings"

type Screen []renderer

func (ts Screen) String() string {
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

// Wide2 represents a two-column wide character.
type Wide2 rune

func (x Wide2) render() string {
	// for multi-width runes, tcell seems to render them as the contents of
	// the first cell, followed by 'X' for each subsequent covered
	// cell/column.
	return string(x) + "X"
}

type Endline struct{ W int }

func (x Endline) render() string { return Empty{x.W}.render() + "\n" }

type Empty struct{ W int }

func (x Empty) render() string { return strings.Repeat(" ", x.W) }

type Rows struct{ W, H int }

func (x Rows) render() string {
	return strings.Repeat(strings.Repeat(" ", x.W)+"\n", x.H)
}