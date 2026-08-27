package blueprints

import (
	"go.uber.org/fx"
)

// Module provides the blueprints domain
var Module = fx.Module("blueprints",
	fx.Provide(NewRepository),
	fx.Provide(NewService),
	fx.Provide(NewHandler),
	fx.Invoke(RegisterRoutes),
)
