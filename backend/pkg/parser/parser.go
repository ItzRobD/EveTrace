package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"EveTrace/pkg/core"
)

const timeLayout = "2006.01.02 15:04:05"

var (
	reEnvelope = regexp.MustCompile(`^\[ (\d{4}\.\d{2}\.\d{2} \d{2}:\d{2}:\d{2}) \] \((\w+)\) (.+)$`)

	// "Pith Eliminator misses you completely"
	reMissIn = regexp.MustCompile(`^(.+?) misses you completely$`)

	// "Your Hornet II misses Dire Pithi Infiltrator completely - Hornet II"
	reMissOut = regexp.MustCompile(`^Your (.+?) misses (.+?) completely`)

	// bounty payout line — ISK amount is comma-formatted
	reBounty = regexp.MustCompile(`<color=[^>]+>([\d,]+) ISK`)

	// standard yield: "You mined N units of ORE"
	reMineYield = regexp.MustCompile(
		`You mined <font size=\d+><color=[^>]+>(\d+)<color=[^>]+><font size=\d+> units of <color=[^>]+><font size=\d+>(.+)$`,
	)
	// critical bonus: "Critical mining success! ... N units of ORE"
	reMineBonus = regexp.MustCompile(
		`Critical mining success!.*<font size=\d+>(\d+)<color=[^>]+><font size=\d+> units of <color=[^>]+><font size=\d+>(.+)$`,
	)
	// residue lost
	reMineResidue = regexp.MustCompile(
		`Additional <font size=\d+><color=[^>]+>(\d+)<color=[^>]+><font size=\d+> units depleted from asteroid as residue`,
	)

	reNavJump   = regexp.MustCompile(`^Jumping from (.+?) to (.+)$`)
	reNavUndock = regexp.MustCompile(`^Undocking from .+? to (.+?) solar system\.$`)
)

// Parser is a stateful per-session parser.
// One instance must be created per session; do not share across sessions.
type Parser struct {
	lastOutTarget string
	locale        core.LocalePattern
	reCombatDmg   *regexp.Regexp
}

// New creates a Parser for the given client language.
// Use core.LangEnglish for English clients (the most common case).
func New(lang core.Language) *Parser {
	lp := core.Locales[lang]
	if lp.DirIn == "" {
		// Unknown language — fall back to English so the parser stays functional.
		lp = core.Locales[core.LangEnglish]
	}
	re := regexp.MustCompile(buildCombatDmgPattern(lp))
	return &Parser{locale: lp, reCombatDmg: re}
}

// buildCombatDmgPattern constructs the combat damage regex from locale direction keywords.
// Pattern captures: [1] damage, [2] direction keyword, [3] entity, [4] weapon (optional), [5] hit type.
func buildCombatDmgPattern(lp core.LocalePattern) string {
	dirIn := regexp.QuoteMeta(lp.DirIn)
	dirOut := regexp.QuoteMeta(lp.DirOut)
	return fmt.Sprintf(
		`^<color=[^>]+><b>(\d+)</b> <color=[^>]+><font size=\d+>(%s|%s)</font>`+
			` <b><color=[^>]+>(.+?)</b><font size=\d+><color=[^>]+> - (?:(.+?) - )?(.+)$`,
		dirIn, dirOut,
	)
}

// Parse parses a single log line and returns 0 or more events.
// A bounty line produces a KillEvent attributed to the last outgoing target.
func (p *Parser) Parse(sessionID, line string) []core.Event {
	m := reEnvelope.FindStringSubmatch(line)
	if m == nil {
		return nil
	}
	ts, err := time.Parse(timeLayout, m[1])
	if err != nil {
		return nil
	}
	evType, msg := m[2], m[3]

	switch evType {
	case core.LogTypeCombat:
		return p.parseCombat(sessionID, ts, msg)
	case core.LogTypeBounty:
		return p.parseBounty(sessionID, ts, msg)
	case core.LogTypeMining:
		return parseMining(sessionID, ts, msg)
	case core.LogTypeNone:
		return parseNav(sessionID, ts, msg)
	}
	return nil
}

