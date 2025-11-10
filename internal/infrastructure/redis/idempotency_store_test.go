package redisstore_test

import (
	"context"
	"testing"
	"time"

	redisstore "fxrates-service/internal/infrastructure/redis"
	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestTryReserve(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := redisstore.New(client, time.Hour)

	ctx := context.Background()
	ok, err := store.TryReserve(ctx, "k1")
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = store.TryReserve(ctx, "k1")
	require.NoError(t, err)
	require.False(t, ok)
}
