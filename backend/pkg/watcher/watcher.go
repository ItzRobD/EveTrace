package watcher

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	"EveTrace/pkg/core"
	"EveTrace/pkg/tailer"
)

const headerTimeLayout = "2006.01.02 15:04:05"

// OffsetFn returns the stored byte offset for a session ID, or 0 if the
// session has not been seen before. Pass nil to always start from the
// beginning of the file (useful for forced replay / testing).
type OffsetFn func(sessionID string) int64

// Session represents a single detected Eve log file.
// Lines is closed when the context is cancelled or the file watch fails.
type Session struct {
	Header        core.SessionHeader
	ID            string
	Lines         <-chan core.Line
	CurrentOffset func() int64 // call this to get the tailer's current read position
}

// Watcher monitors a directory for Eve log files and emits a Session per file.
// Sessions() is closed when the context is cancelled.
type Watcher struct {
	sessions chan Session
}

// New starts a Watcher on dir. offsetFn is called for each discovered file to
// determine where the tailer should resume reading; pass nil to always start
// from the beginning (equivalent to a first-run or forced replay).
func New(ctx context.Context, dir string, offsetFn OffsetFn) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := fw.Add(dir); err != nil {
		fw.Close()
		return nil, err
	}

	w := &Watcher{sessions: make(chan Session, 16)}
	go w.run(ctx, dir, offsetFn, fw)
	return w, nil
}

// Sessions returns the channel of detected sessions.
func (w *Watcher) Sessions() <-chan Session { return w.sessions }

func (w *Watcher) run(ctx context.Context, dir string, offsetFn OffsetFn, fw *fsnotify.Watcher) {
	defer fw.Close()
	defer close(w.sessions)

	// Initial scan for files already present in the directory.
	existing, _ := filepath.Glob(filepath.Join(dir, "*.txt"))
	for _, path := range existing {
		w.addFile(ctx, path, offsetFn)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-fw.Events:
			if !ok {
				return
			}
			if ev.Has(fsnotify.Create) && strings.HasSuffix(ev.Name, ".txt") {
				w.addFile(ctx, ev.Name, offsetFn)
			}
		case <-fw.Errors:
			// non-fatal
		}
	}
}

// addFile adds a file to the Watcher, creates a session, and starts tailing it for log lines.
// The session includes metadata, a unique ID, and current read offset.
func (w *Watcher) addFile(ctx context.Context, path string, offsetFn OffsetFn) {
	header, err := parseSessionHeaderWithRetry(ctx, path)
	if err != nil {
		return
	}

	if header.Collision {
		// Two characters share this file; the second character's data is interleaved
		// in a way the tailer cannot cleanly separate. Skip for now.
		fmt.Printf("watcher: skipping collision log %s\n", path)
		return
	}

	id := fmt.Sprintf("%s/%s", header.Character, header.StartedAt.Format("20060102-150405"))

	var startOffset int64
	if offsetFn != nil {
		startOffset = offsetFn(id)
	}

	t, err := tailer.New(ctx, path, startOffset)
	if err != nil {
		return
	}

	select {
	case w.sessions <- Session{
		Header:        header,
		ID:            id,
		Lines:         t.Lines(),
		CurrentOffset: t.CurrentOffset,
	}:
	case <-ctx.Done():
	}
}

// parseSessionHeaderWithRetry retries parseSessionHeader with exponential
// backoff up to ~3 seconds total. This handles the race where Eve creates the
// log file slightly before it finishes writing the header.
func parseSessionHeaderWithRetry(ctx context.Context, path string) (core.SessionHeader, error) {
	delay := 50 * time.Millisecond
	for range 6 { // 50 → 100 → 200 → 400 → 800 → 1600 ms (~3.1 s total)
		h, err := parseSessionHeader(path)
		if err == nil {
			return h, nil
		}
		select {
		case <-ctx.Done():
			return core.SessionHeader{}, ctx.Err()
		case <-time.After(delay):
			delay *= 2
		}
	}
	return core.SessionHeader{}, fmt.Errorf("could not read header from %s after retries", path)
}

// parseSessionHeader reads a file at the specified path and extracts a
// SessionHeader. It auto-detects the client language from the Listener label
// on header line 3 (0-indexed: line 2), and peeks at line 6 (index 5) to
// flag collision logs where two characters share a single file.
func parseSessionHeader(path string) (core.SessionHeader, error) {
	f, err := os.Open(path)
	if err != nil {
		return core.SessionHeader{}, err
	}
	defer f.Close()

	// Read up to 6 lines: 5 header lines + optional first content line.
	scanner := bufio.NewScanner(f)
	var raw [6]string
	for i := 0; i < 6; i++ {
		if !scanner.Scan() {
			if i < 5 {
				return core.SessionHeader{}, fmt.Errorf("header too short in %s (got %d lines)", path, i)
			}
			break // 6th line is optional
		}
		raw[i] = strings.TrimSpace(scanner.Text())
	}

	// Line index 2: Listener label — determines the client language.
	listenerLine := raw[2]
	var lang core.Language
	var charName string
	for l, lp := range core.Locales {
		if strings.HasPrefix(listenerLine, lp.ListenerLabel) {
			lang = l
			charName = strings.TrimSpace(listenerLine[len(lp.ListenerLabel):])
			break
		}
	}
	if charName == "" {
		return core.SessionHeader{}, fmt.Errorf("no Listener line found in %s", path)
	}

	// Line index 3: session start time.
	lp := core.Locales[lang]
	timeLine := raw[3]
	if !strings.HasPrefix(timeLine, lp.SessionTimeLabel) {
		return core.SessionHeader{}, fmt.Errorf("no session time line found in %s", path)
	}
	startedAt, err := time.Parse(headerTimeLayout, strings.TrimSpace(timeLine[len(lp.SessionTimeLabel):]))
	if err != nil {
		return core.SessionHeader{}, fmt.Errorf("parse session time in %s: %w", path, err)
	}

	// Line index 5: if it starts with "---" a second header block follows,
	// meaning two characters logged in at exactly the same second.
	collision := strings.HasPrefix(raw[5], "---")

	return core.SessionHeader{
		Character: charName,
		StartedAt: startedAt,
		Language:  lang,
		Collision: collision,
	}, nil
}
