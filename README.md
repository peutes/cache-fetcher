# go-cache-fetcher

This is the function cache fetcher client for golang.


## Installation

```
go get github.com/peutes/go-cache-fetcher
```

## You can fetch various functions with cache.

You can fetch various function responses with cache eg. Redis, Memcached, other cache system, and so on.

For example, The first time, You can set the data to Redis while getting the response of the function.
The second time, If cached, You can get from Redis.


### Simple cache control

Simple cache control is that set key and fetch with fetcher function.

`SetKey` and `Fetch` functions are sufficient for the fetcher client.

`Fetch` needs to setted the fetcher function, destination value pointer and cache expiration. 

- `SetKey()`
- `Fetch()`

### Another cache control
If you need a hash key, can use `SetHashKey` instead of `SetKey`.

You can `Set()`, `Get()`, `Del()` individually. If you want key, can use `Key()`. If you want boolean result that is cached, can use `IsCached()`.

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
fetcher := cachefetcher.NewCacheFetcher(
  &ClientImpl{
    Rdb: redis.NewClient(&redis.Options{Addr: "localhost:6379"}),
  },
  nil
)
fetcher.SetKey([]string{"prefix", "str"}, "hoge")
// fetcher.Key() == "prefix_str_hoge"

// This is fetcher function, For example, assume to read from DB.
read := func(s string) string {
  return s + " fetch!!"
}

// The first fetch is from the fetcher function. Not call cache.
var dst string
err := fetcher.Fetch(10*time.Second, &dst, func() (string, error) {
  return read("first"), nil
})
// dst == "first fetch!!" <- get from function

// The second fetch is from the expired cache. Not call fetcher function.
err = fetcher.Fetch(10*time.Second, &dst, func() (string, error) {
  return read("second"), nil
})
// dst == "first fetch!!" <- get from cache

```

Key element support int, float, bool, complex, byte, time, slice, array, "struct with `String()` method" in addition to string.

The client supports serialization with gob serializer.
The cache saves serialized strings.


```go
fetcher.SetKey([]string{"prefix", "any"}, 1, 0.1, true, &[]string{"a", "b"}, time.Unix(0, 0).In(time.UTC))
_ = fetcher.Key() // "prefix_any_1_0.1_true_a_b_1970-01-01_00:00:00_+0000_UTC"
_ = fetcher.HashKey() // "prefix_any_c94a415eb6e20585f4fbc856b6edcf52007259522967c4bea548515e71531663"

read := func() ([]int, error) {
  return []int{1, 2, 3, 4, 5}
}
var dst []int  
err := fetcher.Fetch(10*time.Second, &dst, read)
// dst == []int{1, 2, 3, 4, 5}

```

This client supports more than just string type. If you want use interface{} or another unique type, use `GobRegister()` to register type.

```go
    i := 10
    b := true
    s := "abc"
    ft := 0.123
    i8 := int8(20)
    i64 := int64(30)
    ui8 := uint8(40)
    ui64 := uint64(50)

    e := &testStruct{
        I:     i,
        B:     b,
        S:     s,
        F:     ft,
        I8:    i8,
        I64:   i64,
        UI8:   ui8,
        UI64:  ui64,
        IP:    &i,
        BP:    &b,
        SP:    &s,
        FP:    &ft,
        I8P:   &i8,
        I64P:  &i64,
        UI8P:  &ui8,
        UI64P: &ui64,
        IS:    []int{i, i, i},
        BS:    []bool{b, b, b},
        SS:    []string{s, s, s},
        FS:    []float64{ft, ft, ft},
        IM:    map[int]int{1: i, 2: i, 3: i},
        BM:    map[bool]bool{true: b, false: b},
        SM:    map[string]string{"a": s, "bb": s, "ccc": s},
        FM:    map[float64]float64{0.1: ft, 0.2: ft, 0.3: ft},
    }

    var dst testStruct
    f := cachefetcher.NewCacheFetcher(redisClient, options)
    err := fetcher.SetKey([]string{"prefix", "key"}, "struct1")
    err = fetcher.Set(e, 10*time.Second)
    err = fetcher.Get(&dst)
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

This fetcher client can use single flight with setting option.

If `DebugPrintMode` set true, the cache key will be printed to the terminal.

```go
cachefetcher.Options{
    Group:          &singleflight.Group{}, // default
    GroupTimeout:   30 * time.Second,      // default
    DebugPrintMode: true,                  // default is false
})
```
