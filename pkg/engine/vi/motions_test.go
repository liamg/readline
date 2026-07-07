package vi

import "testing"

func TestVi_NormalMode_WordMotions(t *testing.T) {
	tests := []struct {
		name       string
		buffer     string
		cursor     int
		keys       string
		wantCursor int
	}{
		{name: "w: start of first word", buffer: "one two three", cursor: 0, keys: "w", wantCursor: 4},
		{name: "w: middle of first word", buffer: "one two three", cursor: 1, keys: "w", wantCursor: 4},
		{name: "w: function call", buffer: "function(arg)", cursor: 0, keys: "w", wantCursor: 8},
		{name: "w: function call from param", buffer: "function(arg)", cursor: 8, keys: "w", wantCursor: 9},
		{name: "w: function call from arg", buffer: "function(arg)", cursor: 9, keys: "w", wantCursor: 12},
		{name: "b: middle of word", buffer: "one two three", cursor: 1, keys: "b", wantCursor: 0},
		{name: "b: beginning of next word", buffer: "one two three", cursor: 4, keys: "b", wantCursor: 0},
		{name: "b: function arg", buffer: "function(arg)", cursor: 9, keys: "b", wantCursor: 8},
		{name: "e: beginning of first word", buffer: "one two three", cursor: 0, keys: "e", wantCursor: 2},
		{name: "e: end of first word", buffer: "one two three", cursor: 2, keys: "e", wantCursor: 6},
		{name: "W", buffer: "one,two three", cursor: 0, keys: "W", wantCursor: 8},
		{name: "B", buffer: "one,two three", cursor: 8, keys: "B", wantCursor: 0},
		{name: "E", buffer: "one,two three", cursor: 0, keys: "E", wantCursor: 6},
		{name: "ge", buffer: "one two", cursor: 4, keys: "ge", wantCursor: 2},
		{name: "ge: with symbols", buffer: "one(lol) two", cursor: 10, keys: "ge", wantCursor: 7},
		{name: "ge: with symbols 2", buffer: "one(lol) two", cursor: 7, keys: "ge", wantCursor: 6},
		{name: "ge: with symbols 3", buffer: "one(lol) two", cursor: 6, keys: "ge", wantCursor: 3},
		{name: "gE", buffer: "one,two three", cursor: 8, keys: "gE", wantCursor: 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ed, eng, _ := normalEngine(t, tt.buffer)
			moveCursorTo(ed, tt.cursor)
			mustHandleRunes(t, eng, tt.keys)
			if got := ed.Cursor(); got != tt.wantCursor {
				t.Fatalf("cursor = %d, want %d", got, tt.wantCursor)
			}
		})
	}
}

func TestVi_NormalMode_TextObjects(t *testing.T) {
	tests := []struct {
		name   string
		buffer string
		cursor int
		keys   string
		want   string
	}{
		{name: "diw", buffer: "alpha beta gamma", cursor: 6, keys: "diw", want: "alpha  gamma"},
		{name: "daw", buffer: "alpha beta gamma", cursor: 6, keys: "daw", want: "alpha gamma"},
		{name: "diW", buffer: "alpha one,two gamma", cursor: 6, keys: "diW", want: "alpha  gamma"},
		{name: "daW", buffer: "alpha one,two gamma", cursor: 6, keys: "daW", want: "alpha gamma"},
		{name: `di"`, buffer: `say "hello" now`, cursor: 6, keys: `di"`, want: `say "" now`},
		{name: `da"`, buffer: `say "hello" now`, cursor: 6, keys: `da"`, want: `say  now`},
		{name: "di'", buffer: "say 'hello' now", cursor: 6, keys: "di'", want: "say '' now"},
		{name: "da'", buffer: "say 'hello' now", cursor: 6, keys: "da'", want: "say  now"},
		{name: "di`", buffer: "say `hello` now", cursor: 6, keys: "di`", want: "say `` now"},
		{name: "da`", buffer: "say `hello` now", cursor: 6, keys: "da`", want: "say  now"},
		{name: "di(", buffer: "call(foo) now", cursor: 5, keys: "di(", want: "call() now"},
		{name: "da(", buffer: "call(foo) now", cursor: 5, keys: "da(", want: "call now"},
		{name: "dib", buffer: "call(foo) now", cursor: 5, keys: "dib", want: "call() now"},
		{name: "dab", buffer: "call(foo) now", cursor: 5, keys: "dab", want: "call now"},
		{name: "di[", buffer: "list[foo] now", cursor: 5, keys: "di[", want: "list[] now"},
		{name: "da[", buffer: "list[foo] now", cursor: 5, keys: "da[", want: "list now"},
		{name: "di{", buffer: "map{foo} now", cursor: 4, keys: "di{", want: "map{} now"},
		{name: "da{", buffer: "map{foo} now", cursor: 4, keys: "da{", want: "map now"},
		{name: "diB", buffer: "map{foo} now", cursor: 4, keys: "diB", want: "map{} now"},
		{name: "daB", buffer: "map{foo} now", cursor: 4, keys: "daB", want: "map now"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ed, eng, _ := normalEngine(t, tt.buffer)
			moveCursorTo(ed, tt.cursor)
			mustHandleRunes(t, eng, tt.keys)
			if got := ed.BufferString(); got != tt.want {
				t.Fatalf("buffer = %q, want %q", got, tt.want)
			}
		})
	}
}
