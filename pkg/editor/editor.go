package editor

import (
	"strings"
	"unicode"

	"github.com/liamg/readline/pkg/editor/buffer"
	"github.com/liamg/readline/pkg/editor/completion"
	"github.com/liamg/readline/pkg/editor/suggestion"
	"github.com/rivo/uniseg"
)

// SelectionType describes the kind of visual selection active in the editor.
type SelectionType uint8

const (
	SelectionChar  SelectionType = iota // character-wise (v)
	SelectionLine                       // line-wise (V)
	SelectionBlock                      // block/rectangle (ctrl-v)
)

// Selection holds the anchor point of an active visual selection. The other
// end of the selection is always the current cursor position.
type Selection struct {
	Anchor int
	Type   SelectionType
}

// Range returns the (start, end) rune indices of the selection given the
// current cursor position and buffer contents. end is exclusive, matching Go
// slice conventions. The result honours the selection Type:
//
//   - SelectionChar: the span between anchor and cursor, inclusive of the
//     character under the far end (vi's inclusive visual selection).
//   - SelectionLine: expanded to cover every whole line the span touches,
//     including the trailing newline so the lines are removed cleanly.
//   - SelectionBlock: a single contiguous range cannot represent a rectangular
//     block, so it is approximated as character-wise until multi-span block
//     selection is implemented.
func (s *Selection) Range(cursor int, buffer []rune) (start, end int) {
	lo, hi := s.Anchor, cursor
	if lo > hi {
		lo, hi = hi, lo
	}
	if lo < 0 {
		lo = 0
	}

	if s.Type == SelectionLine {
		return lineWiseRange(buffer, lo, hi)
	}

	// Character-wise (and block, approximated as character-wise).
	end = hi + 1
	if end > len(buffer) {
		end = len(buffer)
	}
	return lo, end
}

// lineWiseRange expands [lo, hi] to cover the whole lines it touches, from the
// start of lo's line to the end of hi's line (including its trailing newline).
func lineWiseRange(buffer []rune, lo, hi int) (start, end int) {
	if len(buffer) == 0 {
		return 0, 0
	}
	if hi >= len(buffer) {
		hi = len(buffer) - 1
	}
	start = 0
	for i := lo - 1; i >= 0; i-- {
		if buffer[i] == '\n' {
			start = i + 1
			break
		}
	}
	end = len(buffer)
	for i := hi; i < len(buffer); i++ {
		if buffer[i] == '\n' {
			end = i + 1
			break
		}
	}
	return start, end
}

type Editor struct {
	buffer    Buffer
	cursor    int
	selection *Selection
	backup    []rune
	logs      []string

	hint []rune

	completer      completion.Completer
	completions    []completion.Group
	suggester      suggestion.Suggester
	autosuggestion []rune

	clampCursorBeforeEnd bool
}

/*
log lines
log lines
log lines
PROMPT> buffer(dim)autosuggest < RPROMPT
  (dim) hint
  completion 1
> completion 2 <
  completion 3
*/

type Buffer interface {
	RuneAt(i int) rune
	Slice(start, end int) []rune
	Len() int
	Insert(i int, r ...rune)
	Delete(start int, length int)
}

type Option func(*Editor)

func WithSuggester(s suggestion.Suggester) Option {
	return func(e *Editor) {
		e.suggester = s
	}
}

func WithCompleter(c completion.Completer) Option {
	return func(e *Editor) {
		e.completer = c
	}
}

