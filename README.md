# go-cache-fetcher

This is the function cache fetcher for golang.

For example, The first time, You can set the data to Redis while getting the response of the function.  
The second time, If cached, You can get from Redis.


## Cache Control Functioon

### Simple cache control

Simple cache control is that set key and fetch with fetcher function.

The fetcher only needs to use the `SetKey` and `Fetch` functions.  
`Fetch` is setted the fetcher function, destination value pointer and cache expiration.  

- `SetKey(prefixes []string, elements ...string)`
- `Fetch(expiration time.Duration, dst interface{}, fetcher interface{}) (interface{}, error)`

### Another cache control
If you need a hash key, can use `SetHashKey` instead of `SetKey`.

You can `Set()`, `Get()`, `Del()` individually. If you want key, can use `Key()`. If you want result that is cached, can use `IsCached()`.

- `SetHashKey(prefixes []string, elements ...string)`
- `Set(value interface{}, expiration time.Duration) error`
- `GetString() (string, error)`
- `Get(dst interface{}) (interface{}, error)`
- `Del() error`
- `Key() string`
- `IsCached() bool`


## How to use

### how to use cachefetcher

```go
client := &cachefetcher.SampleCacheClientImpl{
	Client: redis.NewUniversalClient(
		&redis.UniversalOptions{Addrs: []string{"localhost:6379"}},
	),
}
  
fetcher := cachefetcher.NewCacheFetcher(client, options)
fetcher.SetKey([]string{"prefix", "str"}, false, "hoge")

// cache key is "prefix_str_hoge"

// First fetch from function.

var dst string  
_, err := f.Fetch(10*time.Second, &dst, func() (string, error) {
	return "first", nil
})
// dst == "first" <- get from function

// Second fetch from cache eg. Redis. Not call function.
_, err := f.Fetch(10*time.Second, &dst, func() (string, error) {
	return "second", nil
})
// dst == "first" <- get from cache

```

If the client supports serialization when `Set` and `Get`, Fetcher response is anything interface.  
For example, you can set serialize or encode json, Base64 and so on.

```
fetcher.SetKey([]string{"prefix", "int"}, false, "1")

var dst string  
_, err := f.Fetch(10*time.Second, &dst, func() ([]int, error) {
	return []int{1,2,3,4,5}, nil
})
// dst == "first" <- get from function

```


### implement cache client

This cache fetcher needs cache client implement.  
The client needs `Set` `Get` `Del` `IsErrCacheMiss` functions.

```go
var ctx = context.Background()
type SampleCacheClientImpl struct {
	Client redis.UniversalClient
}

func (i *SampleCacheClientImpl) Set(key string, value interface{}, expiration time.Duration) error {
	// You can serialize or encode json, Base64 and so on.
	return i.Client.Set(ctx, key, value, expiration).Err()
}

func (i *SampleCacheClientImpl) Get(key string, dst interface{}) error {
	// You can deserialize or decode json, Base64 and so on.
	v, err := i.Client.Get(ctx, key).Result()
	reflect.ValueOf(dst).Elem().SetString(v)
	return err
}

func (i *SampleCacheClientImpl) Del(key string) error {
	return i.Client.Del(ctx, key).Err()
}

// return a decision when cache miss err.
func (i *SampleCacheClientImpl) IsErrCacheMiss(err error) bool {
	return errors.Is(err, redis.Nil)
}
```

### Options
This fetcher can use single flight with setting option.  
If `DebugPrintMode` set true, the cache key will be printed to the terminal.

```go
cachefetcher.Options{
	Group:          &singleflight.Group{}, // default
	GroupTimeout:   30 * time.Second,      // default
	DebugPrintMode: true,                  // default is false
})
```
