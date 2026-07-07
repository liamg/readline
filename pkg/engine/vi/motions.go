package vi

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/liamg/readline/pkg/engine"
	"github.com/liamg/readline/pkg/terminal"
	"github.com/rivo/uniseg"
)

type OpFunc func(start, end, count int, unknown rune) (engine.ActionResult, error)

type MotionFunc func(cursor int, buffer []rune, count int) (int, int, error)

func (v *Vi) applyOpWithMotion(op OpFunc) engine.ActionFunc {
	motionCount := 0
	var self engine.ActionFunc
	self = func(c *engine.ActionContext) (engine.ActionResult, error) {
		ev := c.LastKey()
		if ev.Key == terminal.KeyRune && ev.Rune >= '1' && ev.Rune <= '9' {
			motionCount = motionCount*10 + int(ev.Rune-'0')
			return engine.ActionResult{Next: self}, nil // keep waiting
		}
		if ev.Key == terminal.KeyRune && ev.Rune == '0' && motionCount > 0 {
			motionCount *= 10
			return engine.ActionResult{Next: self}, nil
		}
		mc := motionCount
		if mc == 0 {
			mc = 1
		}
		total := v.state.Count()
		v.state.ResetCount()
		if ev.Key != terminal.KeyRune || ev.Mod != 0 {
			return engine.ActionResult{}, fmt.Errorf("expected motion, received %q", ev)
		}

		cursor := c.Editor.Cursor()
		buffer := c.Editor.Buffer()

		var motion MotionFunc
		inclusive := false

		switch ev.Rune {
		case '0': // beginning of line to cursor
			motion = motionLineStart
		case '^': // first non blank char to cursor
			motion = motionLineFirstNonBlank
		case '$': // cursor to end of line
			motion = motionLineEnd
			inclusive = true
		case 'w':
			motion = motionToStartOfWordForward
		case 'b':
			motion = motionToStartOfWordBackward
		case 'e':
			motion = motionToEndOfWordForward
			inclusive = true
		case 'W':
			motion = motionToStartOfNonWhitespaceForward
		case 'B':
			motion = motionToStartOfNonWhitespaceBackward
		case 'E':
			motion = motionToEndOfNonWhitespaceForward
			inclusive = true
		case 'g': // ge and gE
			return engine.ActionResult{
				Next: func(ac *engine.ActionContext) (engine.ActionResult, error) {
					key := ac.LastKey()
					if key.Key != terminal.KeyRune || key.Mod != 0 {
						return engine.ActionResult{}, fmt.Errorf("expected motion after 'g', got %q", key)
					}
					switch key.Rune {
					case 'e':
						motion = motionToEndOfWordBackward
					case 'E':
						motion = motionToEndOfNonWhitespaceBackward
					default:
						return engine.ActionResult{}, fmt.Errorf("unexpected motion %q after 'g'", key.Rune)
					}

					start, end, err := motion(cursor, buffer, mc)
					if err != nil {
						return engine.ActionResult{}, err
					}

					if start > end {
						start, end = end, start
					}
					if end > cursor {
						end++
					}
					return op(start, end, total, 0)
				},
			}, nil
		case 'f':
			return motionWithNextCharacter(motionFindCharacterForward, op, mc), nil
		case 'F':
			return motionWithNextCharacter(motionFindCharacterBackward, op, mc), nil
		case 't':
			return motionWithNextCharacter(motionUntilCharacterForward, op, mc), nil
		case 'T':
			return motionWithNextCharacter(motionUntilCharacterBackward, op, mc), nil
		case 'i', 'a':
			return engine.ActionResult{
				Next: func(ac *engine.ActionContext) (engine.ActionResult, error) {
					key := ac.LastKey()
					if key.Key != terminal.KeyRune || key.Mod != 0 {
						return engine.ActionResult{}, fmt.Errorf("expected text object signifier, got %q", key)
					}
					start, end, err := textObject(buffer, cursor, key.Rune, ev.Rune == 'a')
					if err != nil {
						return engine.ActionResult{}, err
					}
					if start > end {
						start, end = end, start
					}
					return op(start, end, total, 0)
				},
			}, nil
		default:
			// this motion was not understood, so it may be operator specific, like the second `y` in `yy`.
			// give it back to the operator handler to deal with
			return op(0, 0, total, ev.Rune)
		}

		start, end, err := motion(cursor, buffer, mc)
		if err != nil {
			return engine.ActionResult{}, err
		}

		if start > end {
			start, end = end, start
		}
		if inclusive && end > cursor && end < len(buffer) {
			end = nextGraphemeBoundary(buffer, end)
		}
		return op(start, end, total, 0)
	}
	return self
}

