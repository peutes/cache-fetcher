package redisfetcher

import (
	"context"
	"reflect"
	"time"

	"github.com/go-redis/redis/v8"
)

type TestSampleClientImpl struct {
	Client redis.UniversalClient
	Ctx    context.Context
}

func (i *TestSampleClientImpl) Set(key string, value interface{}, expiration time.Duration) error {
	return i.Client.Set(i.Ctx, key, value, expiration).Err()
}

func (i *TestSampleClientImpl) GetString(key string) (string, error) {
	return i.Client.Get(i.Ctx, key).Result()
}

func (i *TestSampleClientImpl) Get(key string, dst interface{}) error {
	v, err := i.GetString(key)
	reflect.ValueOf(dst).Elem().SetString(v)
	return err
}

func (i *TestSampleClientImpl) Del(key string) error {
	return i.Client.Del(i.Ctx, key).Err()
}
