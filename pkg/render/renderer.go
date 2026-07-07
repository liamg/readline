package render

import (
	"bytes"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/liamg/readline/pkg/ansi"
	"github.com/liamg/readline/pkg/config"
	"github.com/liamg/readline/pkg/editor"
	"github.com/rivo/uniseg"
)

// Cell represents one visible grapheme cluster in the rendered output.
// Text may contain multiple runes, for example emoji sequences joined by ZWJ.
type Cell struct {
	Rune  rune
	Text  string
	Style ansi.Style
	Width int
}

// Driver is the minimal interface the renderer needs to write to the terminal.
type Driver interface {
	io.Writer
}

// Renderer maintains the virtual cell buffer and emits minimal terminal writes
// on each Render call, only touching the parts of the screen that changed.
type Renderer struct {
	config config.Config
	editor *editor.Editor
	width  int // terminal width in columns
	height int
	state  State
	driver Driver
}

type QueuedLine struct {
	Line      Line
	Continue  bool
	CursorCol int // 1-indexed - 0 means no cursor
}

type QueuedLines []QueuedLine

type State struct {
	termCur int // flat column offset of the terminal cursor from render-start
	// we cache the parsed prompt as we don't want to regenerate it with every render
	promptCells     []Cell
	statusLineCells []Cell
	previousLines   QueuedLines
	currentLines    QueuedLines
	previousCursorY int // number of physical terminal rows down from render-start to the cursor
}

func New(cfg config.Config, ed *editor.Editor, driver Driver) Renderer {
	return Renderer{
		config: cfg,
		editor: ed,
		driver: driver,
	}
}

// SetSize updates the terminal width+height. Call this initially and on every resize event.
func (r *Renderer) SetSize(w, h int) {
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 30
	}
	r.width = w
	r.height = h
	// force a fresh render
	r.Reset()
}

// Reset clears renderer state without writing to the terminal. Use this at the
// start of a fresh readline session when the terminal cursor is already on a
// clean line.
func (r *Renderer) Reset() {
	r.state = State{}
}

// HasRenderedContent reports whether the renderer still tracks prompt/editor
// rows that have been written to the terminal and may need clearing before
// ordinary output is written.
func (r *Renderer) HasRenderedContent() bool {
	return len(r.state.previousLines) > 0
}

// Clear moves the terminal cursor to the start of the render area and erases
// all rendered content. Use this before suspension or on resize.
func (r *Renderer) Clear(driver Driver) error {
	return r.clear(driver, r.state.previousCursorY)
}

// ClearForResize clears rendered content after the terminal has resized. It
// accounts for terminal reflow: lines rendered at the old width may occupy more
// physical rows after a shrink, so the cursor can be farther from the render
// area's first row than previousCursorY alone indicates.
func (r *Renderer) ClearForResize(driver Driver, newWidth int) error {
	if newWidth <= 0 {
		newWidth = 80
	}
	return r.clear(driver, r.physicalRowsAboveCursor(newWidth))
}

func (r *Renderer) clear(driver Driver, rowsAboveCursor int) error {
	var buf bytes.Buffer
	// Move up to the first row of the render area.
	if rowsAboveCursor > 0 {
		fmt.Fprintf(&buf, "\x1b[%dA", rowsAboveCursor)
	}
	buf.WriteString("\r\x1b[J") // carriage-return, erase to end of screen
	_, err := driver.Write(buf.Bytes())
	if err == nil {
		r.Reset()
	}
	return err
}

func (r *Renderer) physicalRowsAboveCursor(width int) int {
	if width <= 0 {
		width = 80
	}
	return physicalRowsAboveCursor(r.state.previousLines, width)
}

func physicalRows(line Line, width int) int {
	if width <= 0 {
		width = 80
	}
	w := line.Width()
	if w == 0 {
		return 1
	}
	return ((w - 1) / width) + 1
}

