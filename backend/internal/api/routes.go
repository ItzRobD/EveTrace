package api

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"EveTrace/internal/appconfig"
	"EveTrace/internal/database/repo"
	"EveTrace/internal/logger"
	"EveTrace/internal/metrics"
)

// WatcherRestarter is satisfied by watcher_manager.Manager.
type WatcherRestarter interface {
	LogDir() string
	Restart(newLogDir string)
}

type handler struct {
	db         *sql.DB
	hub        *Hub
	ctx        context.Context
	watcherMgr WatcherRestarter
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins during development; tighten in production.
		return true
	},
}

// envelope is the standard response wrapper for all successful responses.
type envelope[T any] struct {
	Data  T   `json:"data"`
	Count int `json:"count"`
}

func wrap[T any](data T, count int) envelope[T] {
	return envelope[T]{Data: data, Count: count}
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
	c.JSON(http.StatusOK, wrap(rows, len(rows)))
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
	c.JSON(http.StatusOK, wrap(row, 1))
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
	rows, err := repo.ListSessionsWithCount(h.db, id)
	if err != nil {
		dbErr(c, err)
		return
	}
	c.JSON(http.StatusOK, wrap(rows, len(rows)))
}

// ── sessions ──────────────────────────────────────────────────────────────────

func (h *handler) listSessions(c *gin.Context) {
	rows, err := repo.ListSessionsWithCount(h.db, 0)
	if err != nil {
		dbErr(c, err)
		return
	}
	c.JSON(http.StatusOK, wrap(rows, len(rows)))
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
	c.JSON(http.StatusOK, wrap(row, 1))
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
	c.JSON(http.StatusOK, wrap(rows, len(rows)))
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
	c.JSON(http.StatusOK, wrap(rows, len(rows)))
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
	c.JSON(http.StatusOK, wrap(rows, len(rows)))
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
	c.JSON(http.StatusOK, wrap(rows, len(rows)))
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
	c.JSON(http.StatusOK, wrap(rows, len(rows)))
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
	c.JSON(http.StatusOK, wrap(rows, len(rows)))
}

// ── log dir presets ───────────────────────────────────────────────────────────

func (h *handler) getLogDirPresets(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"presets": logDirPresets()})
}

// ── status / config ───────────────────────────────────────────────────────────

type statusResponse struct {
	LogDir          string `json:"logDir"`
	MinDate         string `json:"minDate"`
	EventsProcessed int64  `json:"eventsProcessed"`
	SessionsOpened  int64  `json:"sessionsOpened"`
	WSClients       int32  `json:"wsClients"`
}

func (h *handler) getStatus(c *gin.Context) {
	var logDir string
	if h.watcherMgr != nil {
		logDir = h.watcherMgr.LogDir()
	}
	cfg := appconfig.Get()
	c.JSON(http.StatusOK, statusResponse{
		LogDir:          logDir,
		MinDate:         cfg.MinDate,
		EventsProcessed: metrics.EventsProcessed.Load(),
		SessionsOpened:  metrics.SessionsOpened.Load(),
		WSClients:       metrics.WSClients.Load(),
	})
}

func (h *handler) setMinDate(c *gin.Context) {
	var body struct {
		MinDate string `json:"minDate"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// An empty MinDate clears the filter; any non-empty value must be RFC3339.
	if body.MinDate != "" {
		if _, err := time.Parse(time.RFC3339, body.MinDate); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "minDate must be an RFC3339 timestamp"})
			return
		}
	}
	if err := appconfig.SetMinDate(body.MinDate); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist minDate"})
		logger.Error("persist minDate failed", "err", err)
		return
	}

	// Restart watcher to apply filtering to existing files
	if h.watcherMgr != nil {
		h.watcherMgr.Restart(h.watcherMgr.LogDir())
	}

	cfg := appconfig.Get()
	c.JSON(http.StatusOK, statusResponse{
		LogDir:          cfg.LogDir,
		MinDate:         cfg.MinDate,
		EventsProcessed: metrics.EventsProcessed.Load(),
		SessionsOpened:  metrics.SessionsOpened.Load(),
		WSClients:       metrics.WSClients.Load(),
	})
}

func (h *handler) setLogDir(c *gin.Context) {
	var body struct {
		LogDir string `json:"logDir" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if h.watcherMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "watcher not available"})
		return
	}
	if err := appconfig.SetLogDir(body.LogDir); err != nil {
		// Non-fatal: log it but still apply the change in-memory.
		logger.Error("persist logDir failed", "err", err)
	}
	h.watcherMgr.Restart(body.LogDir)
	cfg := appconfig.Get()
	c.JSON(http.StatusOK, statusResponse{
		LogDir:          body.LogDir,
		MinDate:         cfg.MinDate,
		EventsProcessed: metrics.EventsProcessed.Load(),
		SessionsOpened:  metrics.SessionsOpened.Load(),
		WSClients:       metrics.WSClients.Load(),
	})
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
