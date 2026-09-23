package store

import (
	"backend/internal/config"
	"context"
	"log/slog"
)

type Store interface {
	PutUrl(ctx context.Context, id string, url string) bool
	GetUrl(ctx context.Context, id string) (string, bool)
}

func New(
	ctx context.Context,
	dynamoDBcfg *config.DynamodbConfig,
	postgresCfg *config.PostgresConfig,
) (Store, error) {
	if (dynamoDBcfg.Region != "" && dynamoDBcfg.TableName != "") {
		slog.Info("dynamoDBcfg is not empty")
		return newDynamoDBStore(ctx, dynamoDBcfg)
	}

	return newPostgresStore(ctx, postgresCfg)
}
