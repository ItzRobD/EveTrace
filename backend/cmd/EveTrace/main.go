package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"EveTrace/internal/api"
	"EveTrace/internal/appconfig"
	"EveTrace/internal/database"
	"EveTrace/internal/database/repo"
	"EveTrace/internal/logger"
	"EveTrace/internal/watcher_manager"
	"EveTrace/pkg/core"
	"EveTrace/pkg/parser"
	"EveTrace/pkg/watcher"
)

func main() {
	printMode := flag.Bool("print", false, "print parsed events to stdout (debug mode; skips HTTP server)")
	fromStart := flag.Bool("from-start", false, "read existing log content from the beginning (replay mode)")
	logDir := flag.String("logdir", "", "path to Eve Online Gamelogs directory (overrides saved config)")
	logFile := flag.String("logfile", "evetrace.log", "path to the application log file")
	dbFile := flag.String("db", "evetrace.db", "path to the SQLite database file")
	cfgFile := flag.String("config", "", "path to config file (default: OS user config dir)")
	addr := flag.String("addr", ":27182", "address for the HTTP/WebSocket server")
	flushInterval := flag.Duration("flush-interval", 2*time.Minute, "how often to flush buffered events to the database")
	flag.Parse()

	if err := logger.Init(*logFile); err != nil {
		log.Fatalf("logger: %v", err)
	}

	if err := appconfig.Init(*cfgFile); err != nil {
		logger.Error("config load failed", "err", err)
		// Non-fatal: continue with defaults.
	}

	// CLI flag overrides saved config.
	if *logDir == "" {
		*logDir = appconfig.Get().LogDir
	}

	// If no directory is configured or the configured one is invalid, try to auto-detect it.
	if *logDir == "" || !appconfig.IsLogDirValid(*logDir) {
		if detected, ok := appconfig.DetectLogDir(); ok {
			logger.Info("auto-detected log directory", "path", detected)
			*logDir = detected
			// Save the detected path for future launches.
			if err := appconfig.SetLogDir(detected); err != nil {
				logger.Error("failed to save auto-detected log directory", "err", err)
			}
		}
	}

	if err := database.Init(*dbFile); err != nil {
		logger.Error("database init failed", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	hub := api.NewHub()
	go hub.Run(ctx.Done())

	reg := newSessionRegistry()
	buf := repo.NewEventBuffer()

	go buf.Run(ctx, database.DB(), *flushInterval, func(_ int) {
		reg.checkpoint(database.DB())
	})

	if *printMode {
		runPrint(ctx, *logDir, *fromStart, buf, reg)
		finalFlush(buf, reg)
		return
	}

	mgr := watcher_manager.New(ctx, watcher_manager.Deps{
		DB:        database.DB(),
		Hub:       hub,
		Buf:       buf,
		Register:  reg.register,
		FromStart: *fromStart,
	})
	if *logDir != "" && appconfig.IsLogDirValid(*logDir) {
		mgr.Start(*logDir)
	} else {
		logger.Warn("no log directory configured or invalid — auto-detection failed")
		go func() {
			// Wait for the server to start and client to connect before sending the error.
			time.Sleep(3 * time.Second)
			hub.SendDiagnostic(core.LogEvent{
				Level:   core.LevelError,
				Code:    core.CodeNoLogDir,
				Message: "EVE log directory not found. Please configure it in Settings.",
				At:      time.Now(),
			})
		}()
	}

	srv := api.New(database.DB(), hub, *addr, ctx, mgr)
	go func() {
		if err := srv.Run(ctx); err != nil {
			logger.Error("http server", "err", err)
		}
	}()
	logger.Info("server started", "addr", *addr)
	go openBrowser("http://localhost" + *addr)

	// runTray blocks until quit (no-op when built without -tags tray).
	// Everything else is running in goroutines at this point.
	runTray(*addr, cancel)

	// If tray is a no-op, wait for the context to be cancelled (Ctrl-C / SIGTERM).
	<-ctx.Done()

	finalFlush(buf, reg)
}

func finalFlush(buf *repo.EventBuffer, reg *sessionRegistry) {
	logger.Info("flushing event buffer", "buffered", buf.Len(), "totalAdded", buf.TotalAdded())
	if n, err := buf.Flush(database.DB()); err != nil {
		logger.Error("final buffer flush", "err", err)
	} else {
		logger.Info("final buffer flush complete", "written", n)
		reg.checkpoint(database.DB())
	}
}

// runPrint is a debug mode that writes parsed events to stdout without starting
// the HTTP server. It mirrors the watcher_manager session loop but with fmt.Printf output.
func runPrint(ctx context.Context, logDir string, fromStart bool, buf *repo.EventBuffer, reg *sessionRegistry) {
	var offsetFn watcher.OffsetFn
	if !fromStart {
		offsetFn = func(id string) int64 {
			offset, err := repo.GetSessionOffset(database.DB(), id)
			if err != nil {
				logger.Error("get session offset", "session", id, "err", err)
				return 0
			}
			return offset
		}
	}

	w, err := watcher.New(ctx, logDir, offsetFn)
	if err != nil {
		logger.Error("watcher init failed", "err", err)
		return
	}
	go func() {
		for ev := range w.LogEvents() {
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
	}()

	var wg sync.WaitGroup
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		case sess, ok := <-w.Sessions():
			if !ok {
				wg.Wait()
				return
			}
			fmt.Printf("--- session: %s (started %s) ---\n",
				sess.Header.Character,
				sess.Header.StartedAt.Format("2006-01-02 15:04:05"),
			)

			charID, err := repo.UpsertCharacter(database.DB(), sess.Header.Character)
			if err != nil {
				logger.Error("upsert character", "character", sess.Header.Character, "err", err)
			}
			sessionID, err := repo.UpsertSession(database.DB(), charID, sess.ID, sess.LogPath,
				sess.Header.StartedAt, string(sess.Header.Language))
			if err != nil {
				logger.Error("upsert session", "session", sess.ID, "err", err)
			}
			if sessionID != 0 {
				if fromStart {
					if err := repo.ClearSessionEvents(database.DB(), sessionID); err != nil {
						logger.Error("clear session events", "session", sess.ID, "err", err)
					}
				}
				reg.register(sessionID, sess.CurrentOffset)
			}

			p := parser.New(sess.Header.Language)
			wg.Add(1)
			go func(s watcher.Session, p *parser.Parser, sid int32) {
				defer wg.Done()
				for l := range s.Lines {
					for _, ev := range p.Parse(s.ID, l.Text) {
						ev.Live = l.Live
						printEvent(ev)
						if sid != 0 {
							buf.Add(sid, ev)
						}
					}
				}
			}(sess, p, sessionID)
		}
	}
}

func printEvent(ev core.Event) {
	liveTag := "[LIVE]  "
	if !ev.Live {
		liveTag = "[REPLAY]"
	}
	ts := ev.Timestamp.Format("2006-01-02 15:04:05")
	char := ev.SessionID
	for j, c := range ev.SessionID {
		if c == '/' {
			char = ev.SessionID[:j]
			break
		}
	}

	switch ev.Type {
	case core.EventCombat:
		c := ev.Combat
		if c.Miss {
			if c.Direction == core.DirectionIn {
				fmt.Printf("%s [%s] %s  MISS  IN   %s misses you\n", liveTag, char, ts, c.Entity)
			} else {
				fmt.Printf("%s [%s] %s  MISS  OUT  your %s misses %s\n", liveTag, char, ts, c.Weapon, c.Entity)
			}
		} else {
			dir := "IN "
			if c.Direction == core.DirectionOut {
				dir = "OUT"
			}
			weapon := c.Weapon
			if weapon == "" {
				weapon = "?"
			}
			fmt.Printf("%s [%s] %s  DMG   %s  %4d  %-30s  %s - %s\n",
				liveTag, char, ts, dir, c.Damage, c.Entity, weapon, c.HitType)
		}
	case core.EventKill:
		k := ev.Kill
		fmt.Printf("%s [%s] %s  KILL       %-30s  %d ISK\n", liveTag, char, ts, k.Entity, k.BountyISK)
	case core.EventMining:
		m := ev.Mining
		switch {
		case m.Residue:
			fmt.Printf("%s [%s] %s  MINE  RESIDUE  %d units lost\n", liveTag, char, ts, m.Amount)
		case m.Critical:
			fmt.Printf("%s [%s] %s  MINE  CRIT     %d units of %s\n", liveTag, char, ts, m.Amount, m.OreType)
		default:
			fmt.Printf("%s [%s] %s  MINE           %d units of %s\n", liveTag, char, ts, m.Amount, m.OreType)
		}
	case core.EventNav:
		n := ev.Nav
		if n.From == "" {
			fmt.Printf("%s [%s] %s  NAV   undock → %s\n", liveTag, char, ts, n.To)
		} else {
			fmt.Printf("%s [%s] %s  NAV   %s → %s\n", liveTag, char, ts, n.From, n.To)
		}
	case core.EventCapStarvation:
		c := ev.CapStarvation
		fmt.Printf("%s [%s] %s  CAP   %-40s  need %.1f  have %.1f\n",
			liveTag, char, ts, c.Module, c.Required, c.Available)
	case core.EventReload:
		r := ev.Reload
		fmt.Printf("%s [%s] %s  RELOAD  %-30s  → %s (%ds)\n",
			liveTag, char, ts, r.Launcher, r.Charge, r.Seconds)
	case core.EventMiningFull:
		fmt.Printf("%s [%s] %s  MINE  FULL  %s\n", liveTag, char, ts, ev.MiningFull.Module)
	}
}

// sessionRegistry tracks the CurrentOffset function for each active session so
// that offsets can be checkpointed to the DB after every successful flush.
type sessionRegistry struct {
	mu       sync.RWMutex
	sessions map[int32]func() int64
}

func newSessionRegistry() *sessionRegistry {
	return &sessionRegistry{sessions: make(map[int32]func() int64)}
}

func (r *sessionRegistry) register(sid int32, fn func() int64) {
	r.mu.Lock()
	r.sessions[sid] = fn
	r.mu.Unlock()
}

func (r *sessionRegistry) checkpoint(db *sql.DB) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for sid, fn := range r.sessions {
		if err := repo.UpdateSessionOffset(db, sid, fn()); err != nil {
			logger.Error("checkpoint session offset", "sid", sid, "err", err)
		}
	}
}
