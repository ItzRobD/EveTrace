package repo

import (
	"database/sql"
	"errors"

	"github.com/go-jet/jet/v2/qrm"
	. "github.com/go-jet/jet/v2/sqlite"

	genmodel "EveTrace/.gen/model"
	"EveTrace/.gen/table"
)

// SessionWithCount extends a session row with the total number of events
// recorded across all event tables for that session.
type SessionWithCount struct {
	genmodel.Sessions
	EventCount int64 `json:"EventCount"`
}

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

// ListSessionsWithCount returns sessions with a total event count across all
// event tables. If characterID > 0 it filters by character.
func ListSessionsWithCount(db *sql.DB, characterID int32) ([]SessionWithCount, error) {
	where := ""
	args := []any{}
	if characterID > 0 {
		where = "WHERE s.character_id = ?"
		args = append(args, characterID)
	}
	q := `
SELECT
  s.id, s.character_id, s.session_key, s.log_path, s.started_at, s.language, s.last_byte_offset,
  (SELECT COUNT(*) FROM combat_events      WHERE session_id = s.id) +
  (SELECT COUNT(*) FROM kill_events        WHERE session_id = s.id) +
  (SELECT COUNT(*) FROM mining_events      WHERE session_id = s.id) +
  (SELECT COUNT(*) FROM mining_full_events WHERE session_id = s.id) +
  (SELECT COUNT(*) FROM travel_events      WHERE session_id = s.id) +
  (SELECT COUNT(*) FROM cap_events         WHERE session_id = s.id) +
  (SELECT COUNT(*) FROM reload_events      WHERE session_id = s.id) AS event_count
FROM sessions s
` + where + `
ORDER BY s.started_at DESC`

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, wrap("list sessions with count", err)
	}
	defer rows.Close()

	var dest []SessionWithCount
	for rows.Next() {
		var r SessionWithCount
		if err := rows.Scan(
			&r.ID, &r.CharacterID, &r.SessionKey, &r.LogPath,
			&r.StartedAt, &r.Language, &r.LastByteOffset,
			&r.EventCount,
		); err != nil {
			return nil, wrap("scan session with count", err)
		}
		dest = append(dest, r)
	}
	if dest == nil {
		dest = []SessionWithCount{}
	}
	return dest, wrap("list sessions with count", rows.Err())
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