func physicalRowsAboveCursor(lines QueuedLines, width int) int {
	if width <= 0 {
		width = 80
	}
	rows := 0
	for _, line := range lines {
		if line.CursorCol > 0 {
			return rows + (line.CursorCol-1)/width
		}
		rows += physicalRows(line.Line, width)
	}
	return rows
}

func physicalRowsAfterCursor(lines QueuedLines, cursorLine int, width int) int {
	if width <= 0 {
		width = 80
	}
	if cursorLine < 0 || cursorLine >= len(lines) {
		return 0
	}
	rows := physicalRows(lines[cursorLine].Line, width)
	if cursorCol := lines[cursorLine].CursorCol; cursorCol > 0 {
		cursorRowInLine := (cursorCol - 1) / width
		rows -= cursorRowInLine + 1
	}
	for _, line := range lines[cursorLine+1:] {
		rows += physicalRows(line.Line, width)
	}
	return rows
}

func physicalRowsBeforeLine(lines QueuedLines, lineIndex int, width int) int {
	if lineIndex > len(lines) {
		lineIndex = len(lines)
	}
	rows := 0
	for _, line := range lines[:lineIndex] {
		rows += physicalRows(line.Line, width)
	}
	return rows
}

func physicalColumn(column int, width int) int {
	if column <= 0 {
		return column
	}
	if width <= 0 {
		width = 80
	}
	return (column-1)%width + 1
}

type Line []Cell

func (l Line) CountRune(r rune) int {
	var count int
	for _, c := range l {
		if c.Rune == r || cellText(c) == string(r) {
			count++
		}
	}
	return count
}

// FromSpans converts a slice of runes and their associated style spans into a
// slice of Lines
func FromSpans(runes []rune, spans []ansi.Span) Line {
	if len(runes) == 0 {
		return nil
	}

	text := string(runes)
	graphemes := uniseg.NewGraphemes(text)
	line := make([]Cell, 0, len(runes))
	runeOffset := 0
	for graphemes.Next() {
		clusterRunes := graphemes.Runes()
		clusterText := graphemes.Str()
		if clusterText == "" {
			continue
		}
		cell := Cell{
			Text:  clusterText,
			Style: styleAt(runeOffset, spans),
			Width: graphemes.Width(),
		}
		if len(clusterRunes) > 0 {
			cell.Rune = clusterRunes[0]
		}
		line = append(line, cell)
		runeOffset += len(clusterRunes)
	}
	return line
}

func (l Line) Width() int {
	var w int
	for _, c := range l {
		w += c.Width
	}
	return w
}

func SplitLines(input Line, termWidth int) []Line {
	lines := make([]Line, 0, 1)
	line := make(Line, 0, len(input))
	var lineWidth int
	for i, c := range input {
		switch cellText(c) {
		case "\r":
			continue
		case "\n":
			lines = append(lines, line)
			line = make(Line, 0, len(input)-i)
			if i == len(input)-1 {
				lines = append(lines, line)
				return lines
			}
			lineWidth = 0
			continue
		}
		if lineWidth+c.Width > termWidth {
			lines = append(lines, line)
			line = make(Line, 0, len(input)-i)
			lineWidth = 0
		}
		line = append(line, c)
		lineWidth += c.Width
	}
	return append(lines, line)
}

// UpdatePrompt calls the configured prompt function and caches the result.
// This gets called automatically during a Render() if required, but can optionally
// get called to update the cached prompt during a rerender of the readline.
func (r *Renderer) UpdatePrompt() {
	if r.config.Prompt != nil {
		r.state.promptCells = FromSpans(ansi.Parse(r.config.Prompt(r.width, r.height)))
	}
}

func (r *Renderer) UpdateStatusLine() {
	if r.config.StatusLine != nil {
		r.state.statusLineCells = FromSpans(ansi.Parse(r.config.StatusLine(r.width, r.height)))
	}
}

// Render writes the readline prompt and editor to the terminal, updating only changed runes where possible.
func (r *Renderer) Render() error {
	return r.render(false)
}

