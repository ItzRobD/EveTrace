package watcher

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	"EveTrace/pkg/core"
	"EveTrace/pkg/tailer"
)

// errNoHeader is returned when a file contains no separator block at all
// (empty files, non-Eve files). Silently ignored — no log, no retries.
var errNoHeader = errors.New("no header block in file")

// errNoListener is returned when the header block exists but contains no
// Listener line — a session with no character attached. Logged at INFO.
var errNoListener = errors.New("no listener in header")

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
	LogPath       string
	Lines         <-chan core.Line
	CurrentOffset func() int64 // call this to get the tailer's current read position
}

// Watcher monitors a directory for Eve log files and emits a Session per file.
// Sessions() and LogEvents() are both closed when the context is cancelled.
type Watcher struct {
	sessions  chan Session
	logEvents chan core.LogEvent
	minDate   time.Time
}

// New starts a Watcher on dir. offsetFn is called for each discovered file to
// determine where the tailer should resume reading; pass nil to always start
// from the beginning (equivalent to a first-run or forced replay).
func New(ctx context.Context, dir string, offsetFn OffsetFn, minDate time.Time) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := fw.Add(dir); err != nil {
		fw.Close()
		return nil, err
	}
	// A zero minDate means "no filter": StartedAt.Before(zero) is always false.

	w := &Watcher{
		sessions:  make(chan Session, 16),
		logEvents: make(chan core.LogEvent, 64),
		minDate:   minDate,
	}
	go w.run(ctx, dir, offsetFn, fw)
	return w, nil
}

// Sessions returns the channel of detected sessions.
func (w *Watcher) Sessions() <-chan Session { return w.sessions }

// LogEvents returns the channel of structured log events. Consumers must drain
// this channel; it is closed when the context is cancelled.
func (w *Watcher) LogEvents() <-chan core.LogEvent { return w.logEvents }

func (w *Watcher) run(ctx context.Context, dir string, offsetFn OffsetFn, fw *fsnotify.Watcher) {
	defer fw.Close()
	defer close(w.sessions)
	defer close(w.logEvents)

	// writes maps each active file path to the channel used by its tailer.
	// Accessed only from this goroutine — no mutex needed.
	writes := make(map[string]chan struct{})

	// Initial scan for files already present in the directory.
	existing, _ := filepath.Glob(filepath.Join(dir, "*.txt"))
	for _, path := range existing {
		w.addFile(ctx, path, offsetFn, writes)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-fw.Events:
			if !ok {
				return
			}
			if !strings.HasSuffix(ev.Name, ".txt") {
				continue
			}
			if ev.Has(fsnotify.Create) {
				w.addFile(ctx, ev.Name, offsetFn, writes)
			}
			if ev.Has(fsnotify.Write) {
				if ch, ok := writes[ev.Name]; ok {
					select {
					case ch <- struct{}{}:
					default: // tailer hasn't drained yet — coalesce
					}
				}
			}
		case <-fw.Errors:
			// non-fatal
		}
	}
}

// addFile parses the session header, creates a tailer, and emits the Session.
func (w *Watcher) addFile(ctx context.Context, path string, offsetFn OffsetFn, writes map[string]chan struct{}) {
	header, err := parseSessionHeaderWithRetry(ctx, path)
	if err != nil {
		switch {
		case errors.Is(err, errNoHeader):
			// Empty or non-Eve file — silently ignore.
		case errors.Is(err, errNoListener):
			w.emit(ctx, core.LogEvent{
				Level:   core.LevelInfo,
				Code:    core.CodeNoListener,
				File:    path,
				Message: fmt.Sprintf("ignoring %s: no character attached (header-only file)", filepath.Base(path)),
				At:      time.Now(),
			})
		default:
			w.emit(ctx, core.LogEvent{
				Level:   core.LevelWarn,
				Code:    core.CodeHeaderParseFail,
				File:    path,
				Message: fmt.Sprintf("could not read session header from %s: %v", path, err),
				At:      time.Now(),
			})
		}
		return
	}

	if header.Collision {
		w.emit(ctx, core.LogEvent{
			Level:   core.LevelWarn,
			Code:    core.CodeCollision,
			File:    path,
			Message: fmt.Sprintf("log file %s contains two session headers (characters logged in at the same second); file skipped", path),
			At:      time.Now(),
		})
		return
	}

	if header.StartedAt.Before(w.minDate) {
		// Silently skip logs before the minimum date
		return
	}

	id := fmt.Sprintf("%s/%s", header.Character, header.StartedAt.Format("20060102-150405"))

	var startOffset int64
	if offsetFn != nil {
		startOffset = offsetFn(id)
	}

	// Create the write-notification channel for this file and register it so
	// the run loop can forward fsnotify Write events to the tailer.
	writeCh := make(chan struct{}, 4)
	writes[path] = writeCh

	t, err := tailer.New(ctx, path, startOffset, writeCh)
	if err != nil {
		delete(writes, path)
		return
	}

	select {
	case w.sessions <- Session{
		Header:        header,
		ID:            id,
		LogPath:       path,
		Lines:         t.Lines(),
		CurrentOffset: t.CurrentOffset,
	}:
	case <-ctx.Done():
	}
}

