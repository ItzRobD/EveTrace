package main

import (
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
	"syscall"
)

func main() {
	printMode := flag.Bool("print", false, "print parsed events to stdout instead of serving the web UI")
	fromStart := flag.Bool("from-start", false, "read existing log content from the beginning (useful with -print for replay)")
	logDir := flag.String("logdir", defaultLogDir(), "path to Eve Online Gamelogs directory")
	logFile := flag.String("logfile", "evetrace.log", "path to the application log file")
	flag.Parse()

	if err := logger.Init(*logFile); err != nil {
		log.Fatalf("logger: %v", err)
	}

	if *logDir == "" {
		logger.Error("-logdir is required on this platform",
			"example", "-logdir ~/.steam/steam/.../drive_c/Users/.../Documents/EVE/logs/Gamelogs")
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// offsetFn returns the stored read position for a session so the tailer
	// can resume where it left off after a restart. nil = always start from
	// the beginning (used when -from-start is set, or until the DB is wired up).
	var offsetFn watcher.OffsetFn
	if !*fromStart {
		// TODO: replace with a real DB lookup once the schema is in place.
		// offsetFn = func(id string) int64 { return database.GetOffset(id) }
		offsetFn = nil // falls back to reading from beginning for now
	}

	w, err := watcher.New(ctx, *logDir, offsetFn)
	if err != nil {
		logger.Error("watcher init failed", "err", err)
		os.Exit(1)
	}

	// Drain log events: route to the appropriate slog level and (later) forward to the WebSocket hub.
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

	if *printMode {
		runPrint(ctx, w)
	} else {
		fmt.Println("serve mode not yet implemented — use -print to test the parser")
		<-ctx.Done()
	}
}

func runPrint(ctx context.Context, w *watcher.Watcher) {
	for {
		select {
		case <-ctx.Done():
			return
		case sess, ok := <-w.Sessions():
			if !ok {
				return
			}
			fmt.Printf("--- session: %s (started %s) ---\n",
				sess.Header.Character,
				sess.Header.StartedAt.Format("2006-01-02 15:04:05"),
			)
			p := parser.New(sess.Header.Language)
			go func(s watcher.Session, p *parser.Parser) {
				for l := range s.Lines {
					for _, ev := range p.Parse(s.ID, l.Text) {
						ev.Live = l.Live
						printEvent(ev)
					}
				}
			}(sess, p)
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
