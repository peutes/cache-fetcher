package cachefetcher

import (
	"context"
	"errors"
	"reflect"
	"time"

	"github.com/go-redis/redis/v8"
)

type SampleCacheClientImpl struct {
	Rdb *redis.Client
	Ctx context.Context
}

// Set is an implementation of the function in the sample client.
func (i *SampleCacheClientImpl) Set(key string, value interface{}, expiration time.Duration) error {
	// You can serialize or encode json, Base64 and so on.
	return i.Rdb.Set(i.Ctx, key, value, expiration).Err()
}

// Get is an implementation of the function in the sample client.
func (i *SampleCacheClientImpl) Get(key string, dst interface{}) error {
	// You can deserialize or decode json, Base64 and so on.
	v, err := i.Rdb.Get(i.Ctx, key).Result()
	reflect.ValueOf(dst).Elem().SetString(v)
	return err
}

// Del is an implementation of the function in the sample client.
func (i *SampleCacheClientImpl) Del(key string) error {
	return i.Rdb.Del(i.Ctx, key).Err()
}

// IsErrCacheMiss is an implementation of the function in the sample client.
// Please return the decision at the time of cache miss err.
func (i *SampleCacheClientImpl) IsErrCacheMiss(err error) bool {
	return errors.Is(err, redis.Nil)
}
