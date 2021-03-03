package redisfetcher

import (
	"context"
	"reflect"
	"time"

	"github.com/go-redis/redis/v8"
)

type TestSampleClientImpl struct {
	client redis.UniversalClient
	ctx    context.Context
}

func (i *TestSampleClientImpl) Set(key string, value interface{}, expiration time.Duration) error {
	return i.client.Set(i.ctx, key, value, expiration).Err()
}

func (i *TestSampleClientImpl) GetSimple(key string) (string, error) {
	return i.client.Get(i.ctx, key).Result()
}

func (i *TestSampleClientImpl) Get(key string, dst interface{}) error {
	v, err := i.GetSimple(key)
	reflect.ValueOf(dst).Elem().SetString(v)
	return err
}

func (i *TestSampleClientImpl) Del(key string) error {
	return i.client.Del(i.ctx, key).Err()
}
