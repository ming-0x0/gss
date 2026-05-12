package http

import (
	"gss/internal/delivery/http/middleware"
	"gss/internal/infrastructure/logger"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/oaswrap/spec/adapter/ginopenapi"
	"github.com/oaswrap/spec/option"
)

type router interface {
	RegisterRoutes(r ginopenapi.Router)
}

type Router struct {
	engine *gin.Engine
}

func NewRouter(
	log *logger.Logger,
	version string,
	routes ...router,
) *Router {
	engine := gin.New()

	// Global middleware
	engine.Use(
		middleware.Recovery(log),
		middleware.Logger(log),
	)

	oaRouter := ginopenapi.NewRouter(
		engine,
		option.WithSwaggerUI(),
		option.WithTitle("GSS API"),
		option.WithVersion(version),
		option.WithSecurity("bearerAuth", option.SecurityHTTPBearer("Bearer")),
	)

	api := oaRouter.Group("/api")
	for _, r := range routes {
		r.RegisterRoutes(api)
	}

	return &Router{engine: engine}
}

// Handler returns the underlying http.Handler for use with net/http server.
func (r *Router) Handler() http.Handler {
	return r.engine
}