// RenderAccepted renders the final accepted line without transient UI such as
// autosuggestions, hints, completions, or status lines.
func (r *Renderer) RenderAccepted() error {
	return r.render(true)
}

func (r *Renderer) render(accepted bool) error {
	// if we haven't run the prompt func yet, do it
	if r.state.promptCells == nil {
		r.UpdatePrompt()
	}
	if !accepted {
		r.UpdateStatusLine()
	}

	var spans []ansi.Span
	bufRunes := r.editor.Runes()
	if r.config.Highlighter != nil {
		spans = r.config.Highlighter(bufRunes)
	}

	logCells := []Cell{}
	for _, log := range r.editor.GetLogs() {
		for _, r := range log {
			logCells = append(logCells, Cell{
				Rune: r,
				Style: ansi.Style{
					Attr: ansi.AttrDim,
				},
				Width: ansi.RuneWidth(r),
			})
		}
		logCells = append(logCells, Cell{
			Rune: '\n',
			Style: ansi.Style{
				Attr: ansi.AttrDim,
			},
		})
	}

	bufCells := FromSpans(bufRunes, spans)

	autosuggestCells := []Cell{}
	if suggestion := r.editor.GetAutoSuggestion(); !accepted && len(suggestion) > 0 {
		suggestion = normalizeAutosuggestionForDisplay(bufRunes, r.editor.Cursor(), suggestion)
		// Build cells per grapheme cluster (via FromSpans) so multi-rune emoji
		// from history measure and render correctly, then dim them uniformly.
		autosuggestCells = FromSpans(suggestion, nil)
		for i := range autosuggestCells {
			autosuggestCells[i].Style = ansi.Style{Attr: ansi.AttrDim}
		}
	}

	preCells := make([]Cell, 0, len(r.state.promptCells)+len(logCells))
	preCells = append(preCells, logCells...)
	if !accepted || !r.config.HidePromptOnSubmit {
		preCells = append(preCells, r.state.promptCells...)
	}

	preLines := SplitLines(preCells, r.width)

	mainCells := make([]Cell, 0, 1+len(bufCells)+len(autosuggestCells))

	mainCells = append(mainCells, bufCells...)

	mainCells = append(mainCells, autosuggestCells...)

	if len(preLines) > 0 {
		r.queueLines(preLines, true, false)
	}
	r.queueCells(mainCells, true, r.editor.Cursor())

	// TODO: we can try to add the right prompt if there's room at the end of the line...
	// 1. split right prompt into lines
	// 2. measure first (top) line width
	// 3. if width > space left on current line, add a new line (unless remaining lines == 0)
	// 4. render padding and the right prompt, so it aligns to the right
	rightPromptCells := []Cell{}
	_ = rightPromptCells

	// if we can only fit the base lines into the available lines in the terminal,
	// there's no point trying to render hints, right prompts, or completions
	if !accepted && r.remainingRows() > 0 {

		hintCells := []Cell{}
		if hint := r.editor.GetHint(); len(hint) > 0 {
			for _, r := range hint {
				hintCells = append(hintCells, Cell{
					Rune: r,
					Style: ansi.Style{
						Attr: ansi.AttrDim,
					},
					Width: ansi.RuneWidth(r),
				})
			}
		}

		// TODO: these need to be truncated to the terminal width
		completionLines := []Line{}
		var longestCandidateName int
		completionGroups := r.editor.GetCompletions()
		for _, group := range completionGroups {
			for _, candidate := range group.Candidates {
				if w := ansi.VisibleWidth(candidate.Name); w > longestCandidateName {
					longestCandidateName = w
				}
			}
		}
		showCompletionTitles := len(completionGroups) > 1 && len(completionGroups[0].Candidates)+2 < r.remainingRows()
		for _, group := range completionGroups {

			// if more than one completion group is shown, include titles
			if showCompletionTitles && group.Name != "" {
				titleWidth := min(ansi.VisibleWidth(group.Name), r.width)
				title := make(Line, 0, titleWidth)
				for _, ru := range []rune(group.Name)[:titleWidth] {
					title = append(title, Cell{
						Rune: ru,
						Style: ansi.Style{
							Fg: ansi.Color{
								Mode:  ansi.Color16,
								Index: 3,
							},
						},
						Width: ansi.RuneWidth(ru),
					})
				}
				completionLines = append(completionLines, title)
			}

			for _, candidate := range group.Candidates {

				completionLine := make([]Cell, 0, longestCandidateName+1+len(candidate.Description))
				w := ansi.VisibleWidth(candidate.Name)
				rendereredWidth := 0
				name := candidate.Name + strings.Repeat(" ", 1+(longestCandidateName-w))
				for _, ru := range name {
					rw := ansi.RuneWidth(ru)
					if rendereredWidth+rw > r.width {
						break
					}
					rendereredWidth += rw
					completionLine = append(completionLine, Cell{
						Rune: ru,
						Style: ansi.Style{
							Fg: ansi.Color{
								Mode:  ansi.Color16,
								Index: 4,
							},
						},
						Width: rw,
					})
				}
				for _, ru := range candidate.Description {
					rw := ansi.RuneWidth(ru)
					if rendereredWidth+rw > r.width {
						break
					}
					rendereredWidth += rw
					completionLine = append(completionLine, Cell{
						Rune:  ru,
						Width: rw,
					})
				}

				completionLines = append(completionLines, completionLine)
			}
		}

		if len(hintCells) > 0 || len(completionLines) > 0 || len(r.state.statusLineCells) > 0 {
			// new line after prompts + buffer if we have anything to render after it
			r.endLine()
		}

		if len(hintCells) > 0 {
			r.queueLines(SplitLines(hintCells, r.width), false, false)
		}

		if len(completionLines) > 0 {
			completionLines = limitLinesToRows(completionLines, r.remainingRows()-1, r.width)
			r.queueLines(completionLines, false, false)
		}

		if len(r.state.statusLineCells) > 0 && r.remainingRows() > 0 {
			r.queueLines(SplitLines(r.state.statusLineCells, r.width), false, false)
		}
	}

	// write to terminal
	if err := r.writeDiff(); err != nil {
		return err
	}

	return nil
}

