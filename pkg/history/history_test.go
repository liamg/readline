package history_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/liamg/readline/pkg/history"
)

func testImpl(t *testing.T, maxEntries int) (*history.DefaultImpl, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "history")
	return history.NewDefaultImplementation(path, maxEntries), path
}

func TestHistoryPersistsToSession(t *testing.T) {
	h, path := testImpl(t, 10)
	start := time.Now()
	h.Append("hello world", false)
	entry := h.Previous()
	if entry.Text != "hello world" {
		t.Errorf("expected log entry to be \"hello world\", got %q", entry.Text)
	}
	if entry.At < start.UnixMilli() || entry.At > time.Now().UnixMilli() {
		t.Errorf("entry timestamp was incorrect: %d", entry.At)
	}
	_, err := os.Stat(path)
	if err == nil {
		t.Errorf("history file should not have been created, but was")
	}
}

func TestHistoryPrefixedWithSpaceDoesNotPersist(t *testing.T) {
	h, path := testImpl(t, 10)
	h.Append(" hello world", false)
	entry := h.Previous()
	if entry.Text != "" {
		t.Errorf("expected log entry not be persisted, but it was")
	}
	_, err := os.Stat(path)
	if err == nil {
		t.Errorf("history file should not have been created, but was")
	}
}

func TestHistoryPersistsToFile(t *testing.T) {
	h, path := testImpl(t, 10)
	start := time.Now()
	h.Append("hello world", true)
	entry := h.Previous()
	if entry.Text != "hello world" {
		t.Errorf("expected log entry to be \"hello world\", got %q", entry.Text)
	}
	if entry.At < start.UnixMilli() || entry.At > time.Now().UnixMilli() {
		t.Errorf("entry timestamp was incorrect: %d", entry.At)
	}
	h2 := history.NewDefaultImplementation(path, 10)
	entry = h2.Previous()
	if entry.Text != "hello world" {
		t.Errorf("expected log entry to be \"hello world\", got %q", entry.Text)
	}
	if entry.At < start.UnixMilli() || entry.At > time.Now().UnixMilli() {
		t.Errorf("entry timestamp was incorrect: %d", entry.At)
	}
}

func TestHistoryIsMultiplexed(t *testing.T) {
	h, path := testImpl(t, 10)
	h2 := history.NewDefaultImplementation(path, 10)

	// the sleeps here are necessary, as we only store entries with millisecond precision
	h.Append("A", true)
	time.Sleep(time.Millisecond)
	h2.Append("B", true)
	time.Sleep(time.Millisecond)
	h2.Append("C", true)
	time.Sleep(time.Millisecond)
	h.Append("D", true)

	for i, expected := range []string{"D", "C", "B", "A"} {
		if prev := h.Previous(); prev.Text != expected {
			t.Errorf("expected history (first) entry #%d to be %q, not %q", i+1, expected, prev.Text)
		}
		if prev := h2.Previous(); prev.Text != expected {
			t.Errorf("expected history (second) entry #%d to be %q, not %q", i+1, expected, prev.Text)
		}
	}
}

func TestPreviousOnEmptyHistory(t *testing.T) {
	h, _ := testImpl(t, 10)
	entry := h.Previous()
	if entry.Text != "" {
		t.Errorf("expected empty text from empty history, got %q", entry.Text)
	}
	if entry.At != 0 {
		t.Errorf("expected zero timestamp from empty history, got %d", entry.At)
	}
}

func TestNextOnEmptyHistory(t *testing.T) {
	h, _ := testImpl(t, 10)
	entry := h.Next()
	if entry.Text != "" {
		t.Errorf("expected empty text from empty history, got %q", entry.Text)
	}
}

func TestPreviousDoesNotDuplicateEntries(t *testing.T) {
	h, _ := testImpl(t, 100)
	h.Append("one", true)
	h.Append("two", true)
	h.Append("three", true)

	// Navigate all the way back
	for _, expected := range []string{"three", "two", "one"} {
		entry := h.Previous()
		if entry.Text != expected {
			t.Fatalf("expected %q, got %q", expected, entry.Text)
		}
	}

	// Should stay on the oldest entry when going past the beginning
	entry := h.Previous()
	if entry.Text != "one" {
		t.Errorf("expected to stay on oldest entry %q, got %q", "one", entry.Text)
	}
}

