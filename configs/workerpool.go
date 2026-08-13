package configs

type WorkerPoolConfig struct {
	Workers   int `mapstructure:"WORKER_POOL_WORKERS"`
	QueueSize int `mapstructure:"WORKER_POOL_QUEUE_SIZE"`
}
