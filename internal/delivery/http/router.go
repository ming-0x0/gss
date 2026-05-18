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
	version  string
	timeout  time.Duration
	engine   *gin.Engine
	logger   *logger.Logger
	handlers []Handler
}

type Option func(*Router)

func WithVersion(version string) Option {
	return func(r *Router) {
		r.version = version
	}
}

func WithTimeout(timeout int) Option {
	return func(r *Router) {
		r.timeout = time.Duration(timeout)
	}
}

func WithLogger(logger *logger.Logger) Option {
	return func(r *Router) {
		r.logger = logger
	}
}

func WithHandlers(handlers ...Handler) Option {
	return func(r *Router) {
		r.handlers = append(r.handlers, handlers...)
	}
}

func NewRouter(
	opts ...Option,
) (*Router, error) {
	r := &Router{}
	for _, opt := range opts {
		opt(r)
	}

	engine := gin.New()
	r.engine = engine

	engine.Use(
		middleware.RequestID(),
		middleware.Recovery(r.logger),
		middleware.Logger(r.logger),
		middleware.Timeout(r.timeout),
	)

	router := ginopenapi.NewRouter(
		engine,
		option.WithSwaggerUI(),
		option.WithTitle("GSS API"),
		option.WithVersion(r.version),
		option.WithSecurity("bearerAuth", option.SecurityHTTPBearer("Bearer")),
	)

	api := router.Group("/api")
	for _, h := range r.handlers {
		h.RegisterRoutes(api)
	}

	if err := router.Validate(); err != nil {
		return nil, fmt.Errorf("openapi spec validation failed: %w", err)
	}

	return r, nil
}

func (r *Router) Handler() http.Handler {
	return r.engine
}
