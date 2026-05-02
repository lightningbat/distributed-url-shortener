package config

import (
	"fmt"
	"os"
	"reflect"
)

type Config struct {
	RedisAddr   string `env:"REDIS_ADDR"`
	RedisPass   string `env:"REDIS_PASS"`
	AWSRegion   string `env:"AWS_REGION"`
	DynamoTable string `env:"DYNAMO_TABLE"`
}

func LoadConfig() (*Config, error) {
	cfg := &Config{}
	
	v := reflect.ValueOf(cfg).Elem()
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		tag := t.Field(i).Tag.Get("env")

		if tag == "" {
			continue
		}

		value, exists := os.LookupEnv(tag)
		if !exists {
			return nil, fmt.Errorf("missing required environment variable: %s", tag)
		}

		field.SetString(value)
	}

	return cfg, nil
}
