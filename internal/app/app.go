package app

import (
	"context"
	"database/sql"
	"gss/configs"
	deliveryHTTP "gss/internal/delivery/http"
	"gss/internal/delivery/http/handler"
	"gss/internal/delivery/http/handler/auth"
	"gss/internal/delivery/http/handler/health"
	workerpoolHandler "gss/internal/delivery/http/handler/workerpool"
	"gss/internal/infrastructure/logger"
	"gss/internal/infrastructure/orm"
	"gss/internal/repository"
	"gss/pkg/mysql"
	"gss/pkg/workerpool"
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
	wp     *workerpool.Pool
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

	// Initialize Worker Pool
	workers := cfg.WorkerPool.Workers
	if workers <= 0 {
		workers = runtime.NumCPU() * 2
	}
	queueSize := cfg.WorkerPool.QueueSize
	if queueSize <= 0 {
		queueSize = 500
	}

	logger.Info("Initializing worker pool...", "workers", workers, "queueSize", queueSize)
	wp, err := workerpool.New(
		workerpool.WithWorkers(workers),
		workerpool.WithQueueSize(queueSize),
		// Register custom panic handler using WithPanicHandler
		workerpool.WithPanicHandler(func(r any) {
			logger.Error("⚡ [WithPanicHandler] Worker pool recovered from panic!",
				"panic_value", r,
				"timestamp", time.Now().Format(time.RFC3339),
			)
			// Ở đây bạn có thể thêm logic tùy chỉnh như:
			// 1. Gửi thông báo khẩn cấp tới Slack / Telegram Webhook
			// 2. Đẩy chỉ số panic lên Prometheus / Grafana
			// 3. Ghi vết incident vào DB / Dead Letter Queue (DLQ)
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
		workerpoolHandler.NewHandler(baseHandler, wp, logger),
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
		wp:     wp,
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

func (a *App) WorkerPool() *workerpool.Pool {
	return a.wp
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

		if a.wp != nil {
			a.logger.Info("Shutting down worker pool...")
			if err := a.wp.Shutdown(ctx); err != nil {
				a.logger.Error("Error shutting down worker pool", "err", err)
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
