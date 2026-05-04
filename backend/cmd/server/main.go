package main

import (
	"backend/internal/api"
	"backend/internal/config"
	"backend/internal/persistence"
	"backend/internal/queue"
	"backend/internal/worker"
	"context"
	"log"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	maxRetries = 3
	bufferIdSize = 5000
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	q := queue.New(bufferIdSize)
	rdb := persistence.NewRedisClient(cfg.RedisAddr, cfg.RedisPass)
	dynamoTable, err := persistence.NewDynamodbClient(ctx, cfg.AWSRegion, cfg.DynamoTable)
	if err != nil {
		log.Fatalf("dynamodb connection failed: %v", err)
	}

	w := worker.Worker{Redis: rdb, Queue: q}
	go w.Start(ctx)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: api.RegisterRoutes(q, dynamoTable, rdb),
	}

	go func() {
		slog.Info("http server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http listen error", "err", err)
		}
	}()

	<-ctx.Done()
	slog.Info("shutdown signal received")

	shutdownCtx, cancelSrv := context.WithTimeout(context.Background(), 10*time.Second)
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("srv shutdown err", "err", err)
	}
	cancelSrv()

	close(q.Pipe)

	slog.Info("draining queue")
	flushCtx, cancelFlush := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFlush()

	var ids []string
	for id := range q.Pipe {
		ids = append(ids, id)
	}

	if len(ids) > 0 {
		slog.Info("flushing final batch", "count", len(ids))
		if err := pushWithRetry(flushCtx, rdb, ids); err != nil {
			slog.Error("final push failed", "err", err)
		}
	}

	rdb.Close()
	slog.Info("exit")
}

func pushWithRetry(ctx context.Context, rdb *redis.Client, ids []string) error {
	var lastErr error
	for i := range maxRetries {
		err := rdb.LPush(ctx, "IDs", ids).Err()
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
