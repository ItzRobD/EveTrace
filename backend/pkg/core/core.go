package core

import "time"

type Direction string

const (
	DirectionIn  Direction = "in"
	DirectionOut Direction = "out"
)

type EventType string

const (
	EventCombat        EventType = "combat"
	EventKill          EventType = "kill"
	EventMining        EventType = "mining"
	EventNav           EventType = "nav"
	EventCapStarvation EventType = "cap_starvation"
	EventReload        EventType = "reload"
	EventMiningFull    EventType = "mining_full"
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
	// Offset is the byte position in the file immediately after this line — the
	// position a tailer would resume from. Used to checkpoint ingest progress
	// atomically with the events parsed from this line.
	Offset int64
}

type Event struct {
	Type      EventType
	SessionID string
	Timestamp time.Time
	// Live mirrors the Line.Live flag. False means this event was parsed from
	// catch-up replay; true means it arrived from the live game client.
	// Consumers should not broadcast catch-up events over WebSocket.
	Live          bool
	Combat        *CombatPayload
	Kill          *KillPayload
	Mining        *MiningPayload
	Nav           *NavPayload
	CapStarvation *CapStarvationPayload
	Reload        *ReloadPayload
	MiningFull    *MiningFullPayload
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

// CapStarvationPayload records a module that failed to activate because the
// capacitor did not have enough charge. Useful for identifying cap-starved
// ships and correlating with incoming damage spikes.
type CapStarvationPayload struct {
	Module    string  // e.g. "Large Shield Booster II"
	Required  float64 // capacitor units the module needed
	Available float64 // capacitor units actually present
}

// ReloadPayload records a weapon reload event. The reload window is dead time
// for DPS purposes; pairing this timestamp with the next outgoing combat event
// gives the actual reload duration.
type ReloadPayload struct {
	Charge   string // ammo type being loaded, e.g. "Heavy Missile"
	Launcher string // module receiving the charge, e.g. "Missile Launcher Heavy"
	Seconds  int    // stated reload duration
}

// MiningFullPayload records the moment a miner module stopped because the
// ship's cargo hold reached capacity. Acts as a natural mining session boundary.
type MiningFullPayload struct {
	Module string // e.g. "Miner II", "Strip Miner I"
}