func textObject(buffer []rune, cursor int, opener rune, outer bool) (int, int, error) {
	var closer rune
	switch opener {
	case 'w':
		return getWord(buffer, cursor, outer)
	case 'W':
		return getNonWS(buffer, cursor, outer)
	case '{':
		closer = '}'
	case 'B':
		opener = '{'
		closer = '}'
	case '(':
		closer = ')'
	case 'b':
		opener = '('
		closer = ')'
	case '[':
		closer = ']'
	case '`', '"', '\'':
		closer = opener
	default:
		return 0, 0, fmt.Errorf("unsupported text object rune %q", opener)
	}
	return getInside(buffer, cursor, opener, closer, outer)
}

func getInside(buffer []rune, cursor int, opener, closer rune, outer bool) (int, int, error) {
	// find the closest opener before the cursor
	openIndex := -1
	for i := cursor; i >= 0; i-- {
		if buffer[i] == opener {
			openIndex = i
			break
		}
		if buffer[i] == '\n' {
			break
		}
	}

	if openIndex == -1 {
		for i := cursor; i < len(buffer); i++ {
			if buffer[i] == opener {
				openIndex = i
				break
			}
			if buffer[i] == '\n' {
				break
			}
		}

		if openIndex == -1 {
			return 0, 0, fmt.Errorf("could not find opener %q for text object", opener)
		}
	}

	// find the closer after the opener
	closeIndex := -1
	for i := openIndex + 1; i < len(buffer); i++ {
		if buffer[i] == closer {
			closeIndex = i
			break
		}
	}

	if closeIndex == -1 {
		return 0, 0, fmt.Errorf("could not find closer %q for text object", closer)
	}

	if outer {
		return openIndex, closeIndex + 1, nil
	}

	return openIndex + 1, closeIndex, nil
}

func getWord(buffer []rune, cursor int, outer bool) (int, int, error) {
	segs := graphemeSegments(buffer)
	if len(segs) == 0 {
		return 0, 0, nil
	}
	idx := graphemeSegmentIndexAtCursor(segs, cursor)
	if idx >= len(segs) {
		return len(buffer), len(buffer), nil
	}
	typ := graphemeSegmentWordType(segs[idx])
	if typ == WordTypeWhitespace {
		return segs[idx].start, segs[idx].end, nil
	}

	start := segs[idx].start
	end := segs[idx].end
	for i := idx - 1; i >= 0 && graphemeSegmentWordType(segs[i]) == typ; i-- {
		start = segs[i].start
	}
	endIdx := idx
	for i := idx + 1; i < len(segs) && graphemeSegmentWordType(segs[i]) == typ; i++ {
		end = segs[i].end
		endIdx = i
	}

	if !outer {
		return start, end, nil
	}

	for i := endIdx + 1; i < len(segs) && graphemeSegmentWordType(segs[i]) == WordTypeWhitespace; i++ {
		end = segs[i].end
	}
	return start, end, nil
}

