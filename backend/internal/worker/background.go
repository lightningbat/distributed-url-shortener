package worker

import (
	"backend/internal/queue"
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/redis/go-redis/v9"
)

type Worker struct {
	Redis             *redis.Client
	Queue             *queue.DataQueue
}

const (
	IdleInterval = 100 * time.Millisecond
)

func (w *Worker) Start(ctx context.Context) {
	log := slog.With("module", "worker")
	stockLimit := cap(w.Queue.Pipe)

	log.Info("worker started", "limit", stockLimit)

	for {
		if ctx.Err() != nil {
			log.Info("stopping worker")
			return
		}

		qLen := len(w.Queue.Pipe)
		if qLen >= stockLimit {
			log.Debug("queue full, idling", "len", qLen)
			waitOrExit(ctx, IdleInterval)
			continue
		}

		pullSize := min(stockLimit-qLen, 4000)

		ids, err := w.Redis.LPopCount(ctx, "IDs", pullSize).Result()
		if err != nil {
			if !errors.Is(err, redis.Nil) {
				log.Error("redis lpop failed", "err", err)
			}
			waitOrExit(ctx, IdleInterval)
			continue
		}

		for _, id := range ids {
			select {
			case <-ctx.Done():
				log.Warn("shutdown mid-batch", "dropped", len(ids))
				return
			case w.Queue.Pipe <- id:
			}
		}
	}
}

func waitOrExit(ctx context.Context, d time.Duration) {
	jitter := time.Millisecond * time.Duration(rand.IntN(300))
	select {
	case <-ctx.Done():
	case <-time.After(d + jitter):
	}
}
