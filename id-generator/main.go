package main

import (
	"context"
	"flag"
	"log/slog"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jxskiss/base62"
	"github.com/redis/go-redis/v9"
	"gopkg.in/yaml.v3"
)

type Config struct {
	// Behavior (from yaml)
	RefillThreshold int64         `yaml:"refill_threshold"`
	IdleInterval    time.Duration `yaml:"idle_interval"`
	StepSizeRules   []int         `yaml:"step_size_rules"`
	MaxPushRetries  int           `yaml:"max_push_retries"`
	RedisKeyIDs     string        `yaml:"redis_key_ids"`
	RedisKeyCounter string        `yaml:"redis_key_counter"`

	// Infra (from yaml, but overridden by ENV)
	RedisAddr string `yaml:"redis_addr"`
	Debug     bool   `yaml:"debug"`

	// Secrets (ENV only)
	RedisPassword string
}

// LoadConfig provides defaults and overrides with environment variables
func LoadConfig(path string) (*Config, error) {
	// Set Defaults
	cfg := &Config{
		RefillThreshold: 10000,
		IdleInterval:    2 * time.Second,
		MaxPushRetries:  3,
		StepSizeRules:   []int{100, 500, 2000, 5000},
		RedisKeyIDs:     "IDs",
		RedisKeyCounter: "GLOBAL_COUNTER",
		RedisAddr:       "localhost:6379",
		Debug:           os.Getenv("APP_DEBUG") == "true",
	}

	file, err := os.ReadFile(path)
	if err == nil {
		if err := yaml.Unmarshal(file, &cfg); err != nil {
			return nil, err
		}
	}

	if envAddr := os.Getenv("REDIS_ADDR"); envAddr != "" {
		cfg.RedisAddr = envAddr
	}
	if val := os.Getenv("REDIS_PASSWORD"); val != "" {
		cfg.RedisPassword = val
	}

	return cfg, nil
}

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		slog.Error("Could not load config", "error", err)
		os.Exit(1)
	}
	setupLogger(cfg.Debug)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
	})

	slog.Info("ID Generator service starting", "target_batch", cfg.RefillThreshold)
	runRefillWorker(ctx, rdb, cfg)
}

func runRefillWorker(ctx context.Context, rdb *redis.Client, cfg *Config) {
	ruleIndex := 0

	for {
		if ctx.Err() != nil {
			slog.Info("Shutdown signal received, gracefully exiting")
			return
		}

		count, err := rdb.LLen(ctx, cfg.RedisKeyIDs).Result()
		if err != nil {
			slog.Error("Failed to check stock level", "error", err)
			waitOrExit(ctx, cfg.IdleInterval)
			continue
		}

		if count >= cfg.RefillThreshold {
			slog.Debug("Stock healthy, idling", "current", count, "target", cfg.RefillThreshold)
			waitOrExit(ctx, cfg.IdleInterval)
			continue
		}

		stepSize := cfg.StepSizeRules[ruleIndex]
		baseIndex, err := rdb.IncrBy(ctx, cfg.RedisKeyCounter, int64(stepSize)).Result()
		if err != nil {
			slog.Error("Failed to reserve ID range", "error", err)
			waitOrExit(ctx, cfg.IdleInterval)
			continue
		}
		startIndex := baseIndex - int64(stepSize)

		slog.Info("Generating IDs", "start", startIndex, "count", stepSize)
		ids := generateBase62Batch(uint64(startIndex), stepSize)

		err = pushWithRetry(ctx, rdb, ids, cfg)
		if err != nil {
			slog.Error("Critical failure: could not push IDs after retries", "error", err)
			continue
		}

		if ruleIndex < len(cfg.StepSizeRules)-1 {
			ruleIndex++
		}
	}
}

func pushWithRetry(ctx context.Context, rdb *redis.Client, ids []string, cfg *Config) error {
	var lastErr error
	for i := 0; i < cfg.MaxPushRetries; i++ {
		err := rdb.RPush(ctx, cfg.RedisKeyIDs, ids).Err()
		if err == nil {
			slog.Info("Successfully pushed batch to Redis", "size", len(ids))
			return nil
		}

		lastErr = err
		slog.Warn("Push failed, retrying...", "attempt", i+1, "error", err)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second * time.Duration(i+1)):
		}
	}
	return lastErr
}

func waitOrExit(ctx context.Context, duration time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(duration + time.Millisecond*time.Duration(rand.Intn(300))):
	}
}

func generateBase62Batch(start uint64, size int) []string {
	ids := make([]string, 0, size)
	for i := uint64(0); i < uint64(size); i++ {
		ids = append(ids, string(base62.FormatUint(start+i)))
	}
	return ids
}

func setupLogger(debug bool) {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	h := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(h))
}