func getNonWS(buffer []rune, cursor int, outer bool) (int, int, error) {
	segs := graphemeSegments(buffer)
	if len(segs) == 0 {
		return 0, 0, nil
	}
	idx := graphemeSegmentIndexAtCursor(segs, cursor)
	if idx >= len(segs) {
		return len(buffer), len(buffer), nil
	}
	if isWhitespaceString(segs[idx].text) {
		return segs[idx].start, segs[idx].end, nil
	}

	start := segs[idx].start
	end := segs[idx].end
	endIdx := idx
	for i := idx - 1; i >= 0 && !isWhitespaceString(segs[i].text); i-- {
		start = segs[i].start
		endIdx = i
	}
	for i := idx + 1; i < len(segs) && !isWhitespaceString(segs[i].text); i++ {
		end = segs[i].end
		endIdx = i
	}

	if !outer {
		return start, end, nil
	}

	for i := endIdx + 1; i < len(segs) && isWhitespaceString(segs[i].text); i++ {
		end = segs[i].end
	}
	return start, end, nil
}

func motionWithNextCharacter(motionFactory func(rune) MotionFunc, op OpFunc, count int) engine.ActionResult {
	return engine.ActionResult{
		Next: func(ac *engine.ActionContext) (engine.ActionResult, error) {
			key := ac.LastKey()
			if key.Key != terminal.KeyRune || key.Mod != 0 {
				return engine.ActionResult{}, fmt.Errorf("expected character for motion, got %q", key)
			}

			buffer := ac.Editor.Buffer()
			start, end, err := motionFactory(key.Rune)(ac.Editor.Cursor(), buffer, count)
			if err != nil {
				return engine.ActionResult{}, err
			}
			cursor := ac.Editor.Cursor()
			if end > cursor && end < len(buffer) {
				end = nextGraphemeBoundary(buffer, end)
			}
			return op(start, end, 0, 0)
		},
	}
}

func motionLineStart(cursor int, buffer []rune, count int) (int, int, error) {
	before := buffer[:cursor]
	index := lastIndex(before, '\n')
	if index > -1 {
		return index + 1, cursor, nil
	}
	// TODO: handle multiple lines when count is set
	return 0, cursor, nil
}

func motionLineEnd(cursor int, buffer []rune, count int) (int, int, error) {
	before := buffer[:cursor]
	after := buffer[cursor:]
	index := slices.Index(after, '\n')
	if index > -1 {
		return cursor, index + len(before), nil
	}
	// TODO: handle multiple lines when count is set
	return cursor, len(buffer), nil
}

func motionLineFirstNonBlank(cursor int, buffer []rune, count int) (int, int, error) {
	before := buffer[:cursor]
	index := lastIndex(before, '\n') + 1
	for i := index; i < len(buffer); i++ {
		if !isWhitespaceChar(buffer[i]) {
			return i, cursor, nil
		}
	}
	// TODO: handle multiple lines when count is set
	return cursor, len(buffer), nil
}

func motionLineEntire(cursor int, buffer []rune, count int) (int, int, error) {
	start, _, err := motionLineStart(cursor, buffer, 1)
	if err != nil {
		return 0, 0, err
	}

	if count <= 0 {
		count = 1
	}

	end := start
	for range count {
		if end >= len(buffer) {
			break
		}
		nextNewline := slices.Index(buffer[end:], '\n')
		if nextNewline == -1 {
			end = len(buffer)
			break
		}
		end += nextNewline + 1
	}

	return start, end, nil
}

func motionFindCharacterForward(r rune) MotionFunc {
	return func(cursor int, buffer []rune, count int) (int, int, error) {
		if count < 1 {
			count = 1
		}
		segs := graphemeSegments(buffer)
		found := 0
		for i := graphemeSegmentIndexAtCursor(segs, cursor) + 1; i < len(segs); i++ {
			if graphemeSegmentContainsRune(segs[i], r) {
				if found++; found == count {
					return cursor, segs[i].start, nil
				}
			}
		}
		return cursor, cursor, nil
	}
}

func motionFindCharacterBackward(r rune) MotionFunc {
	return func(cursor int, buffer []rune, count int) (int, int, error) {
		if count < 1 {
			count = 1
		}
		segs := graphemeSegments(buffer)
		found := 0
		for i := graphemeSegmentIndexAtCursor(segs, cursor) - 1; i >= 0; i-- {
			if graphemeSegmentContainsRune(segs[i], r) {
				if found++; found == count {
					return segs[i].start, cursor, nil
				}
			}
		}
		return cursor, cursor, nil
	}
}

