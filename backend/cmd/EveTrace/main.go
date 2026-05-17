package main

import (
	"EveTrace/internal/database"
	"EveTrace/internal/database/repo"
	"EveTrace/internal/logger"
	"EveTrace/pkg/core"
	"EveTrace/pkg/parser"
	"EveTrace/pkg/watcher"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"
)

func main() {
	printMode := flag.Bool("print", false, "print parsed events to stdout instead of serving the web UI")
	fromStart := flag.Bool("from-start", false, "read existing log content from the beginning (useful with -print for replay)")
	logDir := flag.String("logdir", defaultLogDir(), "path to Eve Online Gamelogs directory")
	logFile := flag.String("logfile", "evetrace.log", "path to the application log file")
	dbFile := flag.String("db", "evetrace.db", "path to the SQLite database file")
	flag.Parse()

	if err := logger.Init(*logFile); err != nil {
		log.Fatalf("logger: %v", err)
	}

	if err := database.Init(*dbFile); err != nil {
		logger.Error("database init failed", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	if *logDir == "" {
		logger.Error("-logdir is required on this platform",
			"example", "-logdir ~/.steam/steam/.../drive_c/Users/.../Documents/EVE/logs/Gamelogs")
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// offsetFn returns the stored read position for a session so the tailer
	// can resume where it left off after a restart.
	var offsetFn watcher.OffsetFn
	if !*fromStart {
		offsetFn = func(id string) int64 {
			offset, err := repo.GetSessionOffset(database.DB(), id)
			if err != nil {
				logger.Error("get session offset", "session", id, "err", err)
				return 0
			}
			return offset
		}
	}

	w, err := watcher.New(ctx, *logDir, offsetFn)
	if err != nil {
		logger.Error("watcher init failed", "err", err)
		os.Exit(1)
	}

	buf := repo.NewEventBuffer()
	go buf.Run(ctx, database.DB(), 2*time.Minute)

	// Drain log events: route to the appropriate slog level and forward to the WebSocket hub.
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

	var finalOffsets map[int32]int64
	if *printMode {
		finalOffsets = runPrint(ctx, w, buf)
	} else {
		fmt.Println("serve mode not yet implemented — use -print to test the parser")
		<-ctx.Done()
	}

	// Flush events first, then save offsets only on success so offsets never
	// advance past the last successfully written event.
	logger.Info("flushing event buffer", "buffered", buf.Len(), "totalAdded", buf.TotalAdded())
	if n, err := buf.Flush(database.DB()); err != nil {
		logger.Error("final buffer flush", "err", err)
	} else {
		logger.Info("final buffer flush complete", "written", n)
		for sid, offset := range finalOffsets {
			if err := repo.UpdateSessionOffset(database.DB(), sid, offset); err != nil {
				logger.Error("update session offset", "sid", sid, "err", err)
			}
		}
	}
}

// runPrint processes all sessions, printing events to stdout and buffering them
// for DB insertion. Returns a map of sessionID → final byte offset for each
// session, to be saved after the event buffer is flushed successfully.
func runPrint(ctx context.Context, w *watcher.Watcher, buf *repo.EventBuffer) map[int32]int64 {
	var (
		wg        sync.WaitGroup
		offsetsMu sync.Mutex
		offsets   = make(map[int32]int64)
	)

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return offsets
		case sess, ok := <-w.Sessions():
			if !ok {
				wg.Wait()
				return offsets
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
			logger.Debug("session registered", "session", sess.ID, "charID", charID, "sessionID", sessionID)

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
				// Record the final read position; saved to DB after flush succeeds.
				if sid != 0 {
					offsetsMu.Lock()
					offsets[sid] = s.CurrentOffset()
					offsetsMu.Unlock()
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
	// Print just the character name portion (before the slash)
	if i := len(ev.SessionID); i > 0 {
		for j, c := range ev.SessionID {
			if c == '/' {
				char = ev.SessionID[:j]
				break
			}
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

func defaultLogDir() string {
	if runtime.GOOS == "windows" {
		return os.Getenv("USERPROFILE") + `\Documents\EVE\logs\Gamelogs`
	}
	return ""
}
