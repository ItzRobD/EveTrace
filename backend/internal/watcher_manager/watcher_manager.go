package watcher_manager

import (
	"context"
	"database/sql"
	"sync"

	"EveTrace/internal/api"
	"EveTrace/internal/database/repo"
	"EveTrace/internal/logger"
	"EveTrace/internal/metrics"
	"EveTrace/pkg/core"
	"EveTrace/pkg/parser"
	"EveTrace/pkg/watcher"
)

// Deps holds the shared infrastructure needed by the watcher loop.
type Deps struct {
	DB        *sql.DB
	Hub       *api.Hub
	Buf       *repo.EventBuffer
	Register  func(sessionID int32, offsetFn func() int64) // called after session upsert
	FromStart bool
}

// Manager owns a cancellable watcher loop that can be restarted with a
// different log directory while the rest of the server keeps running.
type Manager struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	logDir string
	parent context.Context
	deps   Deps
}

// New returns a Manager. Call Start to begin watching.
func New(parent context.Context, deps Deps) *Manager {
	return &Manager{parent: parent, deps: deps}
}

// LogDir returns the currently watched directory.
func (m *Manager) LogDir() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.logDir
}

// Start watches logDir in a goroutine. If a watcher is already running it is
// stopped first. Returns immediately; the loop runs in the background.
// A blank logDir is a no-op — call again with a real path to begin watching.
func (m *Manager) Start(logDir string) {
	if logDir == "" {
		return
	}
	m.mu.Lock()
	if m.cancel != nil {
		m.cancel()
	}
	ctx, cancel := context.WithCancel(m.parent)
	m.cancel = cancel
	m.logDir = logDir
	m.mu.Unlock()

	go m.run(ctx, logDir)
}

// Restart stops the current watcher and starts a new one for newLogDir.
func (m *Manager) Restart(newLogDir string) {
	m.Start(newLogDir)
}

// Stop cancels the active watcher without starting a replacement.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
}

func (m *Manager) run(ctx context.Context, logDir string) {
	d := m.deps

	var offsetFn watcher.OffsetFn
	if !d.FromStart {
		offsetFn = func(id string) int64 {
			offset, err := repo.GetSessionOffset(d.DB, id)
			if err != nil {
				logger.Error("get session offset", "session", id, "err", err)
				return 0
			}
			return offset
		}
	}

	w, err := watcher.New(ctx, logDir, offsetFn)
	if err != nil {
		logger.Error("watcher init failed", "logdir", logDir, "err", err)
		return
	}

	go drainLogEvents(w.LogEvents())

	for {
		select {
		case <-ctx.Done():
			return
		case sess, ok := <-w.Sessions():
			if !ok {
				return
			}
			logger.Info("session opened", "session", sess.ID, "character", sess.Header.Character)
			metrics.SessionsOpened.Add(1)

			charID, err := repo.UpsertCharacter(d.DB, sess.Header.Character)
			if err != nil {
				logger.Error("upsert character", "character", sess.Header.Character, "err", err)
			}
			sessionID, err := repo.UpsertSession(d.DB, charID, sess.ID, sess.LogPath,
				sess.Header.StartedAt, string(sess.Header.Language))
			if err != nil {
				logger.Error("upsert session", "session", sess.ID, "err", err)
			}

			if sessionID != 0 {
				if d.FromStart {
					if err := repo.ClearSessionEvents(d.DB, sessionID); err != nil {
						logger.Error("clear session events", "session", sess.ID, "err", err)
					}
				}
				d.Register(sessionID, sess.CurrentOffset)
			}

			p := parser.New(sess.Header.Language)
			go func(s watcher.Session, p *parser.Parser, sid int32) {
				for l := range s.Lines {
					for _, ev := range p.Parse(s.ID, l.Text) {
						ev.Live = l.Live
						metrics.EventsProcessed.Add(1)
						if sid != 0 {
							d.Buf.Add(sid, ev)
						}
						d.Hub.Send(ev)
					}
				}
			}(sess, p, sessionID)
		}
	}
}

func drainLogEvents(ch <-chan core.LogEvent) {
	for ev := range ch {
		args := []any{"code", ev.Code, "file", ev.File}
		switch ev.Level {
		case core.LevelDebug:
			logger.Debug(ev.Message, args...)
		case core.LevelInfo:
			logger.Info(ev.Message, args...)
		case core.LevelError:
			logger.Error(ev.Message, args...)
		default:
			logger.Warn(ev.Message, args...)
		}
	}
}
