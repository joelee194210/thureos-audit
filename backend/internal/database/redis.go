package database

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
)

func ConnectRedis(url string) (*redis.Client, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opts)

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}

	log.Println("Connected to Redis")
	return client, nil
}
