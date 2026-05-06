package queue

import (
	"context"
	"log/slog"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
)

type DataQueue struct {
	Pipe          chan string
	persistentRdb *redis.Client
	maxRetries    int
	RedisKeyIDs   string
}

func New(size int, rdb *redis.Client, retries int, redisKeyIDs string) *DataQueue {
	return &DataQueue{
		Pipe:          make(chan string, size),
		persistentRdb: rdb,
		maxRetries:    retries,
		RedisKeyIDs: redisKeyIDs,
	}
}

func (dq *DataQueue) Flush() {
	close(dq.Pipe)
	var ids []string
	for id := range dq.Pipe {
		ids = append(ids, id)
	}

	flushCtx, cancelFlush := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFlush()

	if len(ids) > 0 {
		slog.Info("flushing final batch", "count", len(ids))
		if err := pushWithRetry(flushCtx, dq.persistentRdb, dq.RedisKeyIDs, ids, dq.maxRetries); err != nil {
			slog.Error("final push failed", "err", err)
		}
	}
}

func pushWithRetry(
	ctx context.Context,
	rdb *redis.Client,
	redisKeyIDs string,
	ids []string,
	maxRetries int,
) error {
	var lastErr error
	for i := range maxRetries {
		err := rdb.LPush(ctx, redisKeyIDs, ids).Err()
		if err == nil {
			slog.Info("flush success", "count", len(ids))
			return nil
		}

		lastErr = err
		slog.Warn("push retry", "attempt", i+1, "err", err)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second*time.Duration(i+1) + time.Duration(rand.Intn(300))*time.Millisecond):
		}
	}
	return lastErr
}
