package history

import (
	"bufio"
	"encoding/json"
	"os"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/liamg/readline/pkg/editor/suggestion"
)

type History interface {
	// Reset moves the pointer back to AFTER the most recent entry, so the next call to Previous returns the most recent entry.
	// Also removes any filtering
	Reset()
	// SetFilter sets a filter string. Only entries that contain the filter will be returned by Previous and Next. Reset removes the filter.
	SetFilter(filter string)
	// SetPrefixFilter sets a filter string. Only entries that start with the filter will be returned by Previous and Next. Reset removes the filter.
	SetPrefixFilter(filter string)
	// Previous returns the previous entry in the history, or an empty string if there are no more entries.
	Previous() Entry
	// Next returns the next entry in the history, or an empty string if there are no more entries.
	Next() Entry
	// Append adds a new entry to the history. If permanent is true, the entry will be saved to disk and available in future sessions. If false, the entry will only be available in the current session.
	Append(line string, permanent bool)
	// IsCurrent returns true if the pointer is currently after the most recent entry, i.e. the next call to Previous will return the most recent entry.
	IsCurrent() bool
}

type DefaultImpl struct {
	path           string
	maxEntries     int
	lastRead       time.Time
	fileEntries    []Entry // entries read from the shared history file
	sessionEntries []Entry // non-permanent entries only in this session
	index          int     // zero = after most recent entry
	filter         string
	prefix         string

	// sortedTexts is a pre-sorted (ascending time) slice of entry texts rebuilt
	// only when fileEntries or sessionEntries grow. Avoids re-sorting on every
	// full suggestion search.
	suggestSortedTexts []string
	suggestFileLen     int
	suggestSessionLen  int

	// per-input cache: all texts from sortedTexts that matched the last prefix,
	// most recent last. Filtered in-place on each additional keystroke.
	suggestLastPrefix string
	suggestCandidates []string
}

var _ suggestion.Suggester = (*DefaultImpl)(nil)

func NewDefaultImplementation(path string, maxEntries int) *DefaultImpl {
	impl := &DefaultImpl{
		path:       path,
		maxEntries: maxEntries,
	}
	impl.prune()
	impl.syncFromFile()
	return impl
}

var _ History = (*DefaultImpl)(nil)

type Entry struct {
	Text string `json:"t"`
	At   int64  `json:"a,omitempty"`
}

// Empty is a no-op history implementation for editors that do not persist or
// navigate command history.
type Empty struct{}

func (Empty) Reset()                 {}
func (Empty) SetFilter(string)       {}
func (Empty) SetPrefixFilter(string) {}
func (Empty) Previous() Entry        { return Entry{} }
func (Empty) Next() Entry            { return Entry{} }
func (Empty) Append(string, bool)    {}
func (Empty) IsCurrent() bool        { return true }

func (d *DefaultImpl) IsCurrent() bool {
	return d.index == 0
}

// sortedSuggestionTexts returns a slice of all entry texts sorted ascending by
// time (most recent last). It is rebuilt only when entries have been added since
// the last call, so the sort runs at most once per new history entry.
func (d *DefaultImpl) sortedSuggestionTexts() []string {
	if len(d.fileEntries) == d.suggestFileLen &&
		len(d.sessionEntries) == d.suggestSessionLen {
		return d.suggestSortedTexts
	}
	entries := make([]Entry, 0, len(d.fileEntries)+len(d.sessionEntries))
	entries = append(entries, d.fileEntries...)
	entries = append(entries, d.sessionEntries...)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].At < entries[j].At
	})
	texts := make([]string, len(entries))
	for i, e := range entries {
		texts[i] = e.Text
	}
	d.suggestSortedTexts = texts
	d.suggestFileLen = len(d.fileEntries)
	d.suggestSessionLen = len(d.sessionEntries)
	return texts
}

