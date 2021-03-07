# go-cache-fetcher

This is the function cache fetcher for golang.

For example, The first time, You can set the data to Redis while getting the response of the function.
The second time, If cached, You can get from Redis.


## Installation

```
go get github.com/peutes/go-cache-fetcher@v1.1.3
```

## You can fetch various functions with cache.

You can fetch various functions with cache eg. Redis, Memcached, other cache system, and so on.

### Simple cache control

Simple cache control is that set key and fetch with fetcher function.

The fetcher only needs to use the `SetKey` and `Fetch` functions.
`Fetch` is setted the fetcher function, destination value pointer and cache expiration.  

- `SetKey()`
- `Fetch()`

### Another cache control
If you need a hash key, can use `SetHashKey` instead of `SetKey`.

You can `Set()`, `Get()`, `Del()` individually. If you want key, can use `Key()`. If you want result that is cached, can use `IsCached()`.

- `SetHashKey()`
- `Set()`
- `GetString()`
- `Get()`
- `Del()`
- `Key()`
- `IsCached()`


## Usage

### how to use cachefetcher

```go
f := cachefetcher.NewCacheFetcher(
  &ClientImpl{
    Rdb: redis.NewClient(&redis.Options{Addr: "localhost:6379"}),
  },
  nil
)
f.SetKey([]string{"prefix", "str"}, "hoge")
// f.Key() == "prefix_str_hoge"

// This is fetcher function, For example, read from DB.
fetcher := func(s string) string {
  return s + " fetch!!"
}

// The first fetch is from the fetcher. Not call cache.
var dst string
_, err := f.Fetch(10*time.Second, &dst, func() (string, error) {
  return fetcher("first"), nil
})
// dst == "first fetch!!" <- get from function

// The second fetch is from the expired cache. Not call fetcher.
_, err := f.Fetch(10*time.Second, &dst, func() (string, error) {
  return fetcher("second"), nil
})
// dst == "first fetch!!" <- get from cache

```

Key element support int, float, bool, complex, byte, time, slice, array, "struct with `String()` method" in addition to string.

If the client supports serialization when `Set` and `Get`, Fetcher response is anything interface.
For example, you can set serialize or encode json, Base64 and so on.

```go
f.SetKey([]string{"prefix", "any"}, 1, 0.1, true, &[]string{"a", "b"}, time.Unix(0, 0).In(time.UTC))
_ = f.Key() // "prefix_any_1_0.1_true_a_b_1970-01-01 00:00:00 +0000 UTC"

fetcher := func() ([]int, error) {
  return []int{1, 2, 3, 4, 5}
}
var dst []int  
_, err := f.Fetch(10*time.Second, &dst, fetcher)

```


### implement cache client

This cache fetcher needs cache client implement. The client needs `Set` `Get` `Del` `IsErrCacheMiss` functions.

sample: https://github.com/peutes/go-cache-fetcher/blob/main/cachefetcher/sample_cache_client.go

```go
type ClientImpl struct {
  Rdb *redis.Client
}

func (i *ClientImpl) Set(key string, value interface{}, expiration time.Duration) error {
  // You can serialize or encode json, Base64 and so on.
  return i.Rdb.Set(ctx, key, value, expiration).Err()
}

func (i *ClientImpl) Get(key string, dst interface{}) error {
  // You can deserialize or decode json, Base64 and so on.
  v, err := i.Rdb.Get(ctx, key).Result()
  reflect.ValueOf(dst).Elem().SetString(v)
  return err
}

func (i *ClientImpl) Del(key string) error {
  return i.Rdb.Del(ctx, key).Err()
} 

// return a decision when cache miss err.
func (i *ClientImpl) IsErrCacheMiss(err error) bool {
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