func (r *Renderer) writeDiff() error {
	b := &bytes.Buffer{}

	currentLineCount := len(r.state.currentLines)
	prevousLineCount := len(r.state.previousLines)
	sharedMinLines := min(currentLineCount, prevousLineCount)

	firstDiffLine := sharedMinLines
	for i := range sharedMinLines {
		fresh := r.state.currentLines[i]
		previous := r.state.previousLines[i]
		if fresh.Continue != previous.Continue || !slices.Equal(fresh.Line, previous.Line) {
			firstDiffLine = i
			break
		}
	}

	// NOTE: cursorX is 1-indexed, cursorY is zero-indexed logical line index,
	// and cursorPhysicalY is the physical terminal row index.
	cursorX, cursorY, cursorPhysicalY := 0, 0, 0
	for i, line := range r.state.currentLines {
		if line.CursorCol > 0 {
			cursorY = i
			cursorX = physicalColumn(line.CursorCol, r.width)
			cursorPhysicalY = physicalRowsAboveCursor(r.state.currentLines, r.width)
			break
		}
	}

	if firstDiffLine == sharedMinLines && len(r.state.previousLines) == len(r.state.currentLines) {
		// everything matched perfectly! no redraw needed!
		// just update the cursor position if needed

		// first let's move the cursor to the start of the previous output
		if r.state.previousCursorY > 0 {
			_, _ = fmt.Fprintf(b, "\x1b[%dA", r.state.previousCursorY)
		}
		_, _ = fmt.Fprint(b, "\r")

		if cursorPhysicalY > 0 {
			fmt.Fprintf(b, "\x1b[%dB", cursorPhysicalY)
		}
		fmt.Fprintf(b, "\x1b[%dG", cursorX)

		// bump state
		r.state.previousLines = r.state.currentLines
		r.state.currentLines = nil
		r.state.previousCursorY = cursorPhysicalY

		_, err := r.driver.Write(b.Bytes())
		return err
	}

	// first let's move the cursor to the start of the previous output
	if r.state.previousCursorY > 0 {
		_, _ = fmt.Fprintf(b, "\x1b[%dA", r.state.previousCursorY)
	}
	_, _ = fmt.Fprint(b, "\r")

	// this is the cell in the first diffed line to start rendering from, as the start of the line may already be rendered correctly
	drawFromCellX := 0

	// if this isn't the first render, diff the content and only render the changes
	if len(r.state.previousLines) > 0 {
		// 1. move the cursor down to the first line that changed
		if rows := physicalRowsBeforeLine(r.state.currentLines, firstDiffLine, r.width); rows > 0 {
			fmt.Fprintf(b, "\x1b[%dB", rows)
		}

		// 2. move to the point in the line that changed and clear to EOS and redraw from there
		if firstDiffLine < len(r.state.currentLines) {
			diffLineCurrent := r.state.currentLines[firstDiffLine]
			if firstDiffLine >= len(r.state.previousLines) {
				// move to start of line and clear line
				_, _ = fmt.Fprint(b, "\r\x1b[K")
			} else {
				diffLinePrev := r.state.previousLines[firstDiffLine]
				col := 1
				for i, c := range diffLineCurrent.Line {
					if i >= len(diffLinePrev.Line) {
						drawFromCellX = i
						// move to correct column to start rendering from and delete to EOS
						if rows := (col - 1) / r.width; rows > 0 {
							_, _ = fmt.Fprintf(b, "\x1b[%dB", rows)
						}
						_, _ = fmt.Fprintf(b, "\x1b[%dG\x1b[J", physicalColumn(col, r.width))
						break
					}
					old := diffLinePrev.Line[i]
					if cellText(old) != cellText(c) || old.Style != c.Style {
						drawFromCellX = i
						// move to correct column to start rendering from and delete to EOS
						if rows := (col - 1) / r.width; rows > 0 {
							_, _ = fmt.Fprintf(b, "\x1b[%dB", rows)
						}
						_, _ = fmt.Fprintf(b, "\x1b[%dG\x1b[J", physicalColumn(col, r.width))
						break
					}
					col += c.Width
				}
				if drawFromCellX == 0 {
					_, _ = fmt.Fprint(b, "\r\x1b[K")
				}
			}
		}

	}

	// 3. draw all remaining lines
	if firstDiffLine < len(r.state.currentLines) {
		var currentStyle ansi.Style
		for i, line := range r.state.currentLines[firstDiffLine:] {
			cells := line.Line
			if i == 0 && drawFromCellX > 0 {
				cells = cells[drawFromCellX:]
			}

			if i > 0 {
				_, _ = fmt.Fprint(b, "\r\n")
			}

			for _, cell := range cells {
				if cell.Style != currentStyle {
					writeStyle(b, cell.Style)
					currentStyle = cell.Style
				}
				b.WriteString(cellText(cell))
			}

		}

		// reset style
		_, _ = fmt.Fprint(b, "\x1b[0m")
	}

	// the cursor is at the end of the rendered content, so let's clear the screen from here onward in case the output got shorter
	_, _ = fmt.Fprint(b, "\x1b[J")

	// move the cursor to the correct row
	if up := physicalRowsAfterCursor(r.state.currentLines, cursorY, r.width); up > 0 {
		_, _ = fmt.Fprintf(b, "\x1b[%dA", up)
	}
	// and correct column
	_, _ = fmt.Fprintf(b, "\x1b[%dG", cursorX)

	// bump state
	r.state.previousLines = r.state.currentLines
	r.state.currentLines = nil
	r.state.previousCursorY = cursorPhysicalY

	_, err := r.driver.Write(b.Bytes())
	return err
}