func (p *Parser) parseCombat(sessionID string, ts time.Time, msg string) []core.Event {
	// HTML damage line
	if m := p.reCombatDmg.FindStringSubmatch(msg); m != nil {
		damage, _ := strconv.Atoi(m[1])
		dir := core.DirectionIn
		if m[2] == p.locale.DirOut {
			dir = core.DirectionOut
			p.lastOutTarget = m[3]
		}
		return []core.Event{{
			Type:      core.EventCombat,
			SessionID: sessionID,
			Timestamp: ts,
			Combat: &core.CombatPayload{
				Direction: dir,
				Damage:    damage,
				Entity:    m[3],
				Weapon:    m[4],
				HitType:   m[5],
			},
		}}
	}

	// Outgoing miss — check before incoming miss to avoid false match on "Your ... misses you"
	if m := reMissOut.FindStringSubmatch(msg); m != nil {
		p.lastOutTarget = m[2]
		return []core.Event{{
			Type:      core.EventCombat,
			SessionID: sessionID,
			Timestamp: ts,
			Combat: &core.CombatPayload{
				Direction: core.DirectionOut,
				Entity:    m[2],
				Weapon:    m[1],
				Miss:      true,
			},
		}}
	}

	// Incoming miss
	if m := reMissIn.FindStringSubmatch(msg); m != nil {
		return []core.Event{{
			Type:      core.EventCombat,
			SessionID: sessionID,
			Timestamp: ts,
			Combat: &core.CombatPayload{
				Direction: core.DirectionIn,
				Entity:    m[1],
				Miss:      true,
			},
		}}
	}

	return nil
}

func (p *Parser) parseBounty(sessionID string, ts time.Time, msg string) []core.Event {
	m := reBounty.FindStringSubmatch(msg)
	if m == nil || p.lastOutTarget == "" {
		return nil
	}
	amount, _ := strconv.ParseInt(strings.ReplaceAll(m[1], ",", ""), 10, 64)
	return []core.Event{{
		Type:      core.EventKill,
		SessionID: sessionID,
		Timestamp: ts,
		Kill: &core.KillPayload{
			Entity:    p.lastOutTarget,
			BountyISK: amount,
		},
	}}
}

func parseMining(sessionID string, ts time.Time, msg string) []core.Event {
	// Critical bonus checked first — its message also contains "You mined" text
	if m := reMineBonus.FindStringSubmatch(msg); m != nil {
		amount, _ := strconv.Atoi(m[1])
		return []core.Event{{
			Type:      core.EventMining,
			SessionID: sessionID,
			Timestamp: ts,
			Mining:    &core.MiningPayload{OreType: m[2], Amount: amount, Critical: true},
		}}
	}
	if m := reMineYield.FindStringSubmatch(msg); m != nil {
		amount, _ := strconv.Atoi(m[1])
		return []core.Event{{
			Type:      core.EventMining,
			SessionID: sessionID,
			Timestamp: ts,
			Mining:    &core.MiningPayload{OreType: m[2], Amount: amount},
		}}
	}
	if m := reMineResidue.FindStringSubmatch(msg); m != nil {
		amount, _ := strconv.Atoi(m[1])
		return []core.Event{{
			Type:      core.EventMining,
			SessionID: sessionID,
			Timestamp: ts,
			Mining:    &core.MiningPayload{Amount: amount, Residue: true},
		}}
	}
	return nil
}

func parseNav(sessionID string, ts time.Time, msg string) []core.Event {
	if m := reNavJump.FindStringSubmatch(msg); m != nil {
		return []core.Event{{
			Type:      core.EventNav,
			SessionID: sessionID,
			Timestamp: ts,
			Nav:       &core.NavPayload{From: m[1], To: m[2]},
		}}
	}
	if m := reNavUndock.FindStringSubmatch(msg); m != nil {
		return []core.Event{{
			Type:      core.EventNav,
			SessionID: sessionID,
			Timestamp: ts,
			Nav:       &core.NavPayload{To: m[1]},
		}}
	}
	return nil
}
