package main

import (
	"context"
	"log/slog"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jxskiss/base62"
	"github.com/redis/go-redis/v9"
)

var (
	TargetBatchSize, _ = strconv.ParseInt(os.Getenv("TargetBatchSize"), 10, 64)
	RedisKeyIDs        = os.Getenv("RedisKeyIDs")
	RedisKeyCounter    = os.Getenv("RedisKeyCounter")
)

const (
	IdleInterval   = 2 * time.Second
	MaxPushRetries = 3
)

var StepSizeRules = []int{100, 500, 2000, 5000}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	setupLogger()

	slog.Info("ID Generator service starting", "target_batch", TargetBatchSize)
	runRefillWorker(ctx, rdb)
}

func runRefillWorker(ctx context.Context, rdb *redis.Client) {
	ruleIndex := 0

	for {
		if ctx.Err() != nil {
			slog.Info("Shutdown signal received, gracefully exiting")
			return
		}

		// Check stock level
		count, err := rdb.LLen(ctx, RedisKeyIDs).Result()
		if err != nil {
			slog.Error("Failed to check stock level", "error", err)
			waitOrExit(ctx, IdleInterval)
			continue
		}

		if count >= TargetBatchSize {
			slog.Debug("Stock healthy, idling", "current", count, "target", TargetBatchSize)
			waitOrExit(ctx, IdleInterval)
			continue
		}

		// Reserve Range
		stepSize := StepSizeRules[ruleIndex]
		baseIndex, err := rdb.IncrBy(ctx, RedisKeyCounter, int64(stepSize)).Result()
		if err != nil {
			slog.Error("Failed to reserve ID range", "error", err)
			waitOrExit(ctx, IdleInterval)
			continue
		}
		startIndex := baseIndex - int64(stepSize)

		// Generate IDs
		slog.Info("Generating IDs", "start", startIndex, "count", stepSize)
		ids := generateBase62Batch(uint64(startIndex), stepSize)

		// Push to Redis with Retry
		err = pushWithRetry(ctx, rdb, ids)
		if err != nil {
			slog.Error("Critical failure: could not push IDs after retries", "error", err)
			continue
		}

		// Advance rule index
		if ruleIndex < len(StepSizeRules)-1 {
			ruleIndex++
		}
	}
}

func pushWithRetry(ctx context.Context, rdb *redis.Client, ids []string) error {
	var lastErr error
	for i := 0; i < MaxPushRetries; i++ {
		err := rdb.RPush(ctx, RedisKeyIDs, ids).Err()
		if err == nil {
			slog.Info("Successfully pushed batch to Redis", "size", len(ids))
			return nil
		}

		lastErr = err
		slog.Warn("Push failed, retrying...", "attempt", i+1, "error", err)

		// Wait a bit before retrying, but respect shutdown
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

func setupLogger() {
    programLevel := new(slog.LevelVar)
    programLevel.Set(slog.LevelInfo)

    if os.Getenv("APP_DEBUG") == "true" {
        programLevel.Set(slog.LevelDebug)
    }

    h := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: programLevel})
    slog.SetDefault(slog.New(h))
}
