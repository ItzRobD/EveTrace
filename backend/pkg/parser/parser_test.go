package parser

import (
	"EveTrace/pkg/core"
	"testing"
	"time"
)

const sid = "Rheitland/20260506-003642"

func ts(s string) time.Time {
	t, _ := time.Parse(timeLayout, s)
	return t
}

func TestIncomingDamageWithWeapon(t *testing.T) {
	p := New(core.LangEnglish)
	line := `[ 2026.05.06 01:42:18 ] (combat) <color=0xffcc0000><b>60</b> <color=0x77ffffff><font size=10>from</font> <b><color=0xffffffff>Pith Eliminator</b><font size=10><color=0x77ffffff> - Scourge Cruise Missile - Hits`
	evs := p.Parse(sid, line)
	if len(evs) != 1 {
		t.Fatalf("want 1 event, got %d", len(evs))
	}
	c := evs[0].Combat
	if c.Direction != core.DirectionIn {
		t.Errorf("direction: want in, got %s", c.Direction)
	}
	if c.Damage != 60 {
		t.Errorf("damage: want 60, got %d", c.Damage)
	}
	if c.Entity != "Pith Eliminator" {
		t.Errorf("entity: want %q, got %q", "Pith Eliminator", c.Entity)
	}
	if c.Weapon != "Scourge Cruise Missile" {
		t.Errorf("weapon: want %q, got %q", "Scourge Cruise Missile", c.Weapon)
	}
	if c.HitType != "Hits" {
		t.Errorf("hitType: want %q, got %q", "Hits", c.HitType)
	}
	if c.Miss {
		t.Error("miss should be false")
	}
}

func TestIncomingDamageNoWeapon(t *testing.T) {
	p := New(core.LangEnglish)
	// "- Hits" with no weapon
	line := `[ 2026.05.06 01:42:17 ] (combat) <color=0xffcc0000><b>0</b> <color=0x77ffffff><font size=10>from</font> <b><color=0xffffffff>Pith Dismantler</b><font size=10><color=0x77ffffff> - Hits`
	evs := p.Parse(sid, line)
	if len(evs) != 1 {
		t.Fatalf("want 1 event, got %d", len(evs))
	}
	c := evs[0].Combat
	if c.Weapon != "" {
		t.Errorf("weapon: want empty, got %q", c.Weapon)
	}
	if c.HitType != "Hits" {
		t.Errorf("hitType: want %q, got %q", "Hits", c.HitType)
	}
}

func TestIncomingDamageHitTypeVariants(t *testing.T) {
	p := New(core.LangEnglish)
	cases := []struct {
		suffix  string
		hitType string
	}{
		{`Smashes`, "Smashes"},
		{`Penetrates`, "Penetrates"},
		{`Grazes`, "Grazes"},
		{`Glances Off`, "Glances Off"},
	}
	for _, tc := range cases {
		line := `[ 2026.05.06 01:42:20 ] (combat) <color=0xffcc0000><b>45</b> <color=0x77ffffff><font size=10>from</font> <b><color=0xffffffff>Pith Obliterator</b><font size=10><color=0x77ffffff> - ` + tc.suffix
		evs := p.Parse(sid, line)
		if len(evs) != 1 {
			t.Fatalf("suffix %q: want 1 event, got %d", tc.suffix, len(evs))
		}
		if evs[0].Combat.HitType != tc.hitType {
			t.Errorf("suffix %q: hitType want %q, got %q", tc.suffix, tc.hitType, evs[0].Combat.HitType)
		}
	}
}

func TestOutgoingDamage(t *testing.T) {
	p := New(core.LangEnglish)
	line := `[ 2026.05.06 01:45:08 ] (combat) <color=0xff00ffff><b>775</b> <color=0x77ffffff><font size=10>to</font> <b><color=0xffffffff>Pithior Guerilla</b><font size=10><color=0x77ffffff> - Scourge Heavy Missile - Hits`
	evs := p.Parse(sid, line)
	if len(evs) != 1 {
		t.Fatalf("want 1 event, got %d", len(evs))
	}
	c := evs[0].Combat
	if c.Direction != core.DirectionOut {
		t.Errorf("direction: want out, got %s", c.Direction)
	}
	if c.Damage != 775 {
		t.Errorf("damage: want 775, got %d", c.Damage)
	}
	if c.Entity != "Pithior Guerilla" {
		t.Errorf("entity: %q", c.Entity)
	}
	if p.lastOutTarget != "Pithior Guerilla" {
		t.Errorf("lastOutTarget not updated: %q", p.lastOutTarget)
	}
}