func (r *Renderer) remainingRows() int {
	return r.height - queuedPhysicalRows(r.state.currentLines, r.width)
}

func queuedPhysicalRows(lines QueuedLines, width int) int {
	var rows int
	for _, line := range lines {
		rows += physicalRows(line.Line, width)
	}
	return rows
}

func limitLinesToRows(lines []Line, maxRows int, width int) []Line {
	if maxRows <= 0 {
		return nil
	}
	var rows int
	for i, line := range lines {
		rows += physicalRows(line, width)
		if rows > maxRows {
			return lines[:i]
		}
	}
	return lines
}

func normalizeAutosuggestionForDisplay(buffer []rune, cursor int, suggestion []rune) []rune {
	if len(suggestion) == 0 || cursor <= 0 || cursor > len(buffer) {
		return suggestion
	}
	if buffer[cursor-1] != '\n' {
		return suggestion
	}
	for len(suggestion) > 0 && (suggestion[0] == '\n' || suggestion[0] == '\r') {
		suggestion = suggestion[1:]
	}
	return suggestion
}

func calcCellsWidth(cells []Cell) int {
	var w int
	for _, c := range cells {
		w += c.Width
	}
	return w
}

func cellText(c Cell) string {
	if c.Text != "" {
		return c.Text
	}
	if c.Rune != 0 {
		return string(c.Rune)
	}
	return ""
}

