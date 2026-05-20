package main

import (
	"gss/configs"
	"gss/internal/app"
	"gss/pkg/timezone"
	"log"
)

var VERSION = "0.0.1"

func main() {
	timezone.SetTimeZoneICT()

	if err := configs.Load(); err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	cfg := configs.Get()

	application, err := app.New(VERSION, cfg)
	if err != nil {
		log.Fatalf("failed to initialize app: %v", err)
	}

	if err := application.Run(); err != nil {
		log.Fatalf("app exited with error: %v", err)
	}
}