func TestOutgoingDroneDamage(t *testing.T) {
	p := New(core.LangEnglish)
	line := `[ 2026.05.06 01:46:03 ] (combat) <color=0xff00ffff><b>39</b> <color=0x77ffffff><font size=10>to</font> <b><color=0xffffffff>Dire Pithi Infiltrator</b><font size=10><color=0x77ffffff> - Hornet II - Glances Off`
	evs := p.Parse(sid, line)
	if len(evs) != 1 {
		t.Fatalf("want 1 event, got %d", len(evs))
	}
	c := evs[0].Combat
	if c.Weapon != "Hornet II" {
		t.Errorf("weapon: want %q, got %q", "Hornet II", c.Weapon)
	}
	if c.Direction != core.DirectionOut {
		t.Errorf("direction: want out, got %s", c.Direction)
	}
}

func TestIncomingMiss(t *testing.T) {
	p := New(core.LangEnglish)
	line := `[ 2026.05.06 01:42:17 ] (combat) Pith Eliminator misses you completely`
	evs := p.Parse(sid, line)
	if len(evs) != 1 {
		t.Fatalf("want 1 event, got %d", len(evs))
	}
	c := evs[0].Combat
	if !c.Miss {
		t.Error("miss should be true")
	}
	if c.Direction != core.DirectionIn {
		t.Errorf("direction: want in, got %s", c.Direction)
	}
	if c.Entity != "Pith Eliminator" {
		t.Errorf("entity: %q", c.Entity)
	}
}

func TestOutgoingMissTurret(t *testing.T) {
	p := New(core.LangEnglish)
	line := `[ 2026.05.06 01:46:03 ] (combat) Your Hornet II misses Dire Pithi Infiltrator completely - Hornet II`
	evs := p.Parse(sid, line)
	if len(evs) != 1 {
		t.Fatalf("want 1 event, got %d", len(evs))
	}
	c := evs[0].Combat
	if !c.Miss {
		t.Error("miss should be true")
	}
	if c.Direction != core.DirectionOut {
		t.Errorf("direction: want out, got %s", c.Direction)
	}
	if c.Entity != "Dire Pithi Infiltrator" {
		t.Errorf("entity: %q", c.Entity)
	}
	if c.Weapon != "Hornet II" {
		t.Errorf("weapon: %q", c.Weapon)
	}
	if p.lastOutTarget != "Dire Pithi Infiltrator" {
		t.Errorf("lastOutTarget not updated: %q", p.lastOutTarget)
	}
}

func TestKillAttribution(t *testing.T) {
	p := New(core.LangEnglish)
	// Outgoing hit sets lastOutTarget
	hitLine := `[ 2026.05.06 01:45:08 ] (combat) <color=0xff00ffff><b>775</b> <color=0x77ffffff><font size=10>to</font> <b><color=0xffffffff>Pithior Guerilla</b><font size=10><color=0x77ffffff> - Scourge Heavy Missile - Hits`
	p.Parse(sid, hitLine)

	// Bounty immediately after → kill attributed to Pithior Guerilla
	bountyLine := `[ 2026.05.06 01:45:08 ] (bounty) <font size=12><b><color=0xff00aa00>13,500 ISK</b><color=0x77ffffff> added to next bounty payout`
	evs := p.Parse(sid, bountyLine)
	if len(evs) != 1 {
		t.Fatalf("want 1 kill event, got %d", len(evs))
	}
	if evs[0].Type != core.EventKill {
		t.Errorf("type: want kill, got %s", evs[0].Type)
	}
	k := evs[0].Kill
	if k.Entity != "Pithior Guerilla" {
		t.Errorf("entity: want %q, got %q", "Pithior Guerilla", k.Entity)
	}
	if k.BountyISK != 13500 {
		t.Errorf("bountyISK: want 13500, got %d", k.BountyISK)
	}
}

func TestBountyWithoutPriorTarget(t *testing.T) {
	p := New(core.LangEnglish)
	line := `[ 2026.05.06 01:45:08 ] (bounty) <font size=12><b><color=0xff00aa00>13,500 ISK</b><color=0x77ffffff> added to next bounty payout`
	evs := p.Parse(sid, line)
	if len(evs) != 0 {
		t.Errorf("want 0 events when no prior target, got %d", len(evs))
	}
}

