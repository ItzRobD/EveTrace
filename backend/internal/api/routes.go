package api

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"EveTrace/internal/database/repo"
)

type handler struct {
	db  *sql.DB
	hub *Hub
	ctx context.Context
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins during development; tighten in production.
		return true
	},
}

// ── helpers ───────────────────────────────────────────────────────────────────

func parseID(c *gin.Context, param string) (int32, bool) {
	v, err := strconv.Atoi(c.Param(param))
	if err != nil || v <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return 0, false
	}
	return int32(v), true
}

func dbErr(c *gin.Context, err error) {
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}

// ── characters ────────────────────────────────────────────────────────────────

func (h *handler) listCharacters(c *gin.Context) {
	rows, err := repo.ListCharacters(h.db)
	if err != nil {
		dbErr(c, err)
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (h *handler) getCharacter(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	row, err := repo.GetCharacter(h.db, id)
	if err != nil {
		dbErr(c, err)
		return
	}
	if row.ID == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, row)
}

func (h *handler) deleteCharacter(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := repo.DeleteCharacter(h.db, id); err != nil {
		dbErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *handler) listCharacterSessions(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	rows, err := repo.ListSessions(h.db, id)
	if err != nil {
		dbErr(c, err)
		return
	}
	c.JSON(http.StatusOK, rows)
}

// ── sessions ──────────────────────────────────────────────────────────────────

func (h *handler) listSessions(c *gin.Context) {
	rows, err := repo.ListSessions(h.db, 0)
	if err != nil {
		dbErr(c, err)
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (h *handler) getSession(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	row, err := repo.GetSession(h.db, id)
	if err != nil {
		dbErr(c, err)
		return
	}
	if row.ID == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, row)
}

func (h *handler) deleteSession(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := repo.DeleteSession(h.db, id); err != nil {
		dbErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ── events ────────────────────────────────────────────────────────────────────

func (h *handler) listCombat(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	rows, err := repo.ListCombatEvents(h.db, id)
	if err != nil {
		dbErr(c, err)
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (h *handler) listKills(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	rows, err := repo.ListKillEvents(h.db, id)
	if err != nil {
		dbErr(c, err)
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (h *handler) listMining(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	rows, err := repo.ListMiningEvents(h.db, id)
	if err != nil {
		dbErr(c, err)
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (h *handler) listTravel(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	rows, err := repo.ListTravelEvents(h.db, id)
	if err != nil {
		dbErr(c, err)
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (h *handler) listCap(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	rows, err := repo.ListCapEvents(h.db, id)
	if err != nil {
		dbErr(c, err)
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (h *handler) listReload(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	rows, err := repo.ListReloadEvents(h.db, id)
	if err != nil {
		dbErr(c, err)
		return
	}
	c.JSON(http.StatusOK, rows)
}

// ── websocket ─────────────────────────────────────────────────────────────────

func (h *handler) serveWS(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	client := &Client{hub: h.hub, conn: conn, send: make(chan []byte, 512)}
	h.hub.register <- client
	go client.writePump()
	go client.readPump()
}
