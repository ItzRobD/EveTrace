package api

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// Server wraps the Gin engine and HTTP server for graceful shutdown.
type Server struct {
	engine *gin.Engine
	srv    *http.Server
}

// New builds a Gin router with all routes registered and returns a Server
// ready to be started with Run. ctx is the server's root context; it is
// used by background operations such as session replay.
func New(db *sql.DB, hub *Hub, addr string, ctx context.Context) *Server {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:4200"},
		AllowMethods:     []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	h := &handler{db: db, hub: hub, ctx: ctx}

	api := r.Group("/api")
	{
		api.GET("/characters", h.listCharacters)
		api.GET("/characters/:id", h.getCharacter)
		api.DELETE("/characters/:id", h.deleteCharacter)
		api.GET("/characters/:id/sessions", h.listCharacterSessions)

		api.GET("/sessions", h.listSessions)
		api.GET("/sessions/:id", h.getSession)
		api.DELETE("/sessions/:id", h.deleteSession)
		api.GET("/sessions/:id/combat", h.listCombat)
		api.GET("/sessions/:id/kills", h.listKills)
		api.GET("/sessions/:id/mining", h.listMining)
		api.GET("/sessions/:id/travel", h.listTravel)
		api.GET("/sessions/:id/cap", h.listCap)
		api.GET("/sessions/:id/reload", h.listReload)

		debug := api.Group("/debug")
		{
			debug.POST("/replay/:id", h.replaySession)
			debug.DELETE("/replay/:id", h.cancelReplay)
		}
	}

	r.GET("/ws", h.serveWS)

	return &Server{
		engine: r,
		srv:    &http.Server{Addr: addr, Handler: r},
	}
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
