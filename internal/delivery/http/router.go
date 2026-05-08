package http

import (
	"gss/internal/delivery/http/handler/auth"

	"github.com/gin-gonic/gin"
	"github.com/oaswrap/spec/adapter/ginopenapi"
	"github.com/oaswrap/spec/option"
)

type Router struct {
	engine *gin.Engine
}

func NewRouter(
	authHandler *auth.Handler,
	version string,
) *Router {
	engine := gin.Default()

	router := &Router{engine: engine}
	router.registerRoutes(authHandler, version)

	return router
}

func (r *Router) registerRoutes(
	authHandler *auth.Handler,
	version string,
) {
	router := ginopenapi.NewRouter(
		r.engine,
		option.WithSwaggerUI(),
		option.WithTitle("GSS"),
		option.WithVersion(version),
		option.WithSecurity("bearerAuth", option.SecurityHTTPBearer("Bearer")),
	)

	api := router.Group("/api")
	authHandler.RegisterRoutes(api)
}

func (r *Router) Run(addr string) error {
	return r.engine.Run(addr)
}
