package apiclient

// sseReader is a lightweight Server-Sent Events reader. It is a copy of
// llmkit/internal/sse/reader.go (that package is internal to the llmkit module
// and cannot be imported from here), trimmed to what the CLI needs.

import (
	"bufio"
	"io"
	"strings"
)

type sseEvent struct {
	Type string // from "event:" line; empty if none
	Data string // concatenated "data:" lines (joined by "\n")
}

type sseReader struct {
	scanner *bufio.Scanner
	body    io.ReadCloser
}

func newSSEReader(body io.ReadCloser) *sseReader {
	s := bufio.NewScanner(body)
	// Allow up to 1 MB per line for large frames.
	s.Buffer(make([]byte, 4096), 1<<20)
	return &sseReader{scanner: s, body: body}
}

// next reads the next SSE event, or io.EOF when the stream ends.
func (r *sseReader) next() (sseEvent, error) {
	var ev sseEvent
	var dataLines []string
	hasData := false

	for r.scanner.Scan() {
		line := r.scanner.Text()

		if line == "" {
			// Dispatch on a blank line. Return the event if it carries data OR a
			// type; a bare `event: <type>` frame (e.g. stream_end) is a real event
			// and must be returned, not swallowed — otherwise its type would bleed
			// onto whatever event comes next.
			if hasData || ev.Type != "" {
				ev.Data = strings.Join(dataLines, "\n")
				return ev, nil
			}
			continue
		}

		if strings.HasPrefix(line, ":") { // comment / keep-alive
			continue
		}

		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "data":
			dataLines = append(dataLines, value)
			hasData = true
		case "event":
			ev.Type = value
		}
	}

	if err := r.scanner.Err(); err != nil {
		return sseEvent{}, err
	}
	if hasData || ev.Type != "" {
		ev.Data = strings.Join(dataLines, "\n")
		return ev, nil
	}
	return sseEvent{}, io.EOF
}
