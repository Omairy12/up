package main

import "testing"

func Test_Editor_insert(t *testing.T) {
	type runes = []rune
	tests := []struct {
		comment   string
		e         Editor
		insert    []rune
		wantValue string
	}{
		{
			comment: "prepend ASCII char",
			e: Editor{
				value:  runes(`abc`),
				cursor: 0,
			},
			insert:    runes{'X'},
			wantValue: `Xabc`,
		},
		{
			comment: "prepend UTF char",
			e: Editor{
				value:  runes(`abc`),
				cursor: 0,
			},
			insert:    runes{'☃'},
			wantValue: `☃abc`,
		},
		{
			comment: "insert ASCII char",
			e: Editor{
				value:  runes(`abc`),
				cursor: 1,
			},
			insert:    runes{'X'},
			wantValue: `aXbc`,
		},
		{
			comment: "insert UTF char",
			e: Editor{
				value:  runes(`abc`),
				cursor: 1,
			},
			insert:    runes{'☃'},
			wantValue: `a☃bc`,
		},
		{
			comment: "append ASCII char",
			e: Editor{
				value:  runes(`abc`),
				cursor: 3,
			},
			insert:    runes{'X'},
			wantValue: `abcX`,
		},
		{
			comment: "append UTF char",
			e: Editor{
				value:  runes(`abc`),
				cursor: 3,
			},
			insert:    runes{'☃'},
			wantValue: `abc☃`,
		},
		{
			comment: "insert 2 ASCII chars",
			e: Editor{
				value:  runes(`abc`),
				cursor: 1,
			},
			insert:    runes{'X', 'Y'},
			wantValue: `aXYbc`,
		},
	}

	for _, tt := range tests {
		tt.e.insert(tt.insert...)
		if string(tt.e.value) != tt.wantValue {
			t.Errorf("%q: bad value\nwant: %q\nhave: %q", tt.comment, runes(tt.wantValue), tt.e.value)
		}
	}
}

func Test_Editor_unix_word_rubout(t *testing.T) {
	type runes = []rune
	tests := []struct {
		comment       string
		e             Editor
		wantValue     string
		wantKillspace string
	}{
		{
			comment: "at beginning of line",
			e: Editor{
				value:  runes(`abc`),
				cursor: 0,
			},
			wantValue:     `abc`,
			wantKillspace: ``,
		},
		{
			comment: "at soft beginning of line",
			e: Editor{
				value:  runes(` abc`),
				cursor: 1,
			},
			wantValue:     `abc`,
			wantKillspace: ` `,
		},
		{
			comment: "until soft beginning of line",
			e: Editor{
				value:  runes(` abc`),
				cursor: 2,
			},
			wantValue:     ` bc`,
			wantKillspace: `a`,
		},
		{
			comment: "until beginning of line",
			e: Editor{
				value:  runes(`abc`),
				cursor: 2,
			},
			wantValue:     `c`,
			wantKillspace: `ab`,
		},
		{
			comment: "in middle of line",
			e: Editor{
				value:  runes(`lorem ipsum dolor`),
				cursor: 11,
			},
			wantValue:     `lorem  dolor`,
			wantKillspace: `ipsum`,
		},
		{
			comment: "cursor at beginning of word",
			e: Editor{
				value:  runes(`lorem ipsum dolor`),
				cursor: 12,
			},
			wantValue:     `lorem dolor`,
			wantKillspace: `ipsum `,
		},
		{
			comment: "cursor between multiple spaces",
			e: Editor{
				value:  runes(`a b   c`),
				cursor: 5,
			},
			wantValue:     `a  c`,
			wantKillspace: `b  `,
		},
		{
			comment: "at tab as space char (although is it a realistic case in the context of a command line instruction?)",
			e: Editor{
				value:  runes(`a b		c`),
				cursor: 5,
			},
			wantValue:     `a c`,
			wantKillspace: `b		`,
		},
	}

	for _, tt := range tests {
		tt.e.unixWordRubout()
		if string(tt.e.value) != tt.wantValue {
			t.Errorf("%q: bad value\nwant: %q\nhave: %q", tt.comment, runes(tt.wantValue), tt.e.value)
		}
		if string(tt.e.killspace) != tt.wantKillspace {
			t.Errorf("%q: bad value in killspace\nwant: %q\nhave: %q", tt.comment, runes(tt.wantKillspace), tt.e.value)
		}
	}
}
