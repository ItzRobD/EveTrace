package repo

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/qrm"
	. "github.com/go-jet/jet/v2/sqlite"

	genmodel "EveTrace/.gen/model"
	"EveTrace/.gen/table"
)

// UpsertCharacter finds or creates a character by name, returning their DB ID.
func UpsertCharacter(db *sql.DB, name string) (int32, error) {
	_, err := table.Characters.INSERT(table.Characters.Name).
		VALUES(name).
		ON_CONFLICT(table.Characters.Name).DO_NOTHING().
		Exec(db)
	if err != nil {
		return 0, fmt.Errorf("upsert character %q: %w", name, err)
	}

	var row genmodel.Characters
	err = SELECT(table.Characters.ID).
		FROM(table.Characters).
		WHERE(table.Characters.Name.EQ(String(name))).
		Query(db, &row)
	if err != nil {
		return 0, fmt.Errorf("fetch character id %q: %w", name, err)
	}
	return *row.ID, nil
}

// UpsertSession finds or creates a session by session_key, returning its DB ID.
// If the session already exists from a prior run, the existing row is left
// unchanged so the stored last_byte_offset is preserved for resumption.
func UpsertSession(db *sql.DB, characterID int32, sessionKey, logPath string, startedAt time.Time, lang string) (int32, error) {
	m := genmodel.Sessions{
		CharacterID: characterID,
		SessionKey:  sessionKey,
		LogPath:     logPath,
		StartedAt:   startedAt,
		Language:    lang,
	}
	_, err := table.Sessions.INSERT(table.Sessions.MutableColumns).
		MODEL(m).
		ON_CONFLICT(table.Sessions.SessionKey).DO_NOTHING().
		Exec(db)
	if err != nil {
		return 0, fmt.Errorf("upsert session %q: %w", sessionKey, err)
	}

	var row genmodel.Sessions
	err = SELECT(table.Sessions.ID).
		FROM(table.Sessions).
		WHERE(table.Sessions.SessionKey.EQ(String(sessionKey))).
		Query(db, &row)
	if err != nil {
		return 0, fmt.Errorf("fetch session id %q: %w", sessionKey, err)
	}
	return *row.ID, nil
}

// GetSessionOffset returns the last stored byte offset for sessionKey.
// Returns 0 if the session has never been seen before.
func GetSessionOffset(db *sql.DB, sessionKey string) (int64, error) {
	var row genmodel.Sessions
	err := SELECT(table.Sessions.LastByteOffset).
		FROM(table.Sessions).
		WHERE(table.Sessions.SessionKey.EQ(String(sessionKey))).
		Query(db, &row)
	if errors.Is(err, qrm.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get session offset %q: %w", sessionKey, err)
	}
	return int64(row.LastByteOffset), nil
}

// UpdateSessionOffset persists the latest read position for a session.
// Called by the periodic ticker to checkpoint progress so the tailer can
// resume from this position after a restart.
func UpdateSessionOffset(db *sql.DB, sessionID int32, offset int64) error {
	_, err := table.Sessions.
		UPDATE(table.Sessions.LastByteOffset).
		SET(Int32(int32(offset))).
		WHERE(table.Sessions.ID.EQ(Int32(sessionID))).
		Exec(db)
	if err != nil {
		return fmt.Errorf("update offset session %d: %w", sessionID, err)
	}
	return nil
}
