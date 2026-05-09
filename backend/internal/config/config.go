package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type RedisInstance struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"-"` // Secret
}

type WorkerOptions struct {
	IdleInterval time.Duration `yaml:"idle_interval"`
	PullLimit    int           `yaml:"pull_limit"`
}

type RateLimitConfig struct {
	Shorten struct {
		Count  int           `yaml:"count"`
		Period time.Duration `yaml:"period"`
	} `yaml:"shorten"`

	Redirect struct {
		Count  int           `yaml:"count"`
		Period time.Duration `yaml:"period"`
	} `yaml:"redirect"`
}

type CacheOptions struct {
	MaxWeight int    `yaml:"max_weight"`
	Expiry    string `yaml:"expiry"`
}

type Config struct {
	Server struct {
		Port  string `yaml:"port"`
		Debug bool   `yaml:"debug"`
	} `yaml:"server"`

	PersistentRedis struct {
		RedisInstance `yaml:",inline"`
		RedisKeyIDs   string `yaml:"redis_key_ids"`
	} `yaml:"persistent_redis"`

	VolatileRedis RedisInstance `yaml:"volatile_redis"`

	AWS struct {
		Region      string `yaml:"region"`
		DynamoTable string `yaml:"dynamo_table"`
	} `yaml:"aws"`

	Buffer struct {
		BufferIDSize int `yaml:"buffer_id_size"`
		MaxRetries   int `yaml:"max_retries"`
	} `yaml:"buffer"`

	Worker WorkerOptions `yaml:"worker"`

	RateLimit RateLimitConfig `yaml:"rate_limit"`

	Cache CacheOptions `yaml:"cache"`
}

func LoadConfig(configPath string) (*Config, error) {
	cfg := &Config{}

	// Core Defaults
	cfg.Server.Port = ":8080"
	cfg.Buffer.MaxRetries = 3
	cfg.Buffer.BufferIDSize = 5000
	cfg.Worker.IdleInterval = 100 * time.Millisecond
	cfg.Worker.PullLimit = 4000
	cfg.Server.Debug = os.Getenv("APP_DEBUG") == "true"

	file, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(file, &cfg)
	if err != nil {
		return nil, err
	}

	cfg.PersistentRedis.Password = os.Getenv("REDIS_PERSISTENT_PASS")
	cfg.VolatileRedis.Password = os.Getenv("REDIS_VOLATILE_PASS")

	if addr := os.Getenv("REDIS_PERSISTENT_ADDR"); addr != "" {
		cfg.PersistentRedis.Addr = addr
	}
	if addr := os.Getenv("REDIS_VOLATILE_ADDR"); addr != "" {
		cfg.VolatileRedis.Addr = addr
	}

	if region := os.Getenv("AWS_REGION"); region != "" {
		cfg.AWS.Region = region
	}
	if table := os.Getenv("DYNAMO_TABLE"); table != "" {
		cfg.AWS.DynamoTable = table
	}

	return cfg, nil
}
