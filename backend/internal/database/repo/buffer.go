package repo

import (
	"context"
	"database/sql"
	"log"
	"sync"
	"time"

	genmodel "EveTrace/.gen/model"
	"EveTrace/.gen/table"
	"EveTrace/pkg/core"
)

// EventBuffer accumulates parsed events in memory and flushes them to the
// database in batches. This amortises SQLite write overhead across many events,
// which is critical during historical replay where thousands of rows arrive in
// rapid succession.
//
// Multiple goroutines may call Add concurrently. Flush and Run must only be
// called from a single goroutine (typically main or a dedicated flush goroutine).
type EventBuffer struct {
	mu         sync.Mutex
	combat     []genmodel.CombatEvents
	kill       []genmodel.KillEvents
	mining     []genmodel.MiningEvents
	miningFull []genmodel.MiningFullEvents
	travel     []genmodel.TravelEvents
	cap        []genmodel.CapEvents
	reload     []genmodel.ReloadEvents
	totalAdded int64 // diagnostic: cumulative Add calls across all flushes
}

// NewEventBuffer returns an empty EventBuffer ready for use.
func NewEventBuffer() *EventBuffer { return &EventBuffer{} }

// Add appends a parsed event to the appropriate in-memory slice.
// Safe to call from multiple goroutines simultaneously.
func (b *EventBuffer) Add(sessionID int32, ev core.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch ev.Type {
	case core.EventCombat:
		b.combat = append(b.combat, buildCombat(sessionID, ev.Timestamp, ev.Combat))
	case core.EventKill:
		b.kill = append(b.kill, buildKill(sessionID, ev.Timestamp, ev.Kill))
	case core.EventMining:
		b.mining = append(b.mining, buildMining(sessionID, ev.Timestamp, ev.Mining))
	case core.EventMiningFull:
		b.miningFull = append(b.miningFull, buildMiningFull(sessionID, ev.Timestamp, ev.MiningFull))
	case core.EventNav:
		b.travel = append(b.travel, buildTravel(sessionID, ev.Timestamp, ev.Nav))
	case core.EventCapStarvation:
		b.cap = append(b.cap, buildCap(sessionID, ev.Timestamp, ev.CapStarvation))
	case core.EventReload:
		b.reload = append(b.reload, buildReload(sessionID, ev.Timestamp, ev.Reload))
	default:
		return
	}
	b.totalAdded++
}

// TotalAdded returns the cumulative number of events added since the buffer was created.
func (b *EventBuffer) TotalAdded() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.totalAdded
}

// Len returns the total number of buffered events across all types.
func (b *EventBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.combat) + len(b.kill) + len(b.mining) + len(b.miningFull) +
		len(b.travel) + len(b.cap) + len(b.reload)
}

