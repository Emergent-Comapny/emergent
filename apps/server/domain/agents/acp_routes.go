package agents

import (
	"github.com/labstack/echo/v4"

	"github.com/emergent-company/emergent.memory/pkg/auth"
)

// RegisterACPRoutes registers Agent Communication Protocol (ACP) v1 routes.
// All routes are mounted at /acp/v1/ and /agent-chat/v1/ (top-level, not under /api/).
func RegisterACPRoutes(e *echo.Echo, h *ACPHandler, authMiddleware *auth.Middleware) {
	// Deprecated: use /agent-chat/v1 instead. Kept for backward compatibility.
	registerAgentChatRoutes(e, "/acp/v1", h, authMiddleware)

	// Agent Chat API v1 — preferred prefix. Replaces /acp/v1.
	registerAgentChatRoutes(e, "/agent-chat/v1", h, authMiddleware)
}

// registerAgentChatRoutes registers the Agent Chat API v1 route set under the given prefix.
// Handlers and middleware are identical for every prefix.
func registerAgentChatRoutes(e *echo.Echo, prefix string, h *ACPHandler, authMiddleware *auth.Middleware) {
	// --- Ping (no auth) ---
	e.GET(prefix+"/ping", h.Ping)

	// --- Agent discovery (agents:read) ---
	agentsRead := e.Group(prefix + "/agents")
	agentsRead.Use(authMiddleware.RequireAuth())
	agentsRead.Use(authMiddleware.RequireAPITokenScopes("agents:read"))
	agentsRead.GET("", h.ListAgents)
	agentsRead.GET("/:name", h.GetAgent)

	// --- Run lifecycle (mixed read/write) ---
	// Read operations on runs
	runsRead := e.Group(prefix + "/agents/:name/runs")
	runsRead.Use(authMiddleware.RequireAuth())
	runsRead.Use(authMiddleware.RequireAPITokenScopes("agents:read"))
	runsRead.GET("/:runId", h.GetRun)
	runsRead.GET("/:runId/events", h.GetRunEvents)

	// Write operations on runs
	runsWrite := e.Group(prefix + "/agents/:name/runs")
	runsWrite.Use(authMiddleware.RequireAuth())
	runsWrite.Use(authMiddleware.RequireAPITokenScopes("agents:write"))
	runsWrite.POST("", h.CreateRun)
	runsWrite.DELETE("/:runId", h.CancelRun)
	runsWrite.POST("/:runId/resume", h.ResumeRun)

	// --- Sessions ---
	sessionsRead := e.Group(prefix + "/sessions")
	sessionsRead.Use(authMiddleware.RequireAuth())
	sessionsRead.Use(authMiddleware.RequireAPITokenScopes("agents:read"))
	sessionsRead.GET("", h.ListSessions)
	sessionsRead.GET("/:sessionId", h.GetSession)

	sessionsWrite := e.Group(prefix + "/sessions")
	sessionsWrite.Use(authMiddleware.RequireAuth())
	sessionsWrite.Use(authMiddleware.RequireAPITokenScopes("agents:write"))
	sessionsWrite.POST("", h.CreateSession)
	sessionsWrite.PATCH("/:sessionId/archive", h.ArchiveSession)
	sessionsWrite.PATCH("/:sessionId/unarchive", h.UnarchiveSession)
}
