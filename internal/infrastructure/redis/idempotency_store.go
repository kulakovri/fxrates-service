package redisstore

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Store struct {
	Client *redis.Client
	TTL    time.Duration
}

func New(client *redis.Client, ttl time.Duration) *Store {
	return &Store{Client: client, TTL: ttl}
}

func (s *Store) TryReserve(ctx context.Context, key string) (bool, error) {
	ok, err := s.Client.SetNX(ctx, key, "1", s.TTL).Result()
	if err != nil {
		return false, err
	}
	return ok, nil
}