func TestMiningStandardYield(t *testing.T) {
	p := New(core.LangEnglish)
	line := `[ 2026.05.16 05:47:00 ] (mining) <color=0x77ffffff>You mined <font size=12><color=#ff8dc169>144<color=0x77ffffff><font size=10> units of <color=0xffffffff><font size=12>Pyroxeres II-Grade`
	evs := p.Parse(sid, line)
	if len(evs) != 1 {
		t.Fatalf("want 1 event, got %d", len(evs))
	}
	m := evs[0].Mining
	if m.Amount != 144 {
		t.Errorf("amount: want 144, got %d", m.Amount)
	}
	if m.OreType != "Pyroxeres II-Grade" {
		t.Errorf("oreType: %q", m.OreType)
	}
	if m.Residue || m.Critical {
		t.Error("residue/critical should be false")
	}
}

func TestMiningCriticalBonus(t *testing.T) {
	p := New(core.LangEnglish)
	line := `[ 2026.05.16 05:47:15 ] (mining) <color=#fff0ff45>Critical mining success!<color=0x77ffffff><font size=10> You mined an additional <color=#fff0ff45><font size=12>348<color=0x77ffffff><font size=10> units of <color=0xffffffff><font size=12>Pyroxeres II-Grade`
	evs := p.Parse(sid, line)
	if len(evs) != 1 {
		t.Fatalf("want 1 event, got %d", len(evs))
	}
	m := evs[0].Mining
	if m.Amount != 348 {
		t.Errorf("amount: want 348, got %d", m.Amount)
	}
	if !m.Critical {
		t.Error("critical should be true")
	}
	if m.Residue {
		t.Error("residue should be false")
	}
}

func TestMiningResidue(t *testing.T) {
	p := New(core.LangEnglish)
	line := `[ 2026.05.16 05:48:15 ] (mining) <color=0x77ffffff>Additional <font size=12><color=#ffff454b>145<color=0x77ffffff><font size=10> units depleted from asteroid as residue`
	evs := p.Parse(sid, line)
	if len(evs) != 1 {
		t.Fatalf("want 1 event, got %d", len(evs))
	}
	m := evs[0].Mining
	if m.Amount != 145 {
		t.Errorf("amount: want 145, got %d", m.Amount)
	}
	if !m.Residue {
		t.Error("residue should be true")
	}
	if m.OreType != "" {
		t.Errorf("oreType should be empty for residue, got %q", m.OreType)
	}
}

func TestNavJump(t *testing.T) {
	p := New(core.LangEnglish)
	line := `[ 2026.05.06 01:31:49 ] (None) Jumping from Jita to Sobaseki`
	evs := p.Parse(sid, line)
	if len(evs) != 1 {
		t.Fatalf("want 1 event, got %d", len(evs))
	}
	n := evs[0].Nav
	if n.From != "Jita" {
		t.Errorf("from: %q", n.From)
	}
	if n.To != "Sobaseki" {
		t.Errorf("to: %q", n.To)
	}
}

func TestNavUndock(t *testing.T) {
	p := New(core.LangEnglish)
	line := `[ 2026.05.06 01:30:47 ] (None) Undocking from Jita IV - Moon 4 - Caldari Navy Assembly Plant to Jita solar system.`
	evs := p.Parse(sid, line)
	if len(evs) != 1 {
		t.Fatalf("want 1 event, got %d", len(evs))
	}
	n := evs[0].Nav
	if n.From != "" {
		t.Errorf("from should be empty on undock, got %q", n.From)
	}
	if n.To != "Jita" {
		t.Errorf("to: want %q, got %q", "Jita", n.To)
	}
}

func TestDroppedEventTypes(t *testing.T) {
	p := New(core.LangEnglish)
	dropped := []string{
		`[ 2026.05.06 01:44:27 ] (notify) The target <b>Pithior Guerilla</b> is too far away.`,
		`[ 2026.05.06 01:04:19 ] (question) Do you authorize payment?`,
		`[ 2026.05.16 05:02:56 ] (warning) Warning! This star system has been secured.`,
		`[ 2026.05.06 01:04:19 ] (hint) Some hint`,
		`[ 2026.05.06 01:04:19 ] (info) Some info`,
	}
	for _, line := range dropped {
		if evs := p.Parse(sid, line); len(evs) != 0 {
			t.Errorf("line %q: want 0 events, got %d", line, len(evs))
		}
	}
}

func TestTimestamp(t *testing.T) {
	p := New(core.LangEnglish)
	line := `[ 2026.05.06 01:42:18 ] (combat) <color=0xffcc0000><b>60</b> <color=0x77ffffff><font size=10>from</font> <b><color=0xffffffff>Pith Eliminator</b><font size=10><color=0x77ffffff> - Scourge Cruise Missile - Hits`
	evs := p.Parse(sid, line)
	want := ts("2026.05.06 01:42:18")
	if !evs[0].Timestamp.Equal(want) {
		t.Errorf("timestamp: want %v, got %v", want, evs[0].Timestamp)
	}
}