// emit sends a LogEvent on the logEvents channel without blocking.
// If the buffer is full or the context is done, the event is dropped.
func (w *Watcher) emit(ctx context.Context, ev core.LogEvent) {
	select {
	case w.logEvents <- ev:
	case <-ctx.Done():
	default:
		// buffer full — drop rather than block the watcher goroutine
	}
}

// parseSessionHeaderWithRetry retries parseSessionHeader with exponential
// backoff up to ~3 seconds total. This handles the race where Eve creates the
// log file slightly before it finishes writing the header.
func parseSessionHeaderWithRetry(ctx context.Context, path string) (core.SessionHeader, error) {
	delay := 50 * time.Millisecond
	var lastErr error
	for range 6 { // 50 → 100 → 200 → 400 → 800 → 1600 ms (~3.1 s total)
		h, err := parseSessionHeader(path)
		if err == nil {
			return h, nil
		}
		lastErr = err
		// errNoListener means the header is fully written but has no character —
		// it will never change, so bail immediately without retrying.
		if errors.Is(err, errNoListener) {
			return core.SessionHeader{}, errNoListener
		}
		// errNoHeader means no separator found yet — may be a newly-created Eve
		// file that hasn't been written to yet. Retry with backoff.
		select {
		case <-ctx.Done():
			return core.SessionHeader{}, ctx.Err()
		case <-time.After(delay):
			delay *= 2
		}
	}
	// Preserve errNoHeader so addFile can silently drop non-Eve files.
	if errors.Is(lastErr, errNoHeader) {
		return core.SessionHeader{}, errNoHeader
	}
	return core.SessionHeader{}, fmt.Errorf("could not read header from %s after retries", path)
}

// parseSessionHeader reads the header block (text between the two separator
// lines of dashes) and returns a SessionHeader.
//
// Returns errNoHeader if no separator line is found (empty / non-Eve files).
// Returns errNoListener if the header block has no Listener label (no character).
func parseSessionHeader(path string) (core.SessionHeader, error) {
	f, err := os.Open(path)
	if err != nil {
		return core.SessionHeader{}, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	// Advance to the opening separator line ("---...").
	foundSeparator := false
	for scanner.Scan() {
		if strings.HasPrefix(strings.TrimSpace(scanner.Text()), "---") {
			foundSeparator = true
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return core.SessionHeader{}, fmt.Errorf("scanning %s: %w", path, err)
	}
	if !foundSeparator {
		return core.SessionHeader{}, errNoHeader
	}

	// Collect every line until the closing separator.
	var headerLines []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "---") {
			break
		}
		headerLines = append(headerLines, line)
	}
	if err := scanner.Err(); err != nil {
		return core.SessionHeader{}, fmt.Errorf("scanning %s: %w", path, err)
	}

	if len(headerLines) == 0 {
		return core.SessionHeader{}, fmt.Errorf("empty header block in %s", path)
	}

	// Search the header block for a Listener line to identify language and character.
	var lang core.Language
	var charName string
	for _, line := range headerLines {
		for l, lp := range core.Locales {
			if strings.HasPrefix(line, lp.ListenerLabel) {
				lang = l
				charName = strings.TrimSpace(line[len(lp.ListenerLabel):])
				break
			}
		}
		if charName != "" {
			break
		}
	}
	if charName == "" {
		// Header block has no Listener line — no character is attached to this session.
		return core.SessionHeader{}, errNoListener
	}

	// Search the header block for the session start time.
	lp := core.Locales[lang]
	var startedAt time.Time
	for _, line := range headerLines {
		if strings.HasPrefix(line, lp.SessionTimeLabel) {
			startedAt, err = time.Parse(headerTimeLayout, strings.TrimSpace(line[len(lp.SessionTimeLabel):]))
			if err != nil {
				return core.SessionHeader{}, fmt.Errorf("parse session time in %s: %w", path, err)
			}
			break
		}
	}
	if startedAt.IsZero() {
		return core.SessionHeader{}, fmt.Errorf("no session time line found in %s", path)
	}

	// If a second separator block immediately follows, two characters logged in
	// at the same second and their events are interleaved — skip the file.
	collision := false
	if scanner.Scan() {
		next := strings.TrimSpace(scanner.Text())
		for next == "" && scanner.Scan() {
			next = strings.TrimSpace(scanner.Text())
		}
		collision = strings.HasPrefix(next, "---")
	}

	return core.SessionHeader{
		Character: charName,
		StartedAt: startedAt,
		Language:  lang,
		Collision: collision,
	}, nil
}
