package api

import (
	"context"
	"database/sql"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	genmodel "EveTrace/.gen/model"
	"EveTrace/internal/database/repo"
	"EveTrace/pkg/core"
)

// replayManager tracks and cancels active session replays.
type replayManager struct {
	mu      sync.Mutex
	cancels map[int32]context.CancelFunc
}

var replays = &replayManager{cancels: make(map[int32]context.CancelFunc)}

func (m *replayManager) start(sessionID int32, parent context.Context) context.Context {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cancel, ok := m.cancels[sessionID]; ok {
		cancel()
	}
	ctx, cancel := context.WithCancel(parent)
	m.cancels[sessionID] = cancel
	return ctx
}

func (m *replayManager) done(sessionID int32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.cancels, sessionID)
}

// replaySession loads all stored events for a session and re-emits them through
// the WebSocket hub as live events. Timing between events is preserved but
// compressed by the ?speed= multiplier (default 20, so 20x real-time).
//
//	POST /api/debug/replay/:id?speed=20
func (h *handler) replaySession(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	speed := 20.0
	if s := c.Query("speed"); s != "" {
		if v, err := strconv.ParseFloat(s, 64); err == nil && v > 0 {
			speed = v
		}
	}

	// max_gap_ms: maximum pause between events in replay time (milliseconds).
	// Gaps longer than this are compressed. Default 500ms keeps things moving.
	maxGapMs := 500.0
	if s := c.Query("max_gap_ms"); s != "" {
		if v, err := strconv.ParseFloat(s, 64); err == nil && v >= 0 {
			maxGapMs = v
		}
	}
	maxGap := time.Duration(maxGapMs * float64(time.Millisecond))

	session, err := repo.GetSession(h.db, id)
	if err != nil || session.ID == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	events, err := buildReplayEvents(h.db, id, session.SessionKey)
	if err != nil {
		dbErr(c, err)
		return
	}
	if len(events) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "no events to replay"})
		return
	}

	var durationSecs float64
	if len(events) > 1 {
		durationSecs = events[len(events)-1].originalTs.Sub(events[0].originalTs).Seconds() / speed
	}

	ctx := replays.start(id, h.ctx)
	go func() {
		defer replays.done(id)
		runReplay(ctx, h.hub, events, speed, maxGap)
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message":       "replay started",
		"session_id":    id,
		"events":        len(events),
		"speed":         speed,
		"max_gap_ms":    maxGapMs,
		"duration_secs": durationSecs,
	})
}

// cancelReplay stops a running replay for a session.
//
//	DELETE /api/debug/replay/:id
func (h *handler) cancelReplay(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	replays.mu.Lock()
	cancel, active := replays.cancels[id]
	replays.mu.Unlock()
	if !active {
		c.JSON(http.StatusNotFound, gin.H{"error": "no active replay for this session"})
		return
	}
	cancel()
	c.Status(http.StatusNoContent)
}

// timedEvent pairs an event with its original log timestamp for ordering.
type timedEvent struct {
	originalTs time.Time
	ev         core.Event
}

func runReplay(ctx context.Context, hub *Hub, events []timedEvent, speed float64, maxGap time.Duration) {
	replayStart := time.Now()
	firstTs := events[0].originalTs

	// timeOffset accumulates savings from compressing gaps that exceed maxGap.
	var timeOffset time.Duration
	var prevOriginalTs time.Time

	for _, te := range events {
		if !prevOriginalTs.IsZero() {
			replayGap := time.Duration(float64(te.originalTs.Sub(prevOriginalTs)) / speed)
			if replayGap > maxGap {
				timeOffset += replayGap - maxGap
			}
		}
		prevOriginalTs = te.originalTs

		elapsed := time.Duration(float64(te.originalTs.Sub(firstTs))/speed) - timeOffset
		delay := time.Until(replayStart.Add(elapsed))
		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return
			}
		}
		ev := te.ev
		ev.Live = true
		hub.Send(ev)
	}
}

// buildReplayEvents loads every event type for a session from the DB,
// converts each row to a core.Event, and returns them sorted by timestamp.
func buildReplayEvents(db *sql.DB, sessionID int32, sessionKey string) ([]timedEvent, error) {
	var events []timedEvent

	combat, err := repo.ListCombatEvents(db, sessionID)
	if err != nil {
		return nil, err
	}
	for _, r := range combat {
		events = append(events, timedEvent{
			originalTs: r.Timestamp,
			ev: core.Event{
				Type:      core.EventCombat,
				SessionID: sessionKey,
				Timestamp: r.Timestamp,
				Combat:    combatPayload(r),
			},
		})
	}

	kills, err := repo.ListKillEvents(db, sessionID)
	if err != nil {
		return nil, err
	}
	for _, r := range kills {
		events = append(events, timedEvent{
			originalTs: r.Timestamp,
			ev: core.Event{
				Type:      core.EventKill,
				SessionID: sessionKey,
				Timestamp: r.Timestamp,
				Kill:      &core.KillPayload{Entity: r.Entity, BountyISK: int64(r.BountyIsk)},
			},
		})
	}

	mining, err := repo.ListMiningEvents(db, sessionID)
	if err != nil {
		return nil, err
	}
	for _, r := range mining {
		events = append(events, timedEvent{
			originalTs: r.Timestamp,
			ev: core.Event{
				Type:      core.EventMining,
				SessionID: sessionKey,
				Timestamp: r.Timestamp,
				Mining:    miningPayload(r),
			},
		})
	}

	cap, err := repo.ListCapEvents(db, sessionID)
	if err != nil {
		return nil, err
	}
	for _, r := range cap {
		events = append(events, timedEvent{
			originalTs: r.Timestamp,
			ev: core.Event{
				Type:      core.EventCapStarvation,
				SessionID: sessionKey,
				Timestamp: r.Timestamp,
				CapStarvation: &core.CapStarvationPayload{
					Module:    r.Module,
					Required:  float64(r.CapRequired),
					Available: float64(r.CapAvailable),
				},
			},
		})
	}

	reload, err := repo.ListReloadEvents(db, sessionID)
	if err != nil {
		return nil, err
	}
	for _, r := range reload {
		events = append(events, timedEvent{
			originalTs: r.Timestamp,
			ev: core.Event{
				Type:      core.EventReload,
				SessionID: sessionKey,
				Timestamp: r.Timestamp,
				Reload: &core.ReloadPayload{
					Charge:   r.Charge,
					Launcher: r.Launcher,
					Seconds:  int(r.DurationSeconds),
				},
			},
		})
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].originalTs.Before(events[j].originalTs)
	})
	return events, nil
}

func combatPayload(r genmodel.CombatEvents) *core.CombatPayload {
	p := &core.CombatPayload{
		Direction: core.Direction(r.Direction),
		Damage:    int(r.Damage),
		Entity:    r.Entity,
		Miss:      r.IsMiss != 0,
	}
	if r.Weapon != nil {
		p.Weapon = *r.Weapon
	}
	if r.HitType != nil {
		p.HitType = *r.HitType
	}
	return p
}

func miningPayload(r genmodel.MiningEvents) *core.MiningPayload {
	p := &core.MiningPayload{
		Amount:   int(r.Amount),
		Residue:  r.IsResidue != 0,
		Critical: r.IsCritical != 0,
	}
	if r.OreType != nil {
		p.OreType = *r.OreType
	}
	return p
}
