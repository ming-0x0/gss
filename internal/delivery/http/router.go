package http

import (
	"fmt"
	"gss/internal/delivery/http/middleware"
	"gss/internal/infrastructure/logger"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/oaswrap/spec/adapter/ginopenapi"
	"github.com/oaswrap/spec/option"
)

type Handler interface {
	RegisterRoutes(r ginopenapi.Router)
}

type Router struct {
	engine *gin.Engine
}

func NewRouter(
	logger *logger.Logger,
	version string,
	handlers ...Handler,
) (*Router, error) {
	engine := gin.New()

	// Global middleware
	engine.Use(
		middleware.Recovery(logger),
		middleware.Logger(logger),
	)

	router := ginopenapi.NewRouter(
		engine,
		option.WithSwaggerUI(),
		option.WithTitle("GSS API"),
		option.WithVersion(version),
		option.WithSecurity("bearerAuth", option.SecurityHTTPBearer("Bearer")),
	)

	api := router.Group("/api")
	for _, h := range handlers {
		h.RegisterRoutes(api)
	}

	if err := router.Validate(); err != nil {
		return nil, fmt.Errorf("openapi spec validation failed: %w", err)
	}

	return &Router{engine: engine}, nil
}

// Handler returns the underlying http.Handler for use with net/http server.
func (r *Router) Handler() http.Handler {
	return r.engine.Handler()
}