func motionUntilCharacterForward(r rune) MotionFunc {
	return func(cursor int, buffer []rune, count int) (int, int, error) {
		if count < 1 {
			count = 1
		}
		segs := graphemeSegments(buffer)
		found := 0
		for i := graphemeSegmentIndexAtCursor(segs, cursor) + 1; i < len(segs); i++ {
			if graphemeSegmentContainsRune(segs[i], r) {
				if found++; found == count {
					return cursor, segs[i].start - 1, nil
				}
			}
		}
		return cursor, cursor, nil
	}
}

func motionUntilCharacterBackward(r rune) MotionFunc {
	return func(cursor int, buffer []rune, count int) (int, int, error) {
		if count < 1 {
			count = 1
		}
		segs := graphemeSegments(buffer)
		found := 0
		for i := graphemeSegmentIndexAtCursor(segs, cursor) - 1; i >= 0; i-- {
			if graphemeSegmentContainsRune(segs[i], r) {
				if found++; found == count {
					return segs[i].end, cursor, nil
				}
			}
		}
		return cursor, cursor, nil
	}
}

func motionToStartOfWordForward(cursor int, buffer []rune, count int) (int, int, error) {
	segs := graphemeSegments(buffer)
	if len(segs) == 0 {
		return 0, 0, nil
	}
	start := cursor
	endIdx := graphemeSegmentIndexAtCursor(segs, cursor)
	for range count {
		if endIdx >= len(segs) {
			return start, len(buffer), nil
		}
		starting := graphemeSegmentWordType(segs[endIdx])
		hadWhitespace := starting == WordTypeWhitespace
		found := false
		for i := endIdx + 1; i < len(segs); i++ {
			current := graphemeSegmentWordType(segs[i])
			if !hadWhitespace {
				if current == WordTypeWhitespace {
					hadWhitespace = true
				} else if current != starting {
					endIdx = i
					found = true
					break
				}
				continue
			}
			if current != WordTypeWhitespace {
				endIdx = i
				found = true
				break
			}
		}
		if !found {
			return start, len(buffer), nil
		}
	}
	return start, segs[endIdx].start, nil
}

type WordType uint8

const (
	WordTypeWhitespace WordType = iota
	WordTypeWord
	WordTypeNonWhitespace
)

func motionToStartOfNonWhitespaceForward(cursor int, buffer []rune, count int) (int, int, error) {
	segs := graphemeSegments(buffer)
	if len(segs) == 0 {
		return 0, 0, nil
	}
	start := cursor
	endIdx := graphemeSegmentIndexAtCursor(segs, cursor)
	for range count {
		hadWhitespace := false
		found := false
		for i := endIdx; i < len(segs); i++ {
			if graphemeSegmentWordType(segs[i]) == WordTypeWhitespace {
				hadWhitespace = true
			} else if hadWhitespace {
				endIdx = i
				found = true
				break
			}
		}
		if !found {
			return start, len(buffer), nil
		}
	}
	return start, segs[endIdx].start, nil
}

func motionToStartOfWordBackward(cursor int, buffer []rune, count int) (int, int, error) {
	segs := graphemeSegments(buffer)
	if len(segs) == 0 {
		return 0, 0, nil
	}
	startIdx := graphemeSegmentIndexAtCursor(segs, cursor)
	end := cursor
	for range count {
		if startIdx <= 0 {
			return 0, end, nil
		}
		startType := graphemeSegmentWordType(segs[startIdx])
		atStartOfWord := startIdx == 0 || (startType != WordTypeWhitespace && graphemeSegmentWordType(segs[startIdx-1]) != startType)
		needsBreak := startType == WordTypeWhitespace || atStartOfWord
		found := false
		for i := startIdx - 1; i >= 0; i-- {
			current := graphemeSegmentWordType(segs[i])
			if current != startType {
				if !needsBreak {
					startIdx = i + 1
					found = true
					break
				}
				startType = current
				if current == WordTypeWhitespace {
					continue
				}
				needsBreak = false
				continue
			}
		}
		if !found {
			return 0, end, nil
		}
	}
	return segs[startIdx].start, end, nil
}