// Flush writes all buffered events to the database in a single transaction and
// clears the buffer. If the buffer is empty, Flush is a no-op.
// Returns the number of rows written and any error.
func (b *EventBuffer) Flush(db *sql.DB) (int, error) {
	// Swap out the slices under lock so Add can continue while we write.
	// Set to nil (not [:0]) so subsequent appends get a fresh backing array
	// and cannot alias the slices we're about to write.
	b.mu.Lock()
	combat, kill, mining, miningFull, travel, cap, reload :=
		b.combat, b.kill, b.mining, b.miningFull, b.travel, b.cap, b.reload
	b.combat = nil
	b.kill = nil
	b.mining = nil
	b.miningFull = nil
	b.travel = nil
	b.cap = nil
	b.reload = nil
	b.mu.Unlock()

	total := len(combat) + len(kill) + len(mining) + len(miningFull) +
		len(travel) + len(cap) + len(reload)
	if total == 0 {
		return 0, nil
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, wrap("flush begin tx", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if err := batchCombat(tx, combat); err != nil {
		return 0, err
	}
	if err := batchKill(tx, kill); err != nil {
		return 0, err
	}
	if err := batchMining(tx, mining); err != nil {
		return 0, err
	}
	if err := batchMiningFull(tx, miningFull); err != nil {
		return 0, err
	}
	if err := batchTravel(tx, travel); err != nil {
		return 0, err
	}
	if err := batchCap(tx, cap); err != nil {
		return 0, err
	}
	if err := batchReload(tx, reload); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, wrap("flush commit", err)
	}
	return total, nil
}

// Run starts a ticker that flushes the buffer every interval until ctx is
// cancelled. onFlush is called after each successful periodic flush with the
// number of rows written; use it to checkpoint session offsets. Pass nil if no
// post-flush action is needed. The caller is responsible for a final flush
// after Run returns.
func (b *EventBuffer) Run(ctx context.Context, db *sql.DB, interval time.Duration, onFlush func(n int)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			n, err := b.Flush(db)
			if err != nil {
				log.Printf("event buffer flush: %v", err)
				continue
			}
			if n > 0 {
				log.Printf("event buffer: flushed %d events", n)
				if onFlush != nil {
					onFlush(n)
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

// ── batch insert helpers ──────────────────────────────────────────────────────

// sqliteChunkSize is the max rows per INSERT statement. SQLite limits bound
// variables to 999 by default; combat_events has 8 columns so 100×8=800 is safe
// for every table in this schema.
const sqliteChunkSize = 100

// insertChunked calls insert for each chunk of rows to stay within SQLite's
// per-statement variable limit.
func insertChunked[T any](tx *sql.Tx, rows []T, insert func(*sql.Tx, []T) error) error {
	for len(rows) > 0 {
		n := sqliteChunkSize
		if n > len(rows) {
			n = len(rows)
		}
		if err := insert(tx, rows[:n]); err != nil {
			return err
		}
		rows = rows[n:]
	}
	return nil
}

func batchCombat(tx *sql.Tx, rows []genmodel.CombatEvents) error {
	return insertChunked(tx, rows, func(tx *sql.Tx, chunk []genmodel.CombatEvents) error {
		_, err := table.CombatEvents.INSERT(table.CombatEvents.MutableColumns).
			MODELS(chunk).Exec(tx)
		return wrap("batch combat", err)
	})
}

func batchKill(tx *sql.Tx, rows []genmodel.KillEvents) error {
	return insertChunked(tx, rows, func(tx *sql.Tx, chunk []genmodel.KillEvents) error {
		_, err := table.KillEvents.INSERT(table.KillEvents.MutableColumns).
			MODELS(chunk).Exec(tx)
		return wrap("batch kill", err)
	})
}

func batchMining(tx *sql.Tx, rows []genmodel.MiningEvents) error {
	return insertChunked(tx, rows, func(tx *sql.Tx, chunk []genmodel.MiningEvents) error {
		_, err := table.MiningEvents.INSERT(table.MiningEvents.MutableColumns).
			MODELS(chunk).Exec(tx)
		return wrap("batch mining", err)
	})
}

func batchMiningFull(tx *sql.Tx, rows []genmodel.MiningFullEvents) error {
	return insertChunked(tx, rows, func(tx *sql.Tx, chunk []genmodel.MiningFullEvents) error {
		_, err := table.MiningFullEvents.INSERT(table.MiningFullEvents.MutableColumns).
			MODELS(chunk).Exec(tx)
		return wrap("batch mining full", err)
	})
}

func batchTravel(tx *sql.Tx, rows []genmodel.TravelEvents) error {
	return insertChunked(tx, rows, func(tx *sql.Tx, chunk []genmodel.TravelEvents) error {
		_, err := table.TravelEvents.INSERT(table.TravelEvents.MutableColumns).
			MODELS(chunk).Exec(tx)
		return wrap("batch travel", err)
	})
}

func batchCap(tx *sql.Tx, rows []genmodel.CapEvents) error {
	return insertChunked(tx, rows, func(tx *sql.Tx, chunk []genmodel.CapEvents) error {
		_, err := table.CapEvents.INSERT(table.CapEvents.MutableColumns).
			MODELS(chunk).Exec(tx)
		return wrap("batch cap", err)
	})
}

func batchReload(tx *sql.Tx, rows []genmodel.ReloadEvents) error {
	return insertChunked(tx, rows, func(tx *sql.Tx, chunk []genmodel.ReloadEvents) error {
		_, err := table.ReloadEvents.INSERT(table.ReloadEvents.MutableColumns).
			MODELS(chunk).Exec(tx)
		return wrap("batch reload", err)
	})
}