func TestRepeatedPreviousReturnsDifferentEntries(t *testing.T) {
	h, _ := testImpl(t, 100)
	h.Append("A", true)
	time.Sleep(time.Millisecond)
	h.Append("B", true)
	time.Sleep(time.Millisecond)
	h.Append("C", true)

	seen := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		seen = append(seen, h.Previous().Text)
	}
	if seen[0] != "C" || seen[1] != "B" || seen[2] != "A" {
		t.Errorf("expected [C B A], got %v", seen)
	}
}

func TestPreviousThenNextNavigation(t *testing.T) {
	h, _ := testImpl(t, 10)
	h.Append("first", true)
	time.Sleep(time.Millisecond)
	h.Append("second", true)
	time.Sleep(time.Millisecond)
	h.Append("third", true)

	// Go back two
	e := h.Previous()
	if e.Text != "third" {
		t.Errorf("expected %q, got %q", "third", e.Text)
	}
	e = h.Previous()
	if e.Text != "second" {
		t.Errorf("expected %q, got %q", "second", e.Text)
	}

	// Go forward one
	e = h.Next()
	if e.Text != "third" {
		t.Errorf("expected %q, got %q", "third", e.Text)
	}

	// Go forward past the end returns empty
	e = h.Next()
	if e.Text != "" {
		t.Errorf("expected empty at end, got %q", e.Text)
	}

	// Go back again from the end
	e = h.Previous()
	if e.Text != "third" {
		t.Errorf("expected %q after reset to end, got %q", "third", e.Text)
	}
}

func TestResetMovesToEnd(t *testing.T) {
	h, _ := testImpl(t, 10)
	h.Append("A", true)
	time.Sleep(time.Millisecond)
	h.Append("B", true)

	// Navigate back
	h.Previous() // B
	h.Previous() // A

	// Reset should move back to after most recent
	h.Reset()

	e := h.Previous()
	if e.Text != "B" {
		t.Errorf("after reset, Previous should return most recent %q, got %q", "B", e.Text)
	}
}

func TestResetClearsFilter(t *testing.T) {
	h, _ := testImpl(t, 10)
	h.Append("apple", true)
	time.Sleep(time.Millisecond)
	h.Append("banana", true)
	time.Sleep(time.Millisecond)
	h.Append("avocado", true)

	h.SetFilter("an")
	e := h.Previous()
	if e.Text != "banana" {
		t.Errorf("expected filtered result %q, got %q", "banana", e.Text)
	}

	h.Reset()
	e = h.Previous()
	if e.Text != "avocado" {
		t.Errorf("after reset, filter should be cleared, expected %q, got %q", "avocado", e.Text)
	}
}

func TestSetFilter(t *testing.T) {
	h, _ := testImpl(t, 10)
	h.Append("apple", true)
	time.Sleep(time.Millisecond)
	h.Append("banana", true)
	time.Sleep(time.Millisecond)
	h.Append("apricot", true)
	time.Sleep(time.Millisecond)
	h.Append("cherry", true)

	h.SetFilter("ap")
	entries := collectPrevious(h, 10)
	if len(entries) != 2 {
		t.Fatalf("expected 2 filtered entries, got %d: %v", len(entries), entries)
	}
	if entries[0] != "apricot" || entries[1] != "apple" {
		t.Errorf("expected [apricot apple], got %v", entries)
	}
}

func TestSetPrefixFilter(t *testing.T) {
	h, _ := testImpl(t, 10)
	h.Append("git commit", true)
	time.Sleep(time.Millisecond)
	h.Append("go test", true)
	time.Sleep(time.Millisecond)
	h.Append("git push", true)
	time.Sleep(time.Millisecond)
	h.Append("grep foo", true)

	h.SetPrefixFilter("git")
	entries := collectPrevious(h, 10)
	if len(entries) != 2 {
		t.Fatalf("expected 2 prefix-filtered entries, got %d: %v", len(entries), entries)
	}
	if entries[0] != "git push" || entries[1] != "git commit" {
		t.Errorf("expected [git push, git commit], got %v", entries)
	}
}

