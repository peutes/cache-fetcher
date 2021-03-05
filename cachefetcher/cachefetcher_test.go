package cachefetcher_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/peutes/go-cache-fetcher/cachefetcher"
)

const host = "localhost:6379"

var (
	options = &cachefetcher.Options{DebugPrintMode: true}
	client  *cachefetcher.SampleCacheClientImpl
	ctx     = context.Background()
)

// nolint: staticcheck
func TestMain(m *testing.M) {
	c := redis.NewUniversalClient(
		&redis.UniversalOptions{Addrs: []string{host}},
	)
	c.FlushDB(ctx)

	client = &cachefetcher.SampleCacheClientImpl{
		Client: c,
		Ctx:    ctx,
	}
	m.Run()
}

func before() {
	client.Client.FlushDB(ctx)
}

func TestClient(t *testing.T) {
	before()

	// nolint: goconst
	want := "value"
	if err := client.Set("key", want, 0); err != nil {
		t.Error(err)
	}

	var val string
	err := client.Get("key", &val)
	if err != nil {
		t.Error(err)
	}
	if val != want {
		t.Errorf("failed: %+v", val)
	}

	err = client.Get("key2", &val)
	if err != nil && !client.IsErrCacheMiss(err) {
		t.Errorf("failed: %+v, %+v", val, err)
	}
}

func TestSetKey(t *testing.T) {
	before()

	f := cachefetcher.NewCacheFetcher(client, options)
	f.SetKey([]string{"prefix", "key"}, false, "hoge", "fuga")
	key := f.Key()

	want := "prefix_key_hoge_fuga"
	if key != want {
		t.Errorf("%+v", key)
	}
}

func TestSetKeyWithHash(t *testing.T) {
	before()

	f := cachefetcher.NewCacheFetcher(client, options)
	f.SetKey([]string{"prefix", "key"}, true, "hoge", "fugadddddddd")
	key := f.Key()

	want := "prefix_key_a31d03600d04dd35fc74f8489c9347d154074699ddb37ca893f3a0a9e20ac09d"
	if key != want {
		t.Errorf("%+v", key)
	}
}

func TestFetch(t *testing.T) {
	before()

	f := cachefetcher.NewCacheFetcher(client, options)
	f.SetKey([]string{"prefix", "key"}, false, "hoge", "fuga")

	// first fetch read from fetcher.
	var dst string
	want := "piyo"
	dst2, err := f.Fetch(10*time.Second, &dst, func() (string, error) {
		return want, nil
	})
	if err != nil {
		t.Errorf("%+v", err)
	}

	if f.IsCached() {
		t.Errorf("%+v", f.IsCached())
	}

	if dst != want || dst2 != want {
		t.Errorf("%+v", dst2)
	}

	// second fetch read from cache.
	dst3, err := f.Fetch(10*time.Second, &dst, func() (string, error) {
		return want, nil
	})
	if err != nil {
		t.Errorf("%+v", err)
	}

	if !f.IsCached() {
		t.Errorf("%+v", f.IsCached())
	}

	if dst != want || dst3 != want {
		t.Errorf("%+v, %+v", dst, dst3)
	}
}

func TestSet(t *testing.T) {
	before()

	f := cachefetcher.NewCacheFetcher(client, options)
	f.SetKey([]string{"prefix", "key"}, false, "hoge", "fuga")
	if err := f.Set("value", 10*time.Second); err != nil {
		t.Errorf("%+v", err)
	}

	if !f.IsCached() {
		t.Errorf("%+v", f.IsCached())
	}
}

func TestGetString(t *testing.T) {
	before()

	f := cachefetcher.NewCacheFetcher(client, options)
	f.SetKey([]string{"prefix", "key"}, true, "hoge", "fuga")
	want := "value"
	if err := f.Set(want, 10*time.Second); err != nil {
		t.Errorf("%+v", err)
	}

	dst, err := f.GetString()
	if err != nil {
		t.Errorf("%+v", err)
	}

	if !f.IsCached() {
		t.Errorf("%+v", f.IsCached())
	}

	if dst != want {
		t.Errorf("%+v", dst)
	}
}

func TestGet(t *testing.T) {
	before()

	f := cachefetcher.NewCacheFetcher(client, options)
	f.SetKey([]string{"prefix", "key"}, true, "hoge", "fuga")
	want := "value"
	if err := f.Set(want, 10*time.Second); err != nil {
		t.Errorf("%+v", err)
	}

	var dst string
	dst2, err := f.Get(&dst)
	if err != nil {
		t.Errorf("%+v", err)
	}

	if !f.IsCached() {
		t.Errorf("%+v", f.IsCached())
	}

	if dst != want || dst2 != want {
		t.Errorf("%+v, %+v", dst, dst2)
	}
}

func TestDel(t *testing.T) {
	before()

	f := cachefetcher.NewCacheFetcher(client, options)
	f.SetKey([]string{"prefix", "key"}, false, "hoge", "fuga")
	if err := f.Set("value", 10*time.Second); err != nil {
		t.Errorf("%+v", err)
	}

	if err := f.Del(); err != nil {
		t.Errorf("%+v", err)
	}
	if !f.IsCached() {
		t.Errorf("%+v", f.IsCached())
	}

	var dst string
	dst2, err := f.Get(&dst)
	if err != nil && !errors.Is(err, redis.Nil) {
		t.Errorf("%+v", err)
	}
	if dst != "" || dst2 != nil {
		t.Errorf("%+v, %+v", dst, dst2)
	}
}
