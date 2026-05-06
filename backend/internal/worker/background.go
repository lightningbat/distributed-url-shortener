package worker

import (
	"backend/internal/config"
	"backend/internal/queue"
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/redis/go-redis/v9"
)

type Worker struct {
	Redis       *redis.Client
	Queue       *queue.DataQueue
	Cfg         *config.WorkerOptions
	RedisKeyIDs string
}

func (w *Worker) Start(ctx context.Context) {
	log := slog.Default().WithGroup("worker")
	bufferIDSize := cap(w.Queue.Pipe)

	log.Info("worker started", "limit", bufferIDSize)

	for {
		if ctx.Err() != nil {
			log.Info("stopping worker")
			return
		}

		qLen := len(w.Queue.Pipe)
		if qLen >= bufferIDSize {
			log.Debug("queue full, idling", "len", qLen)
			waitOrExit(ctx, w.Cfg.IdleInterval)
			continue
		}

		pullSize := min(bufferIDSize-qLen, w.Cfg.PullLimit)

		ids, err := w.Redis.LPopCount(ctx, w.RedisKeyIDs, pullSize).Result()
		if err != nil {
			if !errors.Is(err, redis.Nil) {
				log.Error("redis lpop failed", "err", err)
			} else {
				log.Debug("worker idle: no tasks found in queue",
					"key", w.RedisKeyIDs)
			}
			waitOrExit(ctx, w.Cfg.IdleInterval)
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