func TestSessionOnlyEntriesNotWrittenToFile(t *testing.T) {
	h, path := testImpl(t, 10)
	h.Append("persistent", true)
	h.Append("ephemeral", false)

	h2 := history.NewDefaultImplementation(path, 10)
	e := h2.Previous()
	if e.Text != "persistent" {
		t.Errorf("expected new instance to see only %q, got %q", "persistent", e.Text)
	}
	e = h2.Previous()
	if e.Text != "persistent" {
		t.Errorf("expected no more entries, staying on %q, got %q", "persistent", e.Text)
	}
}

func TestSessionEntriesVisibleLocally(t *testing.T) {
	h, _ := testImpl(t, 10)
	h.Append("from-file", true)
	time.Sleep(time.Millisecond)
	h.Append("in-memory", false)

	entries := collectPrevious(h, 10)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(entries), entries)
	}
	if entries[0] != "in-memory" || entries[1] != "from-file" {
		t.Errorf("expected [in-memory, from-file], got %v", entries)
	}
}

func TestMaxEntriesPrune(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history")
	h := history.NewDefaultImplementation(path, 3)

	for i := 0; i < 5; i++ {
		h.Append("entry-"+string(rune('A'+i)), true)
		time.Sleep(time.Millisecond)
	}

	// Re-open to trigger prune
	h2 := history.NewDefaultImplementation(path, 3)
	entries := collectPrevious(h2, 10)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries after prune, got %d: %v", len(entries), entries)
	}
	// Should keep the 3 most recent
	if entries[0] != "entry-E" || entries[1] != "entry-D" || entries[2] != "entry-C" {
		t.Errorf("expected most recent 3 entries, got %v", entries)
	}
}

func TestCorruptLinesSkipped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history")

	// Write a mix of valid and corrupt lines
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = json.NewEncoder(f).Encode(history.Entry{Text: "good1", At: 1})
	_, _ = f.WriteString("this is not json\n")
	_ = json.NewEncoder(f).Encode(history.Entry{Text: "good2", At: 2})
	_, _ = f.WriteString("{\"invalid\n")
	_ = json.NewEncoder(f).Encode(history.Entry{Text: "good3", At: 3})
	_ = f.Close()

	h := history.NewDefaultImplementation(path, 100)
	entries := collectPrevious(h, 10)
	if len(entries) != 3 {
		t.Fatalf("expected 3 valid entries, got %d: %v", len(entries), entries)
	}
	if entries[0] != "good3" || entries[1] != "good2" || entries[2] != "good1" {
		t.Errorf("expected [good3, good2, good1], got %v", entries)
	}
}

func TestMultiplexedSyncPicksUpExternalWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history")
	h1 := history.NewDefaultImplementation(path, 100)
	h2 := history.NewDefaultImplementation(path, 100)

	h1.Append("from-h1", true)
	time.Sleep(time.Millisecond)

	// h2 should see h1's entry on next Previous
	e := h2.Previous()
	if e.Text != "from-h1" {
		t.Errorf("h2 should see h1's entry, got %q", e.Text)
	}
}

func TestMultiplexedInterleavedAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history")
	h1 := history.NewDefaultImplementation(path, 100)
	h2 := history.NewDefaultImplementation(path, 100)
	h3 := history.NewDefaultImplementation(path, 100)

	h1.Append("h1-first", true)
	time.Sleep(time.Millisecond)
	h2.Append("h2-first", true)
	time.Sleep(time.Millisecond)
	h3.Append("h3-first", true)
	time.Sleep(time.Millisecond)
	h1.Append("h1-second", true)
	time.Sleep(time.Millisecond)
	h2.Append("h2-second", true)

	expected := []string{"h2-second", "h1-second", "h3-first", "h2-first", "h1-first"}
	for i, exp := range expected {
		for name, h := range map[string]*history.DefaultImpl{"h1": h1, "h2": h2, "h3": h3} {
			e := h.Previous()
			if e.Text != exp {
				t.Errorf("%s: entry #%d expected %q, got %q", name, i+1, exp, e.Text)
			}
		}
	}
}

