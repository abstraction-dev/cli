// Package history persists recent question/answer exchanges to a local file so
// past conversations can be reviewed offline. It is deliberately independent of
// the API client: history is written from whatever the CLI already rendered.
package history

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const fileName = ".abstr-history.json"

// envPath overrides where history is stored, mainly for tests.
const envPath = "ABSTR_HISTORY"

// Entry is one recorded exchange.
type Entry struct {
	AskedAt   time.Time `json:"asked_at"`
	Workspace string    `json:"workspace"`
	Question  string    `json:"question"`
	Answer    string    `json:"answer"`
}

// Store is an append-only history file, trimmed to the newest limit entries.
type Store struct {
	path  string
	limit int
}

// Path returns the history file location: $ABSTR_HISTORY, else
// ~/.abstr-history.json.
func Path() (string, error) {
	if p := os.Getenv(envPath); p != "" {
		return p, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, fileName), nil
}

// Open returns a Store at the default path, keeping at most limit entries. A
// limit of zero or less disables writes, so history can be turned off without
// the callers branching.
func Open(limit int) (*Store, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	return &Store{path: path, limit: limit}, nil
}

// At returns a Store at an explicit path.
func At(path string, limit int) *Store {
	return &Store{path: path, limit: limit}
}

// FilePath returns the resolved history path.
func (s *Store) FilePath() string { return s.path }

// Enabled reports whether this store writes anything.
func (s *Store) Enabled() bool { return s.limit > 0 }

// Append records one exchange, trimming the file to the newest limit entries.
// A blank question or answer is dropped: a cancelled turn is not history.
func (s *Store) Append(e Entry) error {
	if !s.Enabled() || strings.TrimSpace(e.Question) == "" || strings.TrimSpace(e.Answer) == "" {
		return nil
	}

	entries, err := s.All()
	if err != nil {
		return err
	}

	if e.AskedAt.IsZero() {
		e.AskedAt = time.Now()
	}
	entries = append(entries, e)

	if len(entries) > s.limit {
		entries = entries[len(entries)-s.limit:]
	}
	return s.write(entries)
}

// All returns every stored entry, oldest first. A missing file is empty, not an
// error, so a first run reads cleanly.
func (s *Store) All() ([]Entry, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// Recent returns the newest n entries, newest first.
func (s *Store) Recent(n int) ([]Entry, error) {
	entries, err := s.All()
	if err != nil {
		return nil, err
	}

	if n > 0 && len(entries) > n {
		entries = entries[len(entries)-n:]
	}

	out := make([]Entry, 0, len(entries))
	for i := len(entries) - 1; i >= 0; i-- {
		out = append(out, entries[i])
	}
	return out, nil
}

// Clear removes the history file. A missing file is a no-op.
func (s *Store) Clear() error {
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Headline is the entry's question collapsed to a single line, truncated to
// width runes so a listing stays one row per entry.
func (e Entry) Headline(width int) string {
	q := strings.Join(strings.Fields(e.Question), " ")
	if width <= 0 || len([]rune(q)) <= width {
		return q
	}
	return string([]rune(q)[:width-1]) + "…"
}

func (s *Store) write(entries []Entry) error {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, append(data, '\n'), 0o600)
}
