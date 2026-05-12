package main

import (
	"gss/configs"
	"gss/internal/app"
	"gss/internal/infrastructure/logger"
	"gss/pkg/timezone"
)

var VERSION string

func main() {
	timezone.SetTimeZoneICT()

	err := configs.Load()
	if err != nil {
		panic(err)
	}

	cfg := configs.Get()

	logger := logger.New(
		logger.WithLevel(cfg.Logger.Level),
	)

	app, err := app.New(VERSION, cfg, logger)
	if err != nil {
		logger.Fatal("An error happened while creating the app", "err", err)
	}

	if err := app.Run(); err != nil {
		logger.Fatal("An error happened while starting the HTTP server", "err", err)
	}
}
