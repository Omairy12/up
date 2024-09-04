package main

import "testing"

func Test_Editor_insert(t *testing.T) {
	type runes = []rune
	tests := []struct {
		note      string
		e         Editor
		insert    []rune
		wantValue string
	}{{
		note:      "prepend ASCII char",
		e:         edBetween(``, `abc`),
		insert:    runes{'X'},
		wantValue: `Xabc`,
	}, {
		note:      "prepend UTF char",
		e:         edBetween(``, `abc`),
		insert:    runes{'☃'},
		wantValue: `☃abc`,
	}, {
		note:      "insert ASCII char",
		e:         edBetween(`a`, `bc`),
		insert:    runes{'X'},
		wantValue: `aXbc`,
	}, {
		note:      "insert UTF char",
		e:         edBetween(`a`, `bc`),
		insert:    runes{'☃'},
		wantValue: `a☃bc`,
	}, {
		note:      "append ASCII char",
		e:         edBetween(`abc`, ``),
		insert:    runes{'X'},
		wantValue: `abcX`,
	}, {
		note:      "append UTF char",
		e:         edBetween(`abc`, ``),
		insert:    runes{'☃'},
		wantValue: `abc☃`,
	}, {
		note:      "insert 2 ASCII chars",
		e:         edBetween(`a`, `bc`),
		insert:    runes{'X', 'Y'},
		wantValue: `aXYbc`,
	}}

	for _, tt := range tests {
		tt.e.insert(tt.insert...)
		if string(tt.e.value) != tt.wantValue {
			t.Errorf("%q: bad value\nwant: %q\nhave: %q", tt.note, runes(tt.wantValue), tt.e.value)
		}
	}
}

func Test_Editor_unix_word_rubout(t *testing.T) {
	type runes = []rune
	tests := []struct {
		note          string
		e             Editor
		wantValue     string
		wantKillspace string
	}{{
		note:          "at beginning of line",
		e:             edBetween(``, `abc`),
		wantValue:     `abc`,
		wantKillspace: ``,
	}, {
		note:          "at soft beginning of line",
		e:             edBetween(` `, `abc`),
		wantValue:     `abc`,
		wantKillspace: ` `,
	}, {
		note:          "until soft beginning of line",
		e:             edBetween(` a`, `bc`),
		wantValue:     ` bc`,
		wantKillspace: `a`,
	}, {
		note:          "until beginning of line",
		e:             edBetween(`ab`, `c`),
		wantValue:     `c`,
		wantKillspace: `ab`,
	}, {
		note:          "in middle of line",
		e:             edBetween(`lorem ipsum`, ` dolor`),
		wantValue:     `lorem  dolor`,
		wantKillspace: `ipsum`,
	}, {
		note:          "cursor at beginning of word",
		e:             edBetween(`lorem ipsum `, `dolor`),
		wantValue:     `lorem dolor`,
		wantKillspace: `ipsum `,
	}, {
		note:          "cursor between multiple spaces",
		e:             edBetween(`a b  `, ` c`),
		wantValue:     `a  c`,
		wantKillspace: `b  `,
	}, {
		note:          "at tab as space char (although is it a realistic case in the context of a command line instruction?)",
		e:             edBetween(`a b		`, `c`),
		wantValue:     `a c`,
		wantKillspace: `b		`,
	}}

	for _, tt := range tests {
		tt.e.unixWordRubout()
		if string(tt.e.value) != tt.wantValue {
			t.Errorf("%q: bad value\nwant: %q\nhave: %q", tt.note, runes(tt.wantValue), tt.e.value)
		}
		if string(tt.e.killspace) != tt.wantKillspace {
			t.Errorf("%q: bad value in killspace\nwant: %q\nhave: %q", tt.note, runes(tt.wantKillspace), tt.e.value)
		}
	}
}

func edBetween(beforeCursor, afterCursor string) Editor {
	return Editor{
		value:  []rune(beforeCursor + afterCursor),
		cursor: len(beforeCursor),
	}
}
