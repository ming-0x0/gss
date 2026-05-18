package configs

import "time"

type HTTPConfig struct {
	Port           string        `mapstructure:"HTTP_PORT"`
	ReadTimeout    time.Duration `mapstructure:"HTTP_READ_TIMEOUT"`
	WriteTimeout   time.Duration `mapstructure:"HTTP_WRITE_TIMEOUT"`
	IdleTimeout    time.Duration `mapstructure:"HTTP_IDLE_TIMEOUT"`
	RequestTimeout time.Duration `mapstructure:"HTTP_REQUEST_TIMEOUT"`
}