func cellRuneLen(c Cell) int {
	if c.Text != "" {
		return utf8.RuneCountInString(c.Text)
	}
	if c.Rune != 0 {
		return 1
	}
	return 0
}

func (r *Renderer) queueLines(lines []Line, continueLine, placeCursorAfter bool) {
	if len(lines) == 0 {
		return
	}

	if c := len(r.state.currentLines); c > 0 {
		if prev := r.state.currentLines[c-1]; prev.Continue {
			prev.Line = append(prev.Line, lines[0]...)
			prev.Continue = len(lines) == 1 && continueLine
			if placeCursorAfter {
				if len(lines) == 1 {
					prev.CursorCol = calcCellsWidth(prev.Line) + 1
				} else {
					prev.CursorCol = 0
				}
			}
			r.state.currentLines[c-1] = prev
			lines = lines[1:]
		}
	}

	for i, line := range lines {
		isLast := i == len(lines)-1
		q := QueuedLine{
			Line:     line,
			Continue: isLast && continueLine,
		}
		if placeCursorAfter && isLast {
			q.CursorCol = calcCellsWidth(line) + 1
		}
		r.state.currentLines = append(r.state.currentLines, q)
	}
}

func (r *Renderer) queueCells(cells Line, continueLine bool, cursorOffset int) {
	if cursorOffset < 0 {
		cursorOffset = 0
	}
	totalRunes := 0
	for _, cell := range cells {
		totalRunes += cellRuneLen(cell)
	}
	if cursorOffset > totalRunes {
		cursorOffset = totalRunes
	}

	lineIndex := len(r.state.currentLines) - 1
	if lineIndex < 0 || !r.state.currentLines[lineIndex].Continue {
		r.state.currentLines = append(r.state.currentLines, QueuedLine{Continue: true})
		lineIndex++
	}
	lineWidth := calcCellsWidth(r.state.currentLines[lineIndex].Line)

	newLine := func() {
		r.state.currentLines[lineIndex].Continue = false
		r.state.currentLines = append(r.state.currentLines, QueuedLine{Continue: true})
		lineIndex++
		lineWidth = 0
	}
	placeCursor := func() {
		r.state.currentLines[lineIndex].CursorCol = lineWidth + 1
	}

	cursorRunes := 0
	for _, cell := range cells {
		text := cellText(cell)
		cellRunes := cellRuneLen(cell)
		switch text {
		case "\r":
			if cursorOffset == cursorRunes {
				placeCursor()
			}
			cursorRunes += cellRunes
			continue
		case "\n":
			if cursorOffset == cursorRunes {
				placeCursor()
			}
			cursorRunes += cellRunes
			newLine()
			if cursorOffset == cursorRunes {
				placeCursor()
			}
			continue
		}

		if lineWidth+cell.Width > r.width {
			newLine()
		}
		if cursorOffset == cursorRunes {
			placeCursor()
		}
		r.state.currentLines[lineIndex].Line = append(r.state.currentLines[lineIndex].Line, cell)
		lineWidth += cell.Width
		if cursorOffset > cursorRunes && cursorOffset <= cursorRunes+cellRunes {
			placeCursor()
		}
		cursorRunes += cellRunes
	}

	if cursorOffset == cursorRunes {
		if lineWidth >= r.width {
			newLine()
		}
		placeCursor()
	}
	r.state.currentLines[lineIndex].Continue = continueLine
}

