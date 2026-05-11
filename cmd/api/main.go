package main

import (
	"gss/configs"
	"gss/internal/app"
	"gss/pkg/timezone"
)

func main() {
	timezone.SetTimeZoneICT()

	if err := configs.Load(); err != nil {
		panic(err)
	}

	application := app.New()
	application.Start()
}

