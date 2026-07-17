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

// Flusher is satisfied by repo.EventBuffer — it exposes the pending-write state
// and lets the API force an immediate flush.
type Flusher interface {
	RequestFlush(ctx context.Context) (int, error)
	Stats() (pending int, secondsToNextFlush int)
}

type handler struct {
	db         *sql.DB
	hub        *Hub
	ctx        context.Context
	shutdownFn func() // cancels the root context to trigger graceful shutdown
	watcherMgr WatcherRestarter
	flusher    Flusher
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
	LogDir             string `json:"logDir"`
	MinDate            string `json:"minDate"`
	IdleTimeoutSeconds int    `json:"idleTimeoutSeconds"`
	EventsProcessed    int64  `json:"eventsProcessed"`
	SessionsOpened     int64  `json:"sessionsOpened"`
	WSClients          int32  `json:"wsClients"`
	PendingEvents      int    `json:"pendingEvents"`
	SecondsToNextFlush int    `json:"secondsToNextFlush"`
}

// statusPayload assembles the current status snapshot. logDir is passed in since
// callers source it differently (live watcher dir vs. the value just persisted).
func (h *handler) statusPayload(logDir string) statusResponse {
	var pending, nextFlush int
	if h.flusher != nil {
		pending, nextFlush = h.flusher.Stats()
	}
	return statusResponse{
		LogDir:             logDir,
		MinDate:            appconfig.Get().MinDate,
		IdleTimeoutSeconds: appconfig.Get().IdleTimeoutSecs(),
		EventsProcessed:    metrics.EventsProcessed.Load(),
		SessionsOpened:     metrics.SessionsOpened.Load(),
		WSClients:          metrics.WSClients.Load(),
		PendingEvents:      pending,
		SecondsToNextFlush: nextFlush,
	}
}

func (h *handler) getStatus(c *gin.Context) {
	var logDir string
	if h.watcherMgr != nil {
		logDir = h.watcherMgr.LogDir()
	}
	c.JSON(http.StatusOK, h.statusPayload(logDir))
}

// flushEvents forces buffered events to be written to the database immediately,
// rather than waiting for the periodic flush.
func (h *handler) flushEvents(c *gin.Context) {
	if h.flusher == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "flusher not available"})
		return
	}
	n, err := h.flusher.RequestFlush(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var logDir string
	if h.watcherMgr != nil {
		logDir = h.watcherMgr.LogDir()
	}
	resp := h.statusPayload(logDir)
	c.JSON(http.StatusOK, gin.H{"flushed": n, "status": resp})
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

	c.JSON(http.StatusOK, h.statusPayload(appconfig.Get().LogDir))
}

func (h *handler) setIdleTimeout(c *gin.Context) {
	var body struct {
		Seconds int `json:"seconds"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.Seconds < 0 {
		body.Seconds = 0
	}
	if err := appconfig.SetIdleTimeoutSeconds(body.Seconds); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist idle timeout"})
		logger.Error("persist idleTimeout failed", "err", err)
		return
	}
	// Apply to the running hub so the next window-close uses the new value.
	if h.hub != nil {
		h.hub.UpdateIdleTimeout(time.Duration(body.Seconds) * time.Second)
	}

	var logDir string
	if h.watcherMgr != nil {
		logDir = h.watcherMgr.LogDir()
	}
	c.JSON(http.StatusOK, h.statusPayload(logDir))
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
	c.JSON(http.StatusOK, h.statusPayload(body.LogDir))
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
