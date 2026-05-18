package http

import (
	"fmt"
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
	*gin.Engine
}

func NewRouter(
	logger *logger.Logger,
	version string,
	handlers ...Handler,
) (*Router, error) {
	engine := gin.New()

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

	return &Router{engine}, nil
}

func (r *Router) Handler() http.Handler {
	return r.Engine.Handler()
}