func motionToStartOfNonWhitespaceBackward(cursor int, buffer []rune, count int) (int, int, error) {
	segs := graphemeSegments(buffer)
	if len(segs) == 0 {
		return 0, 0, nil
	}
	startIdx := graphemeSegmentIndexAtCursor(segs, cursor)
	end := cursor
	for range count {
		if startIdx <= 0 {
			return 0, end, nil
		}
		seenNonWhitespace := false
		found := false
		for i := startIdx; i >= 0; i-- {
			ws := graphemeSegmentWordType(segs[i]) == WordTypeWhitespace
			if seenNonWhitespace && ws {
				startIdx = i + 1
				found = true
				break
			}
			if ws && seenNonWhitespace {
				startIdx = i + 1
				found = true
				break
			}
			if !ws && i < startIdx {
				seenNonWhitespace = true
				continue
			}
		}
		if !found {
			return 0, end, nil
		}
	}
	return segs[startIdx].start, end, nil
}

func isSmallWordChar(r rune) bool {
	/*
	 * word A word consists of a sequence of letters, digits and underscores, or a sequence of other non-blank characters,
	 *  separated with white space (spaces, tabs, <EOL>).
	 *  This can be changed with the iskeyword option. An empty line is also considered to be a word.
	 */
	return r == '_' || isAlphaNumericChar(r)
}

func isAlphaNumericChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func motionToEndOfWordForward(cursor int, buffer []rune, count int) (int, int, error) {
	segs := graphemeSegments(buffer)
	if len(segs) == 0 {
		return 0, 0, nil
	}
	start := cursor
	endIdx := graphemeSegmentIndexAtCursor(segs, cursor)
	for range count {
		if endIdx >= len(segs) {
			return start, len(buffer), nil
		}
		starting := graphemeSegmentWordType(segs[endIdx])
		atEndOfWord := starting != WordTypeWhitespace && endIdx+1 < len(segs) && graphemeSegmentWordType(segs[endIdx+1]) != starting
		startedWord := !atEndOfWord && starting != WordTypeWhitespace
		hadWhitespace := false
		found := false
		for i := endIdx + 1; i < len(segs); i++ {
			current := graphemeSegmentWordType(segs[i])
			if !startedWord {
				if current == WordTypeWhitespace {
					hadWhitespace = true
					continue
				}
				if current != starting || hadWhitespace {
					startedWord = true
					starting = current
				}
				continue
			}
			if current != starting {
				endIdx = i - 1
				found = true
				break
			}
		}
		if !found {
			return start, len(buffer), nil
		}
	}
	return start, segs[endIdx].start, nil
}

func motionToEndOfNonWhitespaceForward(cursor int, buffer []rune, count int) (int, int, error) {
	segs := graphemeSegments(buffer)
	if len(segs) == 0 {
		return 0, 0, nil
	}
	start := cursor
	endIdx := graphemeSegmentIndexAtCursor(segs, cursor)
	for range count {
		if endIdx >= len(segs) {
			return start, len(buffer), nil
		}
		startedAtWhitespace := graphemeSegmentWordType(segs[endIdx]) == WordTypeWhitespace
		atEndOfWord := !startedAtWhitespace && endIdx+1 < len(segs) && graphemeSegmentWordType(segs[endIdx+1]) == WordTypeWhitespace
		startedWord := !atEndOfWord && !startedAtWhitespace
		found := false
		for i := endIdx + 1; i < len(segs); i++ {
			currentIsWhitespace := graphemeSegmentWordType(segs[i]) == WordTypeWhitespace
			if !startedWord {
				if !currentIsWhitespace {
					startedWord = true
				}
				continue
			}
			if currentIsWhitespace {
				endIdx = i - 1
				found = true
				break
			}
		}
		if !found {
			return start, len(buffer), nil
		}
	}
	return start, segs[endIdx].start, nil
}

