package vi

import "testing"

func TestFindWord(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		at        int
		backward  bool
		wantStart int
		wantEnd   int
	}{
		// backward — basic word at end
		{
			name:      "backward: word at end of string",
			input:     "hello world",
			at:        11, // after 'd'
			backward:  true,
			wantStart: 6,
			wantEnd:   10,
		},
		// backward — word in middle
		{
			name:      "backward: word in middle",
			input:     "hello world",
			at:        5, // after 'o' in "hello"
			backward:  true,
			wantStart: 0,
			wantEnd:   4,
		},
		// backward — single word, full string
		{
			name:      "backward: single word full string",
			input:     "hello",
			at:        5,
			backward:  true,
			wantStart: 0,
			wantEnd:   4,
		},
		// backward — at beginning, nothing to find
		{
			name:      "backward: at position 0 returns 0,0",
			input:     "hello",
			at:        0,
			backward:  true,
			wantStart: 0,
			wantEnd:   0,
		},
		// backward — punctuation word
		{
			name:      "backward: punctuation run",
			input:     "foo!!",
			at:        5,
			backward:  true,
			wantStart: 3,
			wantEnd:   4,
		},
		// backward — across punctuation boundary
		{
			name:      "backward: stops at punctuation boundary",
			input:     "foo!!bar",
			at:        8,
			backward:  true,
			wantStart: 5,
			wantEnd:   7,
		},
		// forward — basic word
		{
			name:      "forward: word at start",
			input:     "hello world",
			at:        0,
			backward:  false,
			wantStart: 0,
			wantEnd:   4,
		},
		// forward — word after space
		{
			name:      "forward: word after space",
			input:     "hello world",
			at:        6,
			backward:  false,
			wantStart: 6,
			wantEnd:   10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStart, gotEnd := findWord([]rune(tt.input), tt.at, tt.backward)
			if gotStart != tt.wantStart || gotEnd != tt.wantEnd {
				t.Errorf("findWord(%q, %d, %v) = (%d, %d), want (%d, %d)",
					tt.input, tt.at, tt.backward, gotStart, gotEnd, tt.wantStart, tt.wantEnd)
			}
		})
	}
}
