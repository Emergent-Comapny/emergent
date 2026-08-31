package blueprints

import (
	"go.uber.org/fx"

	"github.com/emergent-company/emergent.memory/domain/mcp"
)

// Module provides the blueprints domain
var Module = fx.Module("blueprints",
	fx.Provide(NewRepository),
	fx.Provide(NewService),
	fx.Provide(NewHandler),
	fx.Provide(provideMCPBlueprintToolHandler),
	fx.Invoke(RegisterRoutes),
	fx.Invoke(registerBlueprintToolHandler),
)

// provideMCPBlueprintToolHandler creates an MCPBlueprintToolHandler from fx dependencies.
func provideMCPBlueprintToolHandler(svc *Service) *MCPBlueprintToolHandler {
	return NewMCPBlueprintToolHandler(svc)
}

// registerBlueprintToolHandler injects the MCPBlueprintToolHandler into the MCP
// Service via setter injection to break the circular dependency (blueprints → mcp).
func registerBlueprintToolHandler(mcpService *mcp.Service, handler *MCPBlueprintToolHandler) {
	mcpService.SetBlueprintToolHandler(handler)
}
