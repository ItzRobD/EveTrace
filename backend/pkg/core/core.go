package core

import "time"

type Direction string

const (
	DirectionIn  Direction = "in"
	DirectionOut Direction = "out"
)

type EventType string

const (
	EventCombat EventType = "combat"
	EventKill   EventType = "kill"
	EventMining EventType = "mining"
	EventNav    EventType = "nav"
)

type SessionHeader struct {
	Character string
	StartedAt time.Time
	Language  Language
	// Collision is true when two characters logged in at the same second and
	// share a single log file (a second header block appears at line 6).
	Collision bool
}

// SessionID returns a stable string identifier for this session.
func (h SessionHeader) SessionID() string {
	return h.Character + "/" + h.StartedAt.Format("20060102-150405")
}

// Line is a raw log line emitted by the tailer.
// Live is false while the tailer is replaying previously unread file content
// (catch-up after a tool restart), and true once it has reached the live edge
// and is watching for new writes from the game client.
type Line struct {
	Text string
	Live bool
}

type Event struct {
	Type      EventType
	SessionID string
	Timestamp time.Time
	// Live mirrors the Line.Live flag. False means this event was parsed from
	// catch-up replay; true means it arrived from the live game client.
	// Consumers should not broadcast catch-up events over WebSocket.
	Live   bool
	Combat *CombatPayload
	Kill   *KillPayload
	Mining *MiningPayload
	Nav    *NavPayload
}

type CombatPayload struct {
	Direction Direction
	Damage    int
	Entity    string
	Weapon    string // empty when no weapon is logged (e.g. some NPC hits, misses)
	HitType   string // empty on misses
	Miss      bool
}

type KillPayload struct {
	Entity    string
	BountyISK int64
}

type MiningPayload struct {
	OreType  string // empty for residue events
	Amount   int
	Residue  bool // ore lost to inefficiency, not added to yield
	Critical bool // bonus yield from critical mining success
}

type NavPayload struct {
	From string // empty on undock (origin is a station, not a system)
	To   string
}