func New(opts ...Option) *Editor {
	e := &Editor{
		buffer: buffer.NewSlice(),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func (e *Editor) Log(line string) {
	e.logs = append(e.logs, line)
}

func (e *Editor) GetLogs() []string {
	return e.logs
}

func (e *Editor) GetAutoSuggestion() []rune {
	return e.autosuggestion
}

func (e *Editor) TriggerAutoSuggestion() {
	if e.suggester == nil {
		return
	}
	runes := e.buffer.Slice(0, e.buffer.Len())
	if len(runes) == 0 {
		e.autosuggestion = nil
		return
	}
	e.autosuggestion = e.suggester.Suggest(runes)
}

func (e *Editor) ClearCompletions() {
	e.completions = nil
}

func (e *Editor) GetCompletions() []completion.Group {
	return e.completions
}

func (e *Editor) refreshCompletions() {
	if e.completions == nil {
		return
	}
	if e.completer == nil || e.buffer.Len() == 0 {
		e.completions = nil
		return
	}
	groups := e.completer.Complete(e.buffer.Slice(0, e.buffer.Len()), e.Cursor())
	if len(flattenCompletionCandidates(groups)) == 0 {
		e.completions = nil
		return
	}
	e.completions = groups
}

func (e *Editor) TriggerCompletions() {
	if e.applyVisibleCompletionContinuation() {
		e.completions = nil
		return
	}
	if e.completer == nil {
		return
	}
	groups := e.completer.Complete(e.buffer.Slice(0, e.buffer.Len()), e.Cursor())
	if e.applyCompletionPrefix(groups) {
		e.completions = nil
		return
	}
	e.completions = groups
}

func (e *Editor) applyVisibleCompletionContinuation() bool {
	candidates := flattenCompletionCandidates(e.completions)
	if len(candidates) == 0 {
		return false
	}

	type match struct {
		start int
		value []rune
	}
	var matches []match
	for _, candidate := range candidates {
		if start, value, ok := e.completionContinuation(candidate); ok {
			matches = append(matches, match{start: start, value: value})
		}
	}
	if len(matches) != 1 {
		return false
	}
	e.replaceRange(matches[0].start, e.cursor, matches[0].value)
	return true
}

func (e *Editor) completionContinuation(candidate completion.Candidate) (start int, value []rune, ok bool) {
	content := []rune(completionCandidateContent(candidate))
	start, typedLen := e.completionReplacementStart(content)
	if typedLen == 0 {
		return 0, nil, false
	}

	if candidate.Join != "" && typedLen == len(content) {
		return start, append(append([]rune{}, content...), []rune(candidate.Join)...), true
	}

	if typedLen >= len(content) {
		return 0, nil, false
	}
	suffix := content[typedLen:]
	if !isCompletionContinuation(suffix) {
		return 0, nil, false
	}
	return start, content, true
}

func isCompletionContinuation(suffix []rune) bool {
	if len(suffix) == 0 || len(suffix) > 2 {
		return false
	}
	for _, r := range suffix {
		switch r {
		case '/', '.', '(', ')', ' ':
		default:
			return false
		}
	}
	return true
}

func (e *Editor) applyCompletionPrefix(groups []completion.Group) bool {
	candidates := flattenCompletionCandidates(groups)
	if len(candidates) == 0 {
		return false
	}

	target := completionCandidateContent(candidates[0])
	if len(candidates) > 1 {
		target = commonCompletionPrefix(candidates)
	}
	if target == "" {
		return false
	}

	start, typedLen := e.completionReplacementStart([]rune(target))
	if typedLen == 0 && e.cursor > 0 && !isCompletionBoundary(e.buffer.RuneAt(e.cursor-1)) {
		return false
	}
	if len([]rune(target)) <= typedLen {
		return false
	}
	e.replaceRange(start, e.cursor, []rune(target))
	return true
}

func flattenCompletionCandidates(groups []completion.Group) []completion.Candidate {
	var candidates []completion.Candidate
	for _, group := range groups {
		candidates = append(candidates, group.Candidates...)
	}
	return candidates
}

func completionCandidateContent(candidate completion.Candidate) string {
	if candidate.Content != "" {
		return candidate.Content
	}
	return candidate.Name
}

func commonCompletionPrefix(candidates []completion.Candidate) string {
	prefix := []rune(completionCandidateContent(candidates[0]))
	for _, candidate := range candidates[1:] {
		content := []rune(completionCandidateContent(candidate))
		i := 0
		for i < len(prefix) && i < len(content) && prefix[i] == content[i] {
			i++
		}
		prefix = prefix[:i]
		if len(prefix) == 0 {
			break
		}
	}
	return string(prefix)
}

func (e *Editor) completionReplacementStart(target []rune) (start int, typedLen int) {
	before := e.buffer.Slice(0, e.cursor)
	for i := len(before); i >= 0; i-- {
		if i > 0 && !isCompletionBoundary(before[i-1]) {
			continue
		}
		fragment := before[i:]
		if hasRunePrefix(target, fragment) {
			return i, len(fragment)
		}
	}
	return e.cursor, 0
}

func isCompletionBoundary(r rune) bool {
	return unicode.IsSpace(r) || strings.ContainsRune("|&;<>(){}[]", r)
}

func hasRunePrefix(s []rune, prefix []rune) bool {
	if len(prefix) > len(s) {
		return false
	}
	for i := range prefix {
		if s[i] != prefix[i] {
			return false
		}
	}
	return true
}

func (e *Editor) replaceRange(start, end int, value []rune) {
	if start < 0 {
		start = 0
	}
	if end > e.buffer.Len() {
		end = e.buffer.Len()
	}
	if start > end {
		start = end
	}
	e.buffer.Delete(start, end-start)
	e.cursor = start
	e.Insert(value...)
}

func (e *Editor) Reset() {
	e.buffer = buffer.NewSlice()
	e.cursor = 0
	e.selection = nil
	e.autosuggestion = nil
	e.completions = nil
	e.logs = e.logs[:0]
}

func (e *Editor) SetHint(hint string) {
	e.hint = []rune(hint)
}

func (e *Editor) GetHint() []rune {
	return e.hint
}

func (e *Editor) ClearHint() {
	e.hint = nil
}

func (e *Editor) Backup() {
	e.backup = e.Buffer()
}

func (e *Editor) Restore() {
	e.SetBuffer(e.backup)
}

func (e *Editor) SetBuffer(s []rune) {
	e.buffer = buffer.NewSliceFromRunes(s)
	e.cursor = e.buffer.Len()
	e.selection = nil
	e.autosuggestion = nil
	e.completions = nil
}

// SetSelectionAnchor starts a visual selection of the given type anchored at
// the current cursor position.
func (e *Editor) SetSelectionAnchor(t SelectionType) {
	e.selection = &Selection{Anchor: e.cursor, Type: t}
}

// ClearSelection removes any active visual selection.
func (e *Editor) ClearSelection() {
	e.selection = nil
}

// Selection returns the active selection, or nil if there is none.
func (e *Editor) Selection() *Selection {
	return e.selection
}

func (e *Editor) GetSelectedRunes() []rune {
	if e.selection == nil {
		return nil
	}
	start, end := e.selection.Range(e.cursor, e.Buffer())
	return e.buffer.Slice(start, end)
}

func (e *Editor) BufferString() string {
	return string(e.Buffer())
}

func (e *Editor) Buffer() []rune {
	return e.buffer.Slice(0, e.buffer.Len())
}

func (e *Editor) Runes() []rune {
	return e.buffer.Slice(0, e.buffer.Len())
}

func (e *Editor) ReplaceRune(r rune) {
	if e.cursor >= e.buffer.Len() {
		e.Insert(r)
		return
	}
	start, end := e.GraphemeBoundsAtCursor()
	if start >= end {
		e.Insert(r)
		return
	}
	e.buffer.Delete(start, end-start)
	e.cursor = start
	e.Insert(r)
}

func (e *Editor) GetRuneAt(index int) rune {
	return e.buffer.RuneAt(index)
}

func (e *Editor) SetClampCursorBeforeEnd(clamp bool) {
	e.clampCursorBeforeEnd = clamp
	if clamp && e.cursor >= e.buffer.Len() {
		e.cursor = lastGraphemeStart(e.Buffer())
	}
}

func (e *Editor) MoveCursor(offset int) {
	if offset == 0 {
		return
	}
	if offset == 1 {
		e.moveCursorByGrapheme(1)
		return
	}
	if offset == -1 {
		e.moveCursorByGrapheme(-1)
		return
	}
	newPos := e.cursor + offset
	if newPos < 0 {
		e.cursor = 0
		return
	}
	if e.clampCursorBeforeEnd && newPos >= e.buffer.Len() {
		e.cursor = lastGraphemeStart(e.Buffer())
	} else if !e.clampCursorBeforeEnd && newPos > e.buffer.Len() {
		e.cursor = e.buffer.Len()
	} else {
		e.cursor = newPos
	}
}

func (e *Editor) Cursor() int {
	return e.cursor
}

func (e *Editor) Insert(r ...rune) {
	e.buffer.Insert(e.cursor, r...)
	e.cursor += len(r)
	e.refreshCompletions()
}

// DeletePrevious (previous rune)
func (e *Editor) DeletePrevious() {
	_ = e.DeletePreviousGraphemes(1)
}

func (e *Editor) DeleteNext() {
	_ = e.DeleteNextGraphemes(1)
}

func (e *Editor) DeletePreviousGraphemes(count int) []rune {
	if count <= 0 || e.cursor <= 0 {
		return nil
	}
	buffer := e.Buffer()
	end := e.cursor
	start := end
	for i := 0; i < count && start > 0; i++ {
		start = previousGraphemeBoundary(buffer, start)
	}
	if start >= end {
		return nil
	}
	killed := append([]rune{}, buffer[start:end]...)
	e.buffer.Delete(start, end-start)
	e.cursor = start
	e.refreshCompletions()
	return killed
}

func (e *Editor) DeleteNextGraphemes(count int) []rune {
	if count <= 0 || e.cursor >= e.buffer.Len() {
		return nil
	}
	buffer := e.Buffer()
	start := e.cursor
	end := start
	for i := 0; i < count && end < len(buffer); i++ {
		end = nextGraphemeBoundary(buffer, end)
	}
	if start >= end {
		return nil
	}
	killed := append([]rune{}, buffer[start:end]...)
	e.buffer.Delete(start, end-start)
	e.refreshCompletions()
	return killed
}

func (e *Editor) GraphemeBoundsAtCursor() (start, end int) {
	buffer := e.Buffer()
	if len(buffer) == 0 {
		return 0, 0
	}
	if e.cursor >= len(buffer) {
		end := len(buffer)
		return previousGraphemeBoundary(buffer, end), end
	}
	start = 0
	for _, boundary := range graphemeBoundaries(buffer) {
		if boundary > e.cursor {
			return start, boundary
		}
		start = boundary
	}
	return start, len(buffer)
}

func (e *Editor) GraphemeBoundsBeforeCursor() (start, end int) {
	buffer := e.Buffer()
	if len(buffer) == 0 {
		return 0, 0
	}
	end = previousGraphemeBoundary(buffer, e.cursor)
	start = previousGraphemeBoundary(buffer, end)
	return start, end
}

func (e *Editor) GraphemeBoundsAtPosition(pos int) (start, end int) {
	buffer := e.Buffer()
	if len(buffer) == 0 {
		return 0, 0
	}
	if pos <= 0 {
		bounds := graphemeBoundaries(buffer)
		if len(bounds) < 2 {
			return 0, len(buffer)
		}
		return 0, bounds[1]
	}
	if pos >= len(buffer) {
		return e.GraphemeBoundsBeforePosition(pos)
	}
	start = 0
	for _, boundary := range graphemeBoundaries(buffer) {
		if boundary > pos {
			return start, boundary
		}
		start = boundary
	}
	return start, len(buffer)
}

func (e *Editor) GraphemeBoundsBeforePosition(pos int) (start, end int) {
	buffer := e.Buffer()
	if len(buffer) == 0 || pos <= 0 {
		return 0, 0
	}
	segs := graphemeSegments(buffer)
	if len(segs) == 0 {
		return 0, 0
	}
	for i, seg := range segs {
		if pos <= seg.end {
			if pos <= seg.start {
				if i == 0 {
					return 0, 0
				}
				prev := segs[i-1]
				return prev.start, prev.end
			}
			return seg.start, seg.end
		}
	}
	last := segs[len(segs)-1]
	return last.start, last.end
}

func (e *Editor) moveCursorByGrapheme(offset int) {
	if offset == 0 {
		return
	}
	buffer := e.Buffer()
	pos := e.cursor
	steps := offset
	if steps < 0 {
		for steps < 0 {
			pos = previousGraphemeBoundary(buffer, pos)
			steps++
		}
	} else {
		for steps > 0 {
			pos = nextGraphemeBoundary(buffer, pos)
			steps--
		}
	}
	if e.clampCursorBeforeEnd && pos >= len(buffer) {
		e.cursor = lastGraphemeStart(buffer)
		return
	}
	e.cursor = pos
}

func graphemeBoundaries(runes []rune) []int {
	boundaries := make([]int, 0, len(runes)+1)
	boundaries = append(boundaries, 0)
	segments := uniseg.NewGraphemes(string(runes))
	count := 0
	for segments.Next() {
		count += len(segments.Runes())
		boundaries = append(boundaries, count)
	}
	if last := len(runes); boundaries[len(boundaries)-1] != last {
		boundaries = append(boundaries, last)
	}
	return boundaries
}

func previousGraphemeBoundary(runes []rune, pos int) int {
	if pos <= 0 {
		return 0
	}
	if pos > len(runes) {
		pos = len(runes)
	}
	prev := 0
	for _, boundary := range graphemeBoundaries(runes) {
		if boundary >= pos {
			return prev
		}
		prev = boundary
	}
	return prev
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
		segs = append(segs, graphemeSegment{
			start: offset,
			end:   offset + len(runes),
			text:  g.Str(),
		})
		offset += len(runes)
	}
	return segs
}

func nextGraphemeBoundary(runes []rune, pos int) int {
	if pos >= len(runes) {
		return len(runes)
	}
	for _, boundary := range graphemeBoundaries(runes) {
		if boundary > pos {
			return boundary
		}
	}
	return len(runes)
}

func lastGraphemeStart(runes []rune) int {
	if len(runes) == 0 {
		return 0
	}
	return previousGraphemeBoundary(runes, len(runes))
}
