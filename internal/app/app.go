package app

import (
	"context"
	"database/sql"
	"gss/configs"
	deliveryHTTP "gss/internal/delivery/http"
	"gss/internal/delivery/http/handler"
	"gss/internal/delivery/http/handler/auth"
	backgroundHandler "gss/internal/delivery/http/handler/background"
	"gss/internal/delivery/http/handler/health"
	"gss/internal/infrastructure/logger"
	"gss/internal/infrastructure/orm"
	"gss/internal/repository"
	"gss/pkg/background"
	"gss/pkg/mysql"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"
)

type App struct {
	cfg    *configs.Config
	db     *sql.DB
	bg     *background.Runner
	srv    *http.Server
	logger *logger.Logger
	wg     sync.WaitGroup
}

func New(
	version string,
	cfg *configs.Config,
	logger *logger.Logger,
) (*App, error) {
	logger.Info("Connecting to database...")
	mysqlDB, err := mysql.New(
		mysql.WithDSN(cfg.MySQL.Host, cfg.MySQL.Port, cfg.MySQL.User, cfg.MySQL.Password, cfg.MySQL.Database),
		mysql.WithMaxIdleConns(cfg.MySQL.MaxIdleConns),
		mysql.WithMaxOpenConns(cfg.MySQL.MaxOpenConns),
		mysql.WithConnMaxLifetime(cfg.MySQL.ConnMaxLifetime),
		mysql.WithConnMaxIdleTime(cfg.MySQL.ConnMaxIdleTime),
	)
	if err != nil {
		return nil, err
	}

	db, err := orm.NewDB(mysqlDB, "mysql")
	if err != nil {
		return nil, err
	}

	db.AddQueryHook(orm.NewLogger(
		orm.WithLogger(logger),
		orm.WithLevel(cfg.Logger.Level),
	))

	// Initialize Background Runner Pool
	workers := cfg.Background.Workers
	if workers <= 0 {
		workers = runtime.NumCPU() * 2
	}
	queueSize := cfg.Background.QueueSize
	if queueSize <= 0 {
		queueSize = 500
	}

	logger.Info("Initializing background pool...", "workers", workers, "queueSize", queueSize)
	bg, err := background.New(
		background.WithWorkers(workers),
		background.WithQueueSize(queueSize),
		// Register custom panic handler using WithPanicHandler
		background.WithPanicHandler(func(r any) {
			logger.Error("⚡ [WithPanicHandler] Background pool recovered from panic!",
				"panic_value", r,
				"timestamp", time.Now().Format(time.RFC3339),
			)
		}),
	)
	if err != nil {
		return nil, err
	}

	repositoryContainer := repository.NewRepositoryContainer(db, logger)

	baseHandler := handler.NewHandler(cfg.Logger.Level)

	handlers := []deliveryHTTP.Handler{
		auth.NewHandler(baseHandler, repositoryContainer.UserRepository, logger),
		health.NewHandler(),
		backgroundHandler.NewHandler(baseHandler, bg, logger),
	}

	requestTimeout := cfg.HTTP.RequestTimeout
	if requestTimeout == 0 {
		requestTimeout = 30 * time.Second
	}

	router, err := deliveryHTTP.NewRouter(
		logger,
		version,
		requestTimeout,
		handlers...,
	)
	if err != nil {
		return nil, err
	}

	addr := cfg.HTTP.Port
	if addr == "" {
		addr = ":8080"
	}

	return &App{
		cfg:    cfg,
		db:     mysqlDB,
		bg:     bg,
		logger: logger,
		srv: &http.Server{
			Addr:         addr,
			Handler:      router.Handler(),
			ReadTimeout:  cfg.HTTP.ReadTimeout,
			WriteTimeout: cfg.HTTP.WriteTimeout,
			IdleTimeout:  cfg.HTTP.IdleTimeout,
		},
	}, nil
}

func (a *App) Background() *background.Runner {
	return a.bg
}

func (a *App) Run() error {
	defer a.wg.Wait()
	return a.start()
}

func (a *App) start() error {
	shutdownTimeout := a.cfg.HTTP.ShutdownTimeout
	if shutdownTimeout == 0 {
		shutdownTimeout = 10 * time.Second
	}

	a.stop(shutdownTimeout, func(ctx context.Context) error {
		return a.srv.Shutdown(ctx)
	})

	a.logger.Info("Server listening on " + a.srv.Addr)
	if err := a.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

func (a *App) stop(
	timeout time.Duration,
	callback func(ctx context.Context) error,
) {
	a.wg.Go(func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(quit)

		sig := <-quit
		a.logger.Info("Received signal " + sig.String() + ". Shutting down...")

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		if err := callback(ctx); err != nil {
			a.logger.Error("Error during graceful HTTP server shutdown", "err", err)
		}

		if a.bg != nil {
			a.logger.Info("Shutting down background pool...")
			if err := a.bg.Shutdown(ctx); err != nil {
				a.logger.Error("Error shutting down background pool", "err", err)
			}
		}

		if a.db != nil {
			a.logger.Info("Closing database connection...")
			if err := a.db.Close(); err != nil {
				a.logger.Error("Error closing database", "err", err)
			}
		}
	})
}
