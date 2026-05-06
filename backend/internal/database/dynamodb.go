package database

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/guregu/dynamo/v2"
)


func NewDynamodbClient(ctx context.Context, region string, tableName string) (*dynamo.Table, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return  nil, err
	}
	db := dynamo.New(cfg)
	table := db.Table(tableName)
	return &table, nil
}
