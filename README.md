# cache-fetcher

fetcher for golang with cache, eg redis.


## Function

- Main
	- SetKey(prefixes []string, useHash bool, elements ...string)
	- Fetch(expiration time.Duration, dst interface{}, fetcher interface{}) (interface{}, error)

- Other
	- Set(value interface{}, expiration time.Duration) error
	- GetString() (string, error)
	- Get(dst interface{}) (interface{}, error)
	- Del() error
	- Key() string
	- IsCached() bool


## Sample

```
redisClient := redis.NewUniversalClient(
	&redis.UniversalOptions{Addrs: []string{"localhost:6379"}},
)

client := &cachefetcher.SampleCacheClientImpl{
	Client: redisClient,
	Ctx:    context.Background(),
}
  
fetcher := cachefetcher.NewCacheFetcher(client, options)
fetcher.SetKey([]string{"prefix", "key"}, false, "hoge", "fuga")
// cache key is "prefix_key_hoge_fuga"

// First fetch from function.
var dst string  
_, err := f.Fetch(10*time.Second, &dst, func() (string, error) {
	return "first", nil
})
// dst == "first"

// Second fetch from cache eg. Redis. Not call function.
_, err := f.Fetch(10*time.Second, &dst, func() (string, error) {
	return "second", nil
})
// dst == "first"

```

## Needs cache client
This cache fetcher needs cache client implement.

```
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

// return a decision when cache miss err.
func (i *SampleCacheClientImpl) IsErrCacheMiss(err error) bool {
	return errors.Is(err, redis.Nil)
}
```

## Options
This fetcher can use single flight with setting option.

```
cachefetcher.Options{
	Group:          &singleflight.Group{}, // default
	GroupTimeout:   30 * time.Second,      // default
	DebugPrintMode: true,                  // default is false
})
```
