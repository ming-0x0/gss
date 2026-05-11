package http

import (
	"gss/internal/delivery/http/handler/auth"
	"gss/internal/delivery/http/middleware"
	"gss/internal/infrastructure/logger"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/oaswrap/spec/adapter/ginopenapi"
	"github.com/oaswrap/spec/option"
)

type Router struct {
	engine *gin.Engine
}

func NewRouter(
	authHandler *auth.Handler,
	log *logger.Logger,
	version string,
) *Router {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()

	// Global middleware
	engine.Use(
		middleware.Recovery(log),
		middleware.Logger(log),
	)

	router := &Router{engine: engine}
	router.registerRoutes(authHandler, version)

	return router
}

func (r *Router) registerRoutes(
	authHandler *auth.Handler,
	version string,
) {
	oaRouter := ginopenapi.NewRouter(
		r.engine,
		option.WithSwaggerUI(),
		option.WithTitle("GSS"),
		option.WithVersion(version),
		option.WithSecurity("bearerAuth", option.SecurityHTTPBearer("Bearer")),
	)

	api := oaRouter.Group("/api")
	authHandler.RegisterRoutes(api)
}

// Handler returns the underlying http.Handler for use with net/http server.
func (r *Router) Handler() http.Handler {
	return r.engine
}
