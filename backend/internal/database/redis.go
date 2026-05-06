package database

import (
	"github.com/redis/go-redis/v9"
)

func NewRedisClient(addr string, password string) *redis.Client {
	return  redis.NewClient(&redis.Options{
		Addr: addr,
		Password: password,
	})
}

