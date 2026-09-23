package store

import (
	"backend/internal/config"
	"context"
	"errors"
	"log/slog"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/guregu/dynamo/v2"
)

type dynamoDBStore struct {
	table       *dynamo.Table
	dynamoDBcfg *config.DynamodbConfig
}

func newDynamoDBStore(ctx context.Context, dynamoDBcfg *config.DynamodbConfig) (*dynamoDBStore, error) {
	awscfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion(dynamoDBcfg.Region),
	)
	if err != nil {
		return nil, err
	}

	db := dynamo.New(awscfg)
	table := db.Table(dynamoDBcfg.TableName)

	if _, err := table.Describe().Run(ctx); err != nil {
		slog.Error("failed to connect to dynamodb or validate credentials", "err", err)
		return nil, err
    }

	return &dynamoDBStore{
		table: &table,
	}, nil
}

func (d *dynamoDBStore) PutUrl(
	ctx context.Context,
	id string,
	url string,
) bool {
	mapping := map[string]any{
		d.dynamoDBcfg.Columns.ID: id,
		d.dynamoDBcfg.Columns.URL: url,
	}

	if err := d.table.Put(mapping).Run(ctx); err != nil {
		slog.Error("dynamo put failed", "err", err)
		return false
	}

	return true
}

func (d *dynamoDBStore) GetUrl(
	ctx context.Context,
	id string,
) (string, bool) {
	var originalURL string

	err := d.table.Get(d.dynamoDBcfg.Columns.ID, id).
		Project(d.dynamoDBcfg.Columns.URL).
		One(ctx, &originalURL)

	if err != nil {
		if errors.Is(err, dynamo.ErrNotFound) {
			return "", true
		}
		slog.Error("dynamo get failed", "err", err)
		return "", false
	}

	return originalURL, true
}