func (r *Renderer) endLine() {
	if len(r.state.currentLines) == 0 {
		r.state.currentLines = append(r.state.currentLines, QueuedLine{
			Continue: false,
		})
		return
	}
	r.state.currentLines[len(r.state.currentLines)-1].Continue = false
}

// styleAt returns the Style for rune index i given a sorted, non-overlapping
// set of spans. Returns the zero Style if no span covers i.
func styleAt(i int, spans []ansi.Span) ansi.Style {
	for _, s := range spans {
		if i >= s.Start && i < s.End {
			return s.Style
		}
	}
	return ansi.Style{}
}

// buildColumns returns a slice of length len(cells)+1 where entry i holds the
// flat terminal column at which cell i starts. The final entry is the column
// after the last cell (total rendered width).
func buildColumns(cells []Cell) []int {
	cols := make([]int, len(cells)+1)
	col := 0
	for i, c := range cells {
		cols[i] = col
		col += c.Width
	}
	cols[len(cells)] = col
	return cols
}

// colAt returns cols[i], safely returning the last entry if i is out of range.
func colAt(cols []int, i int) int {
	if len(cols) == 0 {
		return 0
	}
	if i >= len(cols) {
		return cols[len(cols)-1]
	}
	return cols[i]
}

// writeStyle emits an SGR sequence that resets and then re-applies s in full.
func writeStyle(buf *bytes.Buffer, s ansi.Style) {
	buf.WriteString("\x1b[0")
	if s.Attr&ansi.AttrBold != 0 {
		buf.WriteString(";1")
	}
	if s.Attr&ansi.AttrDim != 0 {
		buf.WriteString(";2")
	}
	if s.Attr&ansi.AttrItalic != 0 {
		buf.WriteString(";3")
	}
	if s.Attr&ansi.AttrUnderline != 0 {
		buf.WriteString(";4")
	}
	if s.Attr&ansi.AttrReverse != 0 {
		buf.WriteString(";7")
	}
	if s.Attr&ansi.AttrStrike != 0 {
		buf.WriteString(";9")
	}
	writeSGRColor(buf, s.Fg, true)
	writeSGRColor(buf, s.Bg, false)
	buf.WriteByte('m')
}

func writeSGRColor(buf *bytes.Buffer, c ansi.Color, fg bool) {
	if c.Mode == ansi.ColorDefault {
		return
	}
	buf.WriteByte(';')
	switch c.Mode {
	case ansi.Color16:
		base := 30
		if !fg {
			base = 40
		}
		if c.Index >= 8 {
			fmt.Fprintf(buf, "%d", base+60+int(c.Index)-8)
		} else {
			fmt.Fprintf(buf, "%d", base+int(c.Index))
		}
	case ansi.Color256:
		if fg {
			fmt.Fprintf(buf, "38;5;%d", c.Index)
		} else {
			fmt.Fprintf(buf, "48;5;%d", c.Index)
		}
	case ansi.ColorRGB:
		if fg {
			fmt.Fprintf(buf, "38;2;%d;%d;%d", c.R, c.G, c.B)
		} else {
			fmt.Fprintf(buf, "48;2;%d;%d;%d", c.R, c.G, c.B)
		}
	}
}
