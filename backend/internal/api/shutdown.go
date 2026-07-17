package api

import (
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"EveTrace/internal/logger"
)

// shutdown gracefully stops EveTrace on request from the UI's Quit button.
//
// It is restricted to loopback clients: the server binds all interfaces by
// default (:27182), so killing the app must not be reachable from the LAN.
// The response is sent first, then h.shutdownFn cancels the root context after
// a short delay — that drives the same graceful path Ctrl-C uses (HTTP server
// drain + final buffer flush), so buffered events are persisted before exit.
func (h *handler) shutdown(c *gin.Context) {
	if !clientIsLoopback(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "shutdown is only permitted from the local machine"})
		return
	}

	logger.Info("shutdown requested via API", "remote", c.Request.RemoteAddr)
	c.JSON(http.StatusAccepted, gin.H{"status": "shutting down"})

	if h.shutdownFn != nil {
		// Delay so this 202 reaches the browser before the server tears down.
		time.AfterFunc(200*time.Millisecond, h.shutdownFn)
	}
}

// clientIsLoopback reports whether the request's real socket peer is a loopback
// address. It reads RemoteAddr directly (not gin's ClientIP) so a spoofed
// X-Forwarded-For header cannot bypass the guard.
func clientIsLoopback(c *gin.Context) bool {
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		host = c.Request.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
