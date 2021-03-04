package cachefetcher

import (
	"context"
	"errors"
	"reflect"
	"time"

	"github.com/go-redis/redis/v8"
)

type SampleCacheClientImpl struct {
	Client redis.UniversalClient
	Ctx    context.Context
}

func (i *SampleCacheClientImpl) Set(key string, value interface{}, expiration time.Duration) error {
	return i.Client.Set(i.Ctx, key, value, expiration).Err()
}

func (i *SampleCacheClientImpl) Get(key string, dst interface{}) error {
	v, err := i.Client.Get(i.Ctx, key).Result()
	reflect.ValueOf(dst).Elem().SetString(v)
	return err
}

func (i *SampleCacheClientImpl) Del(key string) error {
	return i.Client.Del(i.Ctx, key).Err()
}

// return a decision when key exists.
func (i *SampleCacheClientImpl) IsFoundKey(err error) bool {
	return err == nil || !errors.Is(err, redis.Nil)
}
