package repo

import (
	"database/sql"
	"errors"

	"github.com/go-jet/jet/v2/qrm"
	. "github.com/go-jet/jet/v2/sqlite"

	genmodel "EveTrace/.gen/model"
	"EveTrace/.gen/table"
)

// ── Characters ────────────────────────────────────────────────────────────────

func ListCharacters(db *sql.DB) ([]genmodel.Characters, error) {
	var dest []genmodel.Characters
	err := SELECT(table.Characters.AllColumns).
		FROM(table.Characters).
		ORDER_BY(table.Characters.Name.ASC()).
		Query(db, &dest)
	return dest, wrap("list characters", err)
}

func GetCharacter(db *sql.DB, id int32) (genmodel.Characters, error) {
	var dest genmodel.Characters
	err := SELECT(table.Characters.AllColumns).
		FROM(table.Characters).
		WHERE(table.Characters.ID.EQ(Int32(id))).
		Query(db, &dest)
	if errors.Is(err, qrm.ErrNoRows) {
		return dest, nil
	}
	return dest, wrap("get character", err)
}

// ── Sessions ──────────────────────────────────────────────────────────────────

// ListSessions returns all sessions. If characterID > 0 it filters by character.
func ListSessions(db *sql.DB, characterID int32) ([]genmodel.Sessions, error) {
	var dest []genmodel.Sessions
	stmt := SELECT(table.Sessions.AllColumns).
		FROM(table.Sessions).
		ORDER_BY(table.Sessions.StartedAt.DESC())
	if characterID > 0 {
		stmt = stmt.WHERE(table.Sessions.CharacterID.EQ(Int32(characterID)))
	}
	err := stmt.Query(db, &dest)
	return dest, wrap("list sessions", err)
}

func GetSession(db *sql.DB, id int32) (genmodel.Sessions, error) {
	var dest genmodel.Sessions
	err := SELECT(table.Sessions.AllColumns).
		FROM(table.Sessions).
		WHERE(table.Sessions.ID.EQ(Int32(id))).
		Query(db, &dest)
	if errors.Is(err, qrm.ErrNoRows) {
		return dest, nil
	}
	return dest, wrap("get session", err)
}

// ── Events ────────────────────────────────────────────────────────────────────

func ListCombatEvents(db *sql.DB, sessionID int32) ([]genmodel.CombatEvents, error) {
	var dest []genmodel.CombatEvents
	err := SELECT(table.CombatEvents.AllColumns).
		FROM(table.CombatEvents).
		WHERE(table.CombatEvents.SessionID.EQ(Int32(sessionID))).
		ORDER_BY(table.CombatEvents.Timestamp.ASC()).
		Query(db, &dest)
	return dest, wrap("list combat events", err)
}

func ListKillEvents(db *sql.DB, sessionID int32) ([]genmodel.KillEvents, error) {
	var dest []genmodel.KillEvents
	err := SELECT(table.KillEvents.AllColumns).
		FROM(table.KillEvents).
		WHERE(table.KillEvents.SessionID.EQ(Int32(sessionID))).
		ORDER_BY(table.KillEvents.Timestamp.ASC()).
		Query(db, &dest)
	return dest, wrap("list kill events", err)
}

func ListMiningEvents(db *sql.DB, sessionID int32) ([]genmodel.MiningEvents, error) {
	var dest []genmodel.MiningEvents
	err := SELECT(table.MiningEvents.AllColumns).
		FROM(table.MiningEvents).
		WHERE(table.MiningEvents.SessionID.EQ(Int32(sessionID))).
		ORDER_BY(table.MiningEvents.Timestamp.ASC()).
		Query(db, &dest)
	return dest, wrap("list mining events", err)
}

func ListTravelEvents(db *sql.DB, sessionID int32) ([]genmodel.TravelEvents, error) {
	var dest []genmodel.TravelEvents
	err := SELECT(table.TravelEvents.AllColumns).
		FROM(table.TravelEvents).
		WHERE(table.TravelEvents.SessionID.EQ(Int32(sessionID))).
		ORDER_BY(table.TravelEvents.Timestamp.ASC()).
		Query(db, &dest)
	return dest, wrap("list travel events", err)
}

func ListCapEvents(db *sql.DB, sessionID int32) ([]genmodel.CapEvents, error) {
	var dest []genmodel.CapEvents
	err := SELECT(table.CapEvents.AllColumns).
		FROM(table.CapEvents).
		WHERE(table.CapEvents.SessionID.EQ(Int32(sessionID))).
		ORDER_BY(table.CapEvents.Timestamp.ASC()).
		Query(db, &dest)
	return dest, wrap("list cap events", err)
}

func ListReloadEvents(db *sql.DB, sessionID int32) ([]genmodel.ReloadEvents, error) {
	var dest []genmodel.ReloadEvents
	err := SELECT(table.ReloadEvents.AllColumns).
		FROM(table.ReloadEvents).
		WHERE(table.ReloadEvents.SessionID.EQ(Int32(sessionID))).
		ORDER_BY(table.ReloadEvents.Timestamp.ASC()).
		Query(db, &dest)
	return dest, wrap("list reload events", err)
}