func TestNavigationStaysInBounds(t *testing.T) {
	h, _ := testImpl(t, 10)
	h.Append("only", true)

	// Going back past beginning should clamp
	e1 := h.Previous()
	e2 := h.Previous()
	e3 := h.Previous()
	if e1.Text != "only" || e2.Text != "only" || e3.Text != "only" {
		t.Errorf("expected clamped to %q, got [%q, %q, %q]", "only", e1.Text, e2.Text, e3.Text)
	}

	// Going forward past end should return empty
	n1 := h.Next()
	if n1.Text != "" {
		t.Errorf("expected empty after going forward from only entry, got %q", n1.Text)
	}
	n2 := h.Next()
	if n2.Text != "" {
		t.Errorf("expected empty stays empty, got %q", n2.Text)
	}
}

func TestNextWithoutPreviousReturnsEmpty(t *testing.T) {
	h, _ := testImpl(t, 10)
	h.Append("something", true)

	e := h.Next()
	if e.Text != "" {
		t.Errorf("Next without Previous should return empty, got %q", e.Text)
	}
}

func TestLeadingSpaceIgnoredForPermanent(t *testing.T) {
	h, path := testImpl(t, 10)
	h.Append(" secret command", true)

	e := h.Previous()
	if e.Text != "" {
		t.Errorf("leading-space entry should not be stored, got %q", e.Text)
	}

	// File should not exist since we only tried to add a space-prefixed entry
	_, err := os.Stat(path)
	if err == nil {
		t.Error("history file should not have been created for space-prefixed entry")
	}
}

func TestFilterWithNavigation(t *testing.T) {
	h, _ := testImpl(t, 10)
	h.Append("ls -la", true)
	time.Sleep(time.Millisecond)
	h.Append("cd /tmp", true)
	time.Sleep(time.Millisecond)
	h.Append("ls -R", true)
	time.Sleep(time.Millisecond)
	h.Append("pwd", true)
	time.Sleep(time.Millisecond)
	h.Append("ls", true)

	h.SetFilter("ls")

	// Should only navigate through entries containing "ls"
	e1 := h.Previous()
	e2 := h.Previous()
	e3 := h.Previous()

	if e1.Text != "ls" {
		t.Errorf("filtered #1 expected %q, got %q", "ls", e1.Text)
	}
	if e2.Text != "ls -R" {
		t.Errorf("filtered #2 expected %q, got %q", "ls -R", e2.Text)
	}
	if e3.Text != "ls -la" {
		t.Errorf("filtered #3 expected %q, got %q", "ls -la", e3.Text)
	}

	// Navigate forward through filtered results
	n1 := h.Next()
	if n1.Text != "ls -R" {
		t.Errorf("filtered Next expected %q, got %q", "ls -R", n1.Text)
	}
}

func TestPrefixFilterWithNavigation(t *testing.T) {
	h, _ := testImpl(t, 10)
	h.Append("git status", true)
	time.Sleep(time.Millisecond)
	h.Append("go build", true)
	time.Sleep(time.Millisecond)
	h.Append("git log", true)
	time.Sleep(time.Millisecond)
	h.Append("go test", true)

	h.SetPrefixFilter("go")
	entries := collectPrevious(h, 10)
	if len(entries) != 2 {
		t.Fatalf("expected 2 prefix matches, got %d: %v", len(entries), entries)
	}
	if entries[0] != "go test" || entries[1] != "go build" {
		t.Errorf("expected [go test, go build], got %v", entries)
	}
}

func TestCombinedFilterAndPrefix(t *testing.T) {
	h, _ := testImpl(t, 10)
	h.Append("git commit -m fix", true)
	time.Sleep(time.Millisecond)
	h.Append("git push", true)
	time.Sleep(time.Millisecond)
	h.Append("go test -run fix", true)
	time.Sleep(time.Millisecond)
	h.Append("git commit -m feat", true)

	h.SetPrefixFilter("git")
	h.SetFilter("commit")

	entries := collectPrevious(h, 10)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries matching both filters, got %d: %v", len(entries), entries)
	}
	if entries[0] != "git commit -m feat" || entries[1] != "git commit -m fix" {
		t.Errorf("expected [git commit -m feat, git commit -m fix], got %v", entries)
	}
}

func TestEmptyHistoryFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history")
	if err := os.WriteFile(path, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}
	h := history.NewDefaultImplementation(path, 10)
	e := h.Previous()
	if e.Text != "" {
		t.Errorf("expected empty from empty file, got %q", e.Text)
	}
}

func TestNewInstanceFromPopulatedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history")
	h := history.NewDefaultImplementation(path, 100)
	h.Append("line1", true)
	time.Sleep(time.Millisecond)
	h.Append("line2", true)
	time.Sleep(time.Millisecond)
	h.Append("line3", true)

	// Create a fresh instance pointing to the same file
	h2 := history.NewDefaultImplementation(path, 100)
	entries := collectPrevious(h2, 10)
	if len(entries) != 3 {
		t.Fatalf("new instance expected 3 entries, got %d: %v", len(entries), entries)
	}
	if entries[0] != "line3" || entries[1] != "line2" || entries[2] != "line1" {
		t.Errorf("expected [line3, line2, line1], got %v", entries)
	}
}

func TestResetDuringNavigation(t *testing.T) {
	h, _ := testImpl(t, 10)
	h.Append("A", true)
	time.Sleep(time.Millisecond)
	h.Append("B", true)
	time.Sleep(time.Millisecond)
	h.Append("C", true)

	h.Previous() // C
	h.Previous() // B
	h.Previous() // A

	h.Reset()

	// After reset, we should be back at the end
	e := h.Previous()
	if e.Text != "C" {
		t.Errorf("after reset during deep navigation, expected %q, got %q", "C", e.Text)
	}
}

func TestFilterNoMatches(t *testing.T) {
	h, _ := testImpl(t, 10)
	h.Append("hello", true)
	h.Append("world", true)

	h.SetFilter("xyz")
	e := h.Previous()
	if e.Text != "" {
		t.Errorf("expected empty for filter with no matches, got %q", e.Text)
	}
}

func TestManyEntriesNavigation(t *testing.T) {
	h, _ := testImpl(t, 1000)
	for i := 0; i < 100; i++ {
		h.Append("cmd-"+string(rune('A'+i%26))+"-"+string(rune('0'+i/26)), true)
	}

	// Navigate all the way back
	var count int
	seen := make(map[string]bool)
	for {
		e := h.Previous()
		if _, dup := seen[e.Text]; dup {
			break // clamped at oldest
		}
		seen[e.Text] = true
		count++
	}
	if count != 100 {
		t.Errorf("expected to navigate through 100 unique entries, got %d", count)
	}
}

func TestExternalFileModification(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history")
	h := history.NewDefaultImplementation(path, 100)
	h.Append("original", true)

	// Simulate another process writing directly to the file
	time.Sleep(10 * time.Millisecond)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	entry := history.Entry{Text: "external", At: time.Now().UnixMilli()}
	_ = json.NewEncoder(f).Encode(entry)
	_ = f.Close()

	// h should pick up the external entry
	e := h.Previous()
	if e.Text != "external" {
		t.Errorf("expected to pick up external entry %q, got %q", "external", e.Text)
	}
	e = h.Previous()
	if e.Text != "original" {
		t.Errorf("expected original entry %q, got %q", "original", e.Text)
	}
}

func TestNonExistentPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deep", "nested", "history")
	h := history.NewDefaultImplementation(path, 10)
	e := h.Previous()
	if e.Text != "" {
		t.Errorf("expected empty from non-existent path, got %q", e.Text)
	}
}

// collectPrevious calls Previous repeatedly and returns the unique entry texts
// in order, stopping when it hits a duplicate (clamped at oldest).
func collectPrevious(h history.History, max int) []string {
	var results []string
	var last string
	for i := 0; i < max; i++ {
		e := h.Previous()
		if e.Text == "" {
			break
		}
		if e.Text == last {
			break
		}
		last = e.Text
		results = append(results, e.Text)
	}
	return results
}
