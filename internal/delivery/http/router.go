package http

import (
	"fmt"
	"gss/internal/delivery/http/middleware"
	"gss/internal/infrastructure/logger"
	"net/http"
	"time"

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

type RouterConfig struct {
	Logger         *logger.Logger
	Version        string
	RequestTimeout time.Duration
}

func NewRouter(
	cfg RouterConfig,
	handlers ...Handler,
) (*Router, error) {
	engine := gin.New()

	engine.Use(
		middleware.RequestID(),
		middleware.Recovery(cfg.Logger),
		middleware.Logger(cfg.Logger),
		middleware.Timeout(cfg.RequestTimeout),
	)

	router := ginopenapi.NewRouter(
		engine,
		option.WithSwaggerUI(),
		option.WithTitle("GSS API"),
		option.WithVersion(cfg.Version),
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
