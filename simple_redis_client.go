package cachefetcher

import (
	"context"
	"errors"
	"reflect"
	"time"

	"github.com/go-redis/redis/v8"
)

var ctx = context.Background()

// SimpleRedisClientImpl is a sample redisClient implementation.
type SimpleRedisClientImpl struct {
	Rdb *redis.Client
}

// Set is an implementation of the function in the sample redisClient.
func (i *SimpleRedisClientImpl) Set(key string, value interface{}, expiration time.Duration) error {
	// You need an implementation to set from the cache.
	return i.Rdb.Set(ctx, key, value, expiration).Err()
}

// Get is an implementation of the function in the sample redisClient.
func (i *SimpleRedisClientImpl) Get(key string, dst interface{}) error {
	// You need an implementation to get from the cache.
	v, err := i.Rdb.Get(ctx, key).Result()
	if err != nil {
		return err
	}

	reflect.ValueOf(dst).Elem().SetString(v)
	return nil
}

// Del is an implementation of the function in the sample redisClient.
func (i *SimpleRedisClientImpl) Del(key string) error {
	return i.Rdb.Del(ctx, key).Err()
}

// IsErrCacheMiss is an implementation of the function in the sample redisClient.
// Please return the decision at the time of cache miss err.
func (i *SimpleRedisClientImpl) IsErrCacheMiss(err error) bool {
	return errors.Is(err, redis.Nil)
}
