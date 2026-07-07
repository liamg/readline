package buffer

// SliceBuffer is a simple []rune-backed Buffer. It is correct for all input
// sizes encountered in a readline prompt; use a gap buffer or rope for very
// large documents.
type SliceBuffer struct {
	runes []rune
}

func NewSlice() *SliceBuffer {
	return &SliceBuffer{}
}

func NewSliceFromRunes(runes []rune) *SliceBuffer {
	return &SliceBuffer{runes: runes}
}

func (b *SliceBuffer) RuneAt(i int) rune {
	if i < 0 || i >= len(b.runes) {
		return 0
	}
	return b.runes[i]
}

func (b *SliceBuffer) Slice(start, end int) []rune {
	if start < 0 || start >= len(b.runes) {
		return nil
	}
	if end > len(b.runes) {
		end = len(b.runes)
	}

	out := make([]rune, end-start)
	copy(out, b.runes[start:end])
	return out
}

func (b *SliceBuffer) Len() int {
	return len(b.runes)
}

func (b *SliceBuffer) Insert(i int, r ...rune) {
	if i < 0 {
		i = 0
	} else if i > len(b.runes) {
		i = len(b.runes)
	}
	b.runes = append(b.runes[:i], append(r, b.runes[i:]...)...)
}

func (b *SliceBuffer) Delete(start, length int) {
	if start < 0 || start >= len(b.runes) {
		return
	}
	if start+length > len(b.runes) {
		length = len(b.runes) - start
	}
	b.runes = append(b.runes[:start], b.runes[start+length:]...)
}
