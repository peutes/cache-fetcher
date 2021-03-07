package cachefetcher

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"reflect"
	"time"

	"github.com/go-redis/redis/v8"
)

// SampleCacheClientImpl is a sample client implementation.
type SampleCacheClientImpl struct {
	Rdb *redis.Client
	Ctx context.Context
}

// Set is an implementation of the function in the sample client.
func (i *SampleCacheClientImpl) Set(key string, value interface{}, expiration time.Duration) error {
	// You can serialize or encode json, Base64 and so on.
	buf := new(bytes.Buffer)
	if err := gob.NewEncoder(buf).Encode(value); err != nil {
		return err
	}

	// You need an implementation to set from the cache.
	return i.Rdb.Set(i.Ctx, key, buf.String(), expiration).Err()
}

// Get is an implementation of the function in the sample client.
func (i *SampleCacheClientImpl) Get(key string, dst interface{}) error {
	// You need an implementation to get from the cache.
	v, err := i.Rdb.Get(i.Ctx, key).Result()
	if err != nil {
		return err
	}

	// You can deserialize or decode json, Base64 and so on.
	gob.Register(reflect.ValueOf(dst).Elem().Interface())
	buf := bytes.NewBufferString(v)
	return gob.NewDecoder(buf).Decode(dst)
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
