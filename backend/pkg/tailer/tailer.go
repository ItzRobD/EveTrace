package tailer

import (
	"bufio"
	"context"
	"io"
	"os"
	"strings"
	"sync/atomic"

	"EveTrace/pkg/core"
)

const headerLines = 5

// countingReader wraps an io.Reader and tracks the total number of bytes read
// from the underlying reader. bufio.Reader reads ahead into its internal
// buffer, so the true "consumed" file position is:
//
//	cr.n - int64(bufReader.Buffered())
type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// Tailer tails a file and emits core.Line values on Lines().
//
// startOffset is the absolute byte position to seek to before tailing:
//   - 0: file is new; tailer skips the 5-line Eve log header then reads all
//     remaining existing content as catch-up (Live=false).
//   - >0: file was partially read in a prior run; tailer seeks directly to
//     that position, treating content from there forward as catch-up until
//     the live edge, then Live=true.
//
// writes is a channel that receives a notification whenever the underlying
// file has been written to. It is provided by the caller (typically the
// directory watcher) so that all tailers share a single inotify instance.
//
// Lines emitted during catch-up have Live=false.
// Once all existing content is drained and the tailer watches for new writes,
// subsequent lines have Live=true.
type Tailer struct {
	lines  chan core.Line
	offset atomic.Int64
}

// New creates and starts a Tailer. Lines() is closed when ctx is cancelled.
func New(ctx context.Context, path string, startOffset int64, writes <-chan struct{}) (*Tailer, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	t := &Tailer{lines: make(chan core.Line, 256)}
	go t.run(ctx, f, startOffset, writes)
	return t, nil
}

// Lines returns the channel of log lines. Closed when the tailer stops.
func (t *Tailer) Lines() <-chan core.Line { return t.lines }

// CurrentOffset returns the byte position of the last fully consumed line.
// Safe to call from any goroutine.
func (t *Tailer) CurrentOffset() int64 { return t.offset.Load() }

func (t *Tailer) run(ctx context.Context, f *os.File, startOffset int64, writes <-chan struct{}) {
	defer f.Close()
	defer close(t.lines)

	cr := &countingReader{r: f}
	r := bufio.NewReader(cr)

	if startOffset > 0 {
		// Resume from a previously stored position.
		if _, err := f.Seek(startOffset, io.SeekStart); err != nil {
			return
		}
		cr.n = startOffset
		r.Reset(cr)
	} else {
		// New file: consume the 5-line Eve log header without emitting it.
		for i := 0; i < headerLines; i++ {
			if _, err := r.ReadString('\n'); err != nil {
				return
			}
		}
	}

	var partial string

	// Drain all existing content as historical data (Live=false).
	t.readLines(ctx, r, cr, &partial, false)

	// Transition to live: wait for write notifications from the shared watcher.
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-writes:
			if !ok {
				return
			}
			t.readLines(ctx, r, cr, &partial, true)
		}
	}
}

// readLines drains all complete lines currently available from r.
// Incomplete trailing data is held in *partial until the next write event.
func (t *Tailer) readLines(ctx context.Context, r *bufio.Reader, cr *countingReader, partial *string, live bool) {
	for {
		chunk, err := r.ReadString('\n')
		*partial += chunk
		if err != nil {
			// No newline yet — keep partial buffered for the next write event.
			return
		}
		line := strings.TrimRight(*partial, "\r\n")
		*partial = ""

		// Consumed position = bytes bufio has pulled from the file minus
		// what it has buffered but not yet returned to us.
		t.offset.Store(cr.n - int64(r.Buffered()))

		if line == "" {
			continue
		}
		select {
		case t.lines <- core.Line{Text: line, Live: live}:
		case <-ctx.Done():
			return
		}
	}
}