func motionToEndOfWordBackward(cursor int, buffer []rune, count int) (int, int, error) {
	segs := graphemeSegments(buffer)
	if len(segs) == 0 {
		return 0, 0, nil
	}
	startIdx := graphemeSegmentIndexAtCursor(segs, cursor)
	end := cursor
	for range count {
		if startIdx <= 0 {
			return 0, end, nil
		}
		startType := graphemeSegmentWordType(segs[startIdx])
		hadWhitespace := startType == WordTypeWhitespace
		found := false
		for i := startIdx - 1; i >= 0; i-- {
			current := graphemeSegmentWordType(segs[i])
			if current != WordTypeWhitespace && current != startType {
				startIdx = i
				found = true
				break
			}
			if !hadWhitespace {
				if current == WordTypeWhitespace {
					hadWhitespace = true
				}
				continue
			}
			if current != WordTypeWhitespace {
				startIdx = i
				found = true
				break
			}
		}
		if !found {
			return 0, end, nil
		}
	}
	return segs[startIdx].start, end, nil
}

func motionToEndOfNonWhitespaceBackward(cursor int, buffer []rune, count int) (int, int, error) {
	segs := graphemeSegments(buffer)
	if len(segs) == 0 {
		return 0, 0, nil
	}
	startIdx := graphemeSegmentIndexAtCursor(segs, cursor)
	end := cursor
	for range count {
		if startIdx <= 0 {
			return 0, end, nil
		}
		startType := graphemeSegmentWordType(segs[startIdx])
		hadWhitespace := startType == WordTypeWhitespace
		found := false
		for i := startIdx - 1; i >= 0; i-- {
			current := graphemeSegmentWordType(segs[i])
			if !hadWhitespace {
				if current == WordTypeWhitespace {
					hadWhitespace = true
				}
				continue
			}
			if current != WordTypeWhitespace {
				startIdx = i
				found = true
				break
			}
		}
		if !found {
			return 0, end, nil
		}
	}
	return segs[startIdx].start, end, nil
}

type graphemeSegment struct {
	start int
	end   int
	text  string
}

func graphemeSegments(buffer []rune) []graphemeSegment {
	if len(buffer) == 0 {
		return nil
	}
	var segs []graphemeSegment
	g := uniseg.NewGraphemes(string(buffer))
	offset := 0
	for g.Next() {
		runes := g.Runes()
		text := g.Str()
		segs = append(segs, graphemeSegment{
			start: offset,
			end:   offset + len(runes),
			text:  text,
		})
		offset += len(runes)
	}
	return segs
}

func graphemeSegmentIndexAtCursor(segs []graphemeSegment, cursor int) int {
	if cursor <= 0 {
		return 0
	}
	for i, seg := range segs {
		if cursor < seg.end {
			return i
		}
		if cursor == seg.end {
			return min(i+1, len(segs))
		}
	}
	return len(segs)
}

func graphemeSegmentWordType(seg graphemeSegment) WordType {
	if seg.text == "" {
		return WordTypeWhitespace
	}
	if isWhitespaceString(seg.text) {
		return WordTypeWhitespace
	}
	for _, r := range seg.text {
		if !unicode.IsMark(r) {
			if isSmallWordChar(r) {
				return WordTypeWord
			}
			break
		}
	}
	return WordTypeNonWhitespace
}

func graphemeSegmentContainsRune(seg graphemeSegment, r rune) bool {
	return strings.ContainsRune(seg.text, r)
}

func isWhitespaceString(s string) bool {
	for _, r := range s {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return s != ""
}

func nextGraphemeBoundary(buffer []rune, pos int) int {
	if pos >= len(buffer) {
		return len(buffer)
	}
	for _, seg := range graphemeSegments(buffer) {
		if seg.end > pos {
			return seg.end
		}
	}
	return len(buffer)
}
