# go-cache-fetcher

This is the function cache fetcher for golang.


## Installation

```
go get github.com/peutes/go-cache-fetcher
```

## You can fetch various functions with cache.

You can fetch various functions with cache eg. Redis, Memcached, other cache system, and so on.

For example, The first time, You can set the data to Redis while getting the response of the function.
The second time, If cached, You can get from Redis.


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
- `Get()`
- `SetString()`
- `GetString()`
- `Del()`
- `Key()`
- `IsCached()`
- `GobRegister()`

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
err := f.Fetch(10*time.Second, &dst, func() (string, error) {
  return fetcher("first"), nil
})
// dst == "first fetch!!" <- get from function

// The second fetch is from the expired cache. Not call fetcher.
err := f.Fetch(10*time.Second, &dst, func() (string, error) {
  return fetcher("second"), nil
})
// dst == "first fetch!!" <- get from cache

```

Key element support int, float, bool, complex, byte, time, slice, array, "struct with `String()` method" in addition to string.

The client supports serialization with gob serializer.

```go
f.SetKey([]string{"prefix", "any"}, 1, 0.1, true, &[]string{"a", "b"}, time.Unix(0, 0).In(time.UTC))
_ = f.Key() // "prefix_any_1_0.1_true_a_b_1970-01-01_00:00:00_+0000_UTC"
_ = f.HashKey() // "prefix_any_c94a415eb6e20585f4fbc856b6edcf52007259522967c4bea548515e71531663"

fetcher := func() ([]int, error) {
  return []int{1, 2, 3, 4, 5}
}
var dst []int  
err := f.Fetch(10*time.Second, &dst, fetcher)
// dst == []int{1, 2, 3, 4, 5}

```


### implement cache client

This cache fetcher needs cache client implement. The client needs `Set` `Get` `Del` `IsErrCacheMiss` functions.

The simple redis client is https://github.com/peutes/go-cache-fetcher/blob/main/cachefetcher/simple_redis_client.go

```go
// SimpleRedisClientImpl is a sample client implementation.
type SimpleRedisClientImpl struct {
    Rdb *redis.Client
}

// Set is an implementation of the function in the sample client.
func (i *SimpleRedisClientImpl) Set(key string, value interface{}, expiration time.Duration) error {
    // You need an implementation to set from the cache.
    return i.Rdb.Set(ctx, key, value, expiration).Err()
}

// Get is an implementation of the function in the sample client.
func (i *SimpleRedisClientImpl) Get(key string, dst interface{}) error {
    // You need an implementation to get from the cache.
    v, err := i.Rdb.Get(ctx, key).Result()
    if err != nil {
        return err
    }

    reflect.ValueOf(dst).Elem().SetString(v)
    return nil
}

// Del is an implementation of the function in the sample client.
func (i *SimpleRedisClientImpl) Del(key string) error {
    return i.Rdb.Del(ctx, key).Err()
}

// IsErrCacheMiss is an implementation of the function in the sample client.
// Please return the decision at the time of cache miss err.
func (i *SimpleRedisClientImpl) IsErrCacheMiss(err error) bool {
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
