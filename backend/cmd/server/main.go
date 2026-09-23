package main

import (
	"backend/internal/api"
	"backend/internal/config"
	"backend/internal/database/store"
	"backend/internal/database/redis"
	"backend/internal/queue"
	"backend/internal/worker"
	"context"
	"flag"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}

	setupLogger(cfg.Server.Debug)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	persistentRdb := redis.NewRedisClient(cfg.PersistentRedis.Addr, cfg.PersistentRedis.Password)
	defer persistentRdb.Close()
	volatileRdb := redis.NewRedisClient(cfg.VolatileRedis.Addr, cfg.VolatileRedis.Password)
	defer volatileRdb.Close()

	queue := queue.New(
		cfg.Buffer.BufferIDSize,
		persistentRdb,
		cfg.Buffer.MaxRetries,
		cfg.PersistentRedis.RedisKeyIDs,
	)

	database, err := store.New(ctx, &cfg.DynamoDB, &cfg.Postgres)
	if err != nil {
		log.Fatalf("database initialization failed: %v", err)
	}

	w := worker.Worker{
		Redis: persistentRdb,
		Queue: queue,
		Cfg: &cfg.Worker,
		RedisKeyIDs: cfg.PersistentRedis.RedisKeyIDs,
	}
	go w.Start(ctx)

	srv := &http.Server{
		Addr:    cfg.Server.Port,
		Handler: api.RegisterRoutes(queue, database, volatileRdb, &cfg.RateLimit, &cfg.Cache),
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

	slog.Info("draining queue")
	queue.Flush()

	slog.Info("exit")
}

func setupLogger(debug bool) {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	h := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(h))
}
