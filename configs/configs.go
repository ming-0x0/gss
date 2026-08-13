package configs

import (
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

type Config struct {
	HTTP       HTTPConfig       `mapstructure:",squash"`
	Logger     LoggerConfig     `mapstructure:",squash"`
	MySQL      MySQLConfig      `mapstructure:",squash"`
	WorkerPool WorkerPoolConfig `mapstructure:",squash"`
}

var ptr atomic.Pointer[Config]

func init() {
	ptr.Store(&Config{})
}

func Load() error {
	v := viper.New()

	v.SetConfigFile(".env")
	v.SetConfigType("env")
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return err
	}

	cfg, err := parse(v)
	if err != nil {
		return err
	}
	ptr.Store(cfg)

	v.WatchConfig()
	v.OnConfigChange(func(e fsnotify.Event) {
		cfg, err := parse(v)
		if err == nil {
			ptr.Store(cfg)
		}
	})

	return nil
}

func parse(v *viper.Viper) (*Config, error) {
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func Get() *Config {
	return ptr.Load()
}
