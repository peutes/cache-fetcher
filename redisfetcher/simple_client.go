package redisfetcher

import (
	"context"
	"reflect"
	"time"

	"github.com/go-redis/redis/v8"
)

type SimpleClientImpl struct {
	Client redis.UniversalClient
	Ctx    context.Context
}

func (i *SimpleClientImpl) Set(key string, value interface{}, expiration time.Duration) error {
	return i.Client.Set(i.Ctx, key, value, expiration).Err()
}

func (i *SimpleClientImpl) GetString(key string) (string, error) {
	return i.Client.Get(i.Ctx, key).Result()
}

func (i *SimpleClientImpl) Get(key string, dst interface{}) error {
	v, err := i.GetString(key)
	reflect.ValueOf(dst).Elem().SetString(v)
	return err
}

func (i *SimpleClientImpl) Del(key string) error {
	return i.Client.Del(i.Ctx, key).Err()
}
