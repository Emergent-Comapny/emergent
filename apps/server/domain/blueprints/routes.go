package blueprints

import (
	"github.com/labstack/echo/v4"

	"github.com/emergent-company/emergent.memory/pkg/auth"
)

// RegisterRoutes registers blueprint routes
func RegisterRoutes(e *echo.Echo, h *Handler, authMiddleware *auth.Middleware) {
	// All blueprint endpoints require authentication
	g := e.Group("/api/blueprints")
	g.Use(authMiddleware.RequireAuth())

	// Static sub-paths must be registered before /:id so they are not swallowed.
	g.GET("/name/:name/versions", h.ListVersionsByName)

	g.POST("", h.CreateBlueprint)
	g.GET("", h.ListBlueprints)
	g.GET("/:id", h.GetBlueprint)
	g.PUT("/:id", h.UpdateBlueprint)
	g.POST("/:id/publish", h.PublishBlueprint)
	g.POST("/:id/deprecate", h.DeprecateBlueprint)
	g.POST("/:id/versions", h.NewVersion)
	// Apply is project-scoped: RequireProjectID enforces the X-Project-ID
	// header which populates user.ProjectID (the target project).
	g.POST("/:id/apply", h.ApplyBlueprint, authMiddleware.RequireProjectID())
	g.DELETE("/:id", h.DeleteBlueprint)
}
