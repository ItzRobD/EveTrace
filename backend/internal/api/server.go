package api

import (
	"context"
	"database/sql"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"EveTrace/web"
)

// Server wraps the Gin engine and HTTP server for graceful shutdown.
type Server struct {
	engine *gin.Engine
	srv    *http.Server
}

// New builds a Gin router with all routes registered and returns a Server
// ready to be started with Run. ctx is the server's root context; it is
// used by background operations such as session replay.
func New(db *sql.DB, hub *Hub, addr string, ctx context.Context, watcherMgr WatcherRestarter, flusher Flusher) *Server {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:4200", "http://localhost:27182"},
		AllowMethods:     []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	h := &handler{db: db, hub: hub, ctx: ctx, watcherMgr: watcherMgr, flusher: flusher}

	apiGrp := r.Group("/api")
	{
		apiGrp.GET("/status", h.getStatus)
		apiGrp.POST("/flush", h.flushEvents)
		apiGrp.GET("/config/presets", h.getLogDirPresets)
		apiGrp.POST("/config/logdir", h.setLogDir)
		apiGrp.POST("/config/mindate", h.setMinDate)

		apiGrp.GET("/characters", h.listCharacters)
		apiGrp.GET("/characters/:id", h.getCharacter)
		apiGrp.DELETE("/characters/:id", h.deleteCharacter)
		apiGrp.GET("/characters/:id/sessions", h.listCharacterSessions)

		apiGrp.GET("/sessions", h.listSessions)
		apiGrp.GET("/sessions/:id", h.getSession)
		apiGrp.DELETE("/sessions/:id", h.deleteSession)
		apiGrp.GET("/sessions/:id/combat", h.listCombat)
		apiGrp.GET("/sessions/:id/kills", h.listKills)
		apiGrp.GET("/sessions/:id/mining", h.listMining)
		apiGrp.GET("/sessions/:id/travel", h.listTravel)
		apiGrp.GET("/sessions/:id/cap", h.listCap)
		apiGrp.GET("/sessions/:id/reload", h.listReload)

		debug := apiGrp.Group("/debug")
		{
			debug.POST("/replay/:id", h.replaySession)
			debug.DELETE("/replay/:id", h.cancelReplay)
		}
	}

	r.GET("/ws", h.serveWS)

	// SPA static file serving (only active when built with -tags embed).
	registerSPA(r)

	return &Server{
		engine: r,
		srv:    &http.Server{Addr: addr, Handler: r},
	}
}

// registerSPA wires up embedded static file serving. When EmbeddedFiles is
// empty (dev build without -tags embed) this is a no-op.
func registerSPA(r *gin.Engine) {
	// Confirm dist was embedded by checking for the root directory.
	// On a stub (non-embed) build, EmbeddedFiles is an empty FS and
	// dist/ won't exist.
	if _, err := web.EmbeddedFiles.Open("dist/browser/index.html"); err != nil {
		// No embedded files (dev build) — API-only mode.
		return
	}

	browserFS, err := fs.Sub(web.EmbeddedFiles, "dist/browser")
	if err != nil {
		return
	}

	fileServer := http.FileServer(http.FS(browserFS))

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// Let API and WS routes through — they are already registered and would
		// only hit NoRoute if somehow unmatched, but guard anyway.
		if strings.HasPrefix(path, "/api/") || path == "/ws" {
			c.Status(http.StatusNotFound)
			return
		}

		// Try to serve the file directly.
		trimmed := strings.TrimPrefix(path, "/")
		if _, err := browserFS.Open(trimmed); err == nil {
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}

		// SPA fallback: serve index.html so Angular's router can handle the path.
		c.Request.URL.Path = "/"
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}

// Run starts the HTTP server in the current goroutine and shuts it down
// gracefully when ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.srv.Shutdown(shutCtx)
	}
}
