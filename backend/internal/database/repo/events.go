package repo

import (
	"database/sql"
	"fmt"
	"time"

	genmodel "EveTrace/.gen/model"
	"EveTrace/.gen/table"
	"EveTrace/pkg/core"
)

// InsertEvent writes a single parsed event immediately. Prefer EventBuffer.Add
// for normal operation; use this only when an immediate, synchronous write is required.
func InsertEvent(db *sql.DB, sessionID int32, ev core.Event) error {
	switch ev.Type {
	case core.EventCombat:
		m := buildCombat(sessionID, ev.Timestamp, ev.Combat)
		_, err := table.CombatEvents.INSERT(table.CombatEvents.MutableColumns).MODEL(m).Exec(db)
		return wrap("insert combat", err)
	case core.EventKill:
		m := buildKill(sessionID, ev.Timestamp, ev.Kill)
		_, err := table.KillEvents.INSERT(table.KillEvents.MutableColumns).MODEL(m).Exec(db)
		return wrap("insert kill", err)
	case core.EventMining:
		m := buildMining(sessionID, ev.Timestamp, ev.Mining)
		_, err := table.MiningEvents.INSERT(table.MiningEvents.MutableColumns).MODEL(m).Exec(db)
		return wrap("insert mining", err)
	case core.EventMiningFull:
		m := buildMiningFull(sessionID, ev.Timestamp, ev.MiningFull)
		_, err := table.MiningFullEvents.INSERT(table.MiningFullEvents.MutableColumns).MODEL(m).Exec(db)
		return wrap("insert mining full", err)
	case core.EventNav:
		m := buildTravel(sessionID, ev.Timestamp, ev.Nav)
		_, err := table.TravelEvents.INSERT(table.TravelEvents.MutableColumns).MODEL(m).Exec(db)
		return wrap("insert travel", err)
	case core.EventCapStarvation:
		m := buildCap(sessionID, ev.Timestamp, ev.CapStarvation)
		_, err := table.CapEvents.INSERT(table.CapEvents.MutableColumns).MODEL(m).Exec(db)
		return wrap("insert cap starvation", err)
	case core.EventReload:
		m := buildReload(sessionID, ev.Timestamp, ev.Reload)
		_, err := table.ReloadEvents.INSERT(table.ReloadEvents.MutableColumns).MODEL(m).Exec(db)
		return wrap("insert reload", err)
	}
	return nil
}

// ── model builders ────────────────────────────────────────────────────────────

func buildCombat(sessionID int32, ts time.Time, p *core.CombatPayload) genmodel.CombatEvents {
	isMiss := int32(0)
	if p.Miss {
		isMiss = 1
	}
	return genmodel.CombatEvents{
		SessionID: sessionID,
		Timestamp: ts,
		Direction: string(p.Direction),
		Damage:    int32(p.Damage),
		Entity:    p.Entity,
		Weapon:    nullStr(p.Weapon),
		HitType:   nullStr(p.HitType),
		IsMiss:    isMiss,
	}
}

func buildKill(sessionID int32, ts time.Time, p *core.KillPayload) genmodel.KillEvents {
	return genmodel.KillEvents{
		SessionID: sessionID,
		Timestamp: ts,
		Entity:    p.Entity,
		// KillEvents.BountyIsk is int32; Eve per-tick bounty payouts fit within int32 range.
		BountyIsk: int32(p.BountyISK),
	}
}

func buildMining(sessionID int32, ts time.Time, p *core.MiningPayload) genmodel.MiningEvents {
	isResidue, isCritical := int32(0), int32(0)
	if p.Residue {
		isResidue = 1
	}
	if p.Critical {
		isCritical = 1
	}
	return genmodel.MiningEvents{
		SessionID:  sessionID,
		Timestamp:  ts,
		OreType:    nullStr(p.OreType),
		Amount:     int32(p.Amount),
		IsResidue:  isResidue,
		IsCritical: isCritical,
	}
}

func buildMiningFull(sessionID int32, ts time.Time, p *core.MiningFullPayload) genmodel.MiningFullEvents {
	return genmodel.MiningFullEvents{SessionID: sessionID, Timestamp: ts, Module: p.Module}
}

func buildTravel(sessionID int32, ts time.Time, p *core.NavPayload) genmodel.TravelEvents {
	return genmodel.TravelEvents{
		SessionID:  sessionID,
		Timestamp:  ts,
		FromSystem: nullStr(p.From),
		ToSystem:   p.To,
	}
}

func buildCap(sessionID int32, ts time.Time, p *core.CapStarvationPayload) genmodel.CapEvents {
	return genmodel.CapEvents{
		SessionID:    sessionID,
		Timestamp:    ts,
		Module:       p.Module,
		CapRequired:  float32(p.Required),
		CapAvailable: float32(p.Available),
	}
}

func buildReload(sessionID int32, ts time.Time, p *core.ReloadPayload) genmodel.ReloadEvents {
	return genmodel.ReloadEvents{
		SessionID:       sessionID,
		Timestamp:       ts,
		Charge:          p.Charge,
		Launcher:        p.Launcher,
		DurationSeconds: int32(p.Seconds),
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// nullStr returns nil for an empty string so it maps to SQL NULL.
func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func wrap(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", op, err)
}
