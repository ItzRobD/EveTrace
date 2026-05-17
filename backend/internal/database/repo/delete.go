package repo

import (
	"database/sql"
	"fmt"

	. "github.com/go-jet/jet/v2/sqlite"

	"EveTrace/.gen/table"
)

// DeleteSession removes a session and all of its events in a single transaction.
// This is the primary delete operation exposed to the UI for removing duplicate
// or unwanted session data.
func DeleteSession(db *sql.DB, sessionID int32) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("delete session %d: begin tx: %w", sessionID, err)
	}
	defer tx.Rollback() //nolint:errcheck

	id := Int32(sessionID)
	steps := []struct {
		name string
		stmt DeleteStatement
	}{
		{"combat_events", table.CombatEvents.DELETE().WHERE(table.CombatEvents.SessionID.EQ(id))},
		{"kill_events", table.KillEvents.DELETE().WHERE(table.KillEvents.SessionID.EQ(id))},
		{"mining_events", table.MiningEvents.DELETE().WHERE(table.MiningEvents.SessionID.EQ(id))},
		{"mining_full_events", table.MiningFullEvents.DELETE().WHERE(table.MiningFullEvents.SessionID.EQ(id))},
		{"travel_events", table.TravelEvents.DELETE().WHERE(table.TravelEvents.SessionID.EQ(id))},
		{"cap_events", table.CapEvents.DELETE().WHERE(table.CapEvents.SessionID.EQ(id))},
		{"reload_events", table.ReloadEvents.DELETE().WHERE(table.ReloadEvents.SessionID.EQ(id))},
		{"sessions", table.Sessions.DELETE().WHERE(table.Sessions.ID.EQ(id))},
	}

	for _, s := range steps {
		if _, err := s.stmt.Exec(tx); err != nil {
			return fmt.Errorf("delete session %d: %s: %w", sessionID, s.name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("delete session %d: commit: %w", sessionID, err)
	}
	return nil
}

// DeleteCharacter removes a character and all of their sessions and events in a
// single transaction. Use with care — this permanently removes all history for
// the character across every log file.
func DeleteCharacter(db *sql.DB, characterID int32) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("delete character %d: begin tx: %w", characterID, err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Collect all session IDs belonging to this character so we can delete
	// their events before removing the sessions themselves.
	sessionIDs, err := characterSessionIDs(tx, characterID)
	if err != nil {
		return fmt.Errorf("delete character %d: list sessions: %w", characterID, err)
	}

	for _, sid := range sessionIDs {
		id := Int32(sid)
		eventDeletes := []struct {
			name string
			stmt DeleteStatement
		}{
			{"combat_events", table.CombatEvents.DELETE().WHERE(table.CombatEvents.SessionID.EQ(id))},
			{"kill_events", table.KillEvents.DELETE().WHERE(table.KillEvents.SessionID.EQ(id))},
			{"mining_events", table.MiningEvents.DELETE().WHERE(table.MiningEvents.SessionID.EQ(id))},
			{"mining_full_events", table.MiningFullEvents.DELETE().WHERE(table.MiningFullEvents.SessionID.EQ(id))},
			{"travel_events", table.TravelEvents.DELETE().WHERE(table.TravelEvents.SessionID.EQ(id))},
			{"cap_events", table.CapEvents.DELETE().WHERE(table.CapEvents.SessionID.EQ(id))},
			{"reload_events", table.ReloadEvents.DELETE().WHERE(table.ReloadEvents.SessionID.EQ(id))},
		}
		for _, s := range eventDeletes {
			if _, err := s.stmt.Exec(tx); err != nil {
				return fmt.Errorf("delete character %d: session %d: %s: %w", characterID, sid, s.name, err)
			}
		}
	}

	if _, err := table.Sessions.DELETE().
		WHERE(table.Sessions.CharacterID.EQ(Int32(characterID))).
		Exec(tx); err != nil {
		return fmt.Errorf("delete character %d: sessions: %w", characterID, err)
	}

	if _, err := table.Characters.DELETE().
		WHERE(table.Characters.ID.EQ(Int32(characterID))).
		Exec(tx); err != nil {
		return fmt.Errorf("delete character %d: characters: %w", characterID, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("delete character %d: commit: %w", characterID, err)
	}
	return nil
}

// characterSessionIDs returns all session IDs for a character, used to
// enumerate which event rows need to be removed before the sessions row.
func characterSessionIDs(tx *sql.Tx, characterID int32) ([]int32, error) {
	rows, err := tx.Query(
		`SELECT id FROM sessions WHERE character_id = ?`, characterID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int32
	for rows.Next() {
		var id int32
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