// Suggest returns the suffix of the most recent history entry that starts with
// the current buffer contents.
//
// On the first call (or cache miss) it searches the pre-sorted text cache and
// stores all matches in suggestCandidates. On subsequent calls that extend the
// prefix it filters suggestCandidates in-place — no allocation, no full scan.
func (d *DefaultImpl) Suggest(r []rune) []rune {
	if len(r) == 0 {
		d.suggestLastPrefix = ""
		d.suggestCandidates = nil
		return nil
	}

	prefix := string(r)

	// Fast path: new input extends the last prefix — filter candidates in-place.
	if d.suggestCandidates != nil && strings.HasPrefix(prefix, d.suggestLastPrefix) {
		filtered := d.suggestCandidates[:0]
		for _, text := range d.suggestCandidates {
			if strings.HasPrefix(text, prefix) && text != prefix {
				filtered = append(filtered, text)
			}
		}
		d.suggestCandidates = filtered
		d.suggestLastPrefix = prefix
		if len(filtered) == 0 {
			return nil
		}
		return []rune(filtered[len(filtered)-1][len(prefix):])
	}

	// Full search against the pre-sorted text cache.
	texts := d.sortedSuggestionTexts()
	candidates := d.suggestCandidates[:0] // reuse allocation if available
	for _, text := range texts {
		if strings.HasPrefix(text, prefix) && text != prefix {
			candidates = append(candidates, text)
		}
	}
	d.suggestLastPrefix = prefix
	d.suggestCandidates = candidates
	if len(candidates) == 0 {
		return nil
	}
	return []rune(candidates[len(candidates)-1][len(prefix):])
}

func (d *DefaultImpl) Reset() {
	d.index = 0
	d.filter = ""
	d.prefix = ""
	d.suggestLastPrefix = ""
	d.suggestCandidates = nil
}

func (d *DefaultImpl) getCombinedEntries() []Entry {
	d.syncFromFile()

	entries := make([]Entry, 0, len(d.fileEntries)+len(d.sessionEntries))
	entries = append(entries, d.fileEntries...)
	entries = append(entries, d.sessionEntries...)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].At < entries[j].At
	})

	if d.filter == "" && d.prefix == "" {
		return entries
	}

	filtered := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if d.filter != "" && !strings.Contains(entry.Text, d.filter) {
			continue
		}
		if d.prefix != "" && !strings.HasPrefix(entry.Text, d.prefix) {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func (d *DefaultImpl) SetFilter(filter string) {
	d.filter = filter
}

func (d *DefaultImpl) SetPrefixFilter(prefix string) {
	d.prefix = prefix
}

func (d *DefaultImpl) Previous() Entry {
	entries := d.getCombinedEntries()
	if len(entries) == 0 {
		return Entry{}
	}
	if d.index < len(entries) {
		d.index++
	}

	return entries[len(entries)-d.index]
}

func (d *DefaultImpl) Next() Entry {
	if d.index > 0 {
		d.index--
	}
	if d.index == 0 {
		return Entry{}
	}
	entries := d.getCombinedEntries()
	if len(entries) == 0 || d.index > len(entries) {
		return Entry{}
	}
	return entries[len(entries)-d.index]
}

func (d *DefaultImpl) prune() {
	f, err := os.OpenFile(d.path, os.O_RDWR, 0o600)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
	scanner := bufio.NewScanner(f)
	entries := make([]Entry, 0, d.maxEntries)
	for scanner.Scan() {
		var entry Entry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue // skip corrupt lines
		}
		entries = append(entries, entry)
	}
	if len(entries) > d.maxEntries {
		entries = entries[len(entries)-d.maxEntries:]
	}
	_, _ = f.Seek(0, 0)
	_ = f.Truncate(0)
	enc := json.NewEncoder(f)
	for _, entry := range entries {
		_ = enc.Encode(entry)
	}
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}

func (d *DefaultImpl) syncFromFile() {
	stat, err := os.Stat(d.path)
	if err != nil {
		return
	}
	if !stat.ModTime().After(d.lastRead) {
		return
	}
	d.fileEntries = d.readEntireFile()
	d.lastRead = stat.ModTime()
}

func (d *DefaultImpl) writeToFile(entry Entry) {
	d.syncFromFile()
	f, err := os.OpenFile(d.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	_ = json.NewEncoder(f).Encode(entry)
	_ = f.Close()
	d.fileEntries = append(d.fileEntries, entry)
	stat, err := os.Stat(d.path)
	if err == nil {
		d.lastRead = stat.ModTime()
	}
}

func (d *DefaultImpl) readEntireFile() []Entry {
	f, err := os.OpenFile(d.path, os.O_RDONLY, 0o600)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)

	entries := make([]Entry, 0, d.maxEntries)

	for scanner.Scan() {
		var entry Entry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue // skip corrupt lines
		}
		entries = append(entries, entry)
	}

	return entries
}

func (d *DefaultImpl) Append(line string, permanent bool) {
	if strings.HasPrefix(line, " ") {
		return
	}
	entry := Entry{
		Text: line,
		At:   time.Now().UnixMilli(),
	}
	if permanent {
		d.writeToFile(entry)
	} else {
		d.sessionEntries = append(d.sessionEntries, entry)
	}
}
