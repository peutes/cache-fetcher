package cachefetcher_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/peutes/go-cache-fetcher/cachefetcher"
)

const host = "localhost:6379"

var (
	options  = &cachefetcher.Options{DebugPrintMode: true}
	client   *cachefetcher.SampleCacheClientImpl
	ctx      = context.Background()
	timeType = time.Unix(0, 0).In(time.UTC)
)

type (
	unique      string
	testStruct1 struct{}
	testStruct2 struct {
		A int
		B string
		C []bool
	}
)

func (testStruct1) String() string {
	return "testStruct1"
}

// nolint: staticcheck
func TestMain(m *testing.M) {
	client = &cachefetcher.SampleCacheClientImpl{
		Rdb: redis.NewClient(&redis.Options{Addr: host}),
		Ctx: ctx,
	}
	m.Run()
}

func before() {
	client.Rdb.FlushDB(ctx)
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
		t.Errorf("%#v is not %#v", val, want)
	}

	err = client.Get("key2", &val)
	if err != nil && !client.IsErrCacheMiss(err) {
		t.Errorf("failed: %#v, %#v", val, err)
	}
}

func Test_SetKey(t *testing.T) {
	before()

	b0 := true
	b1 := false
	i0 := 0
	i1 := uint(1)
	i2 := uint64(2)
	i3 := uintptr(3)
	c := complex(1.1, 1.2)
	f0 := float32(0.1)
	f1 := float64(0.2)
	s := "abc"
	b := byte(10)
	u := unique("u")

	sl := []bool{b0, b1}
	bl := []byte(s)
	arr := [2]bool{b0, b1}
	m := map[interface{}]interface{}{b0: b0, i0: i0, c: c, f0: f0, s: s}

	ts1 := &testStruct1{}
	ts2 := &testStruct2{}

	fc := func() bool { return b0 }
	ch := make(chan int)

	type args struct {
		prefixes []string
		elements []interface{}
	}

	tests := []struct {
		name string
		args args
		want string
		err  error
	}{
		{"prefix", args{[]string{"prefix", "key"}, nil}, "prefix_key", nil},
		{"space", args{[]string{"prefix", " k e y "}, nil}, "prefix__k_e_y_", nil},
		{"string", args{[]string{"prefix", "key"}, []interface{}{"hoge", "fuga"}}, "prefix_key_hoge_fuga", nil},

		{
			"anything1",
			args{
				[]string{"prefix", "key"},
				[]interface{}{b0, b1, i0, i1, i2, i3, c, f0, f1, s, b, u},
			},
			"prefix_key_true_false_0_1_2_3_(1.1+1.2i)_0.1_0.2_abc_10_u",
			nil,
		},
		{
			"pointer",
			args{
				[]string{"prefix", "key"},
				[]interface{}{&b0, &b1, &i0, &i1, &i2, &i3, &c, &f0, &f1, &s, &b, &u},
			},
			"prefix_key_true_false_0_1_2_3_(1.1+1.2i)_0.1_0.2_abc_10_u",
			nil,
		},
		{
			"slice array",
			args{
				[]string{"prefix", "key"},
				[]interface{}{sl, arr, bl},
			},
			"prefix_key_true_false_true_false_97_98_99",
			nil,
		},
		{
			"struct",
			args{
				[]string{"prefix", "key"},
				[]interface{}{ts1, timeType},
			},
			"prefix_key_testStruct1_1970-01-01_00:00:00_+0000_UTC",
			nil,
		},
		{
			"README",
			args{
				[]string{"prefix", "any"},
				[]interface{}{1, 0.1, true, &[]string{"a", "b"}, time.Unix(0, 0).In(time.UTC)},
			},
			"prefix_any_1_0.1_true_a_b_1970-01-01_00:00:00_+0000_UTC",
			nil,
		},

		// invalid
		{"nil", args{[]string{"prefix", "key"}, []interface{}{nil}}, "", cachefetcher.ErrInvalidKeyElements},
		{"map", args{[]string{"prefix", "key"}, []interface{}{m}}, "", cachefetcher.ErrInvalidKeyElements},
		{"struct2", args{[]string{"prefix", "key"}, []interface{}{ts2}}, "", cachefetcher.ErrInvalidKeyElements},
		{"func", args{[]string{"prefix", "key"}, []interface{}{fc}}, "", cachefetcher.ErrInvalidKeyElements},
		{"chan", args{[]string{"prefix", "key"}, []interface{}{ch}}, "", cachefetcher.ErrInvalidKeyElements},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := cachefetcher.NewCacheFetcher(client, options)
			if err := f.SetKey(tt.args.prefixes, tt.args.elements...); !errors.Is(err, tt.err) {
				t.Errorf("%#v, %#v", tt.name, err)
			}

			key := f.Key()
			if key != tt.want {
				t.Errorf("%#v: %#v is not %#v", tt.name, key, tt.want)
			}
		})
	}
}

func TestSetKeyWithHash(t *testing.T) {
	before()

	type args struct {
		prefixes []string
		elements []interface{}
	}

	tests := []struct {
		name string
		args args
		want string
		err  error
	}{
		{
			"prefix",
			args{[]string{"prefix", "key"},
				nil,
			},
			"prefix_key",
			nil,
		},
		{
			"space",
			args{
				[]string{"prefix", " k e y "},
				nil,
			},
			"prefix__k_e_y_",
			nil,
		},
		{
			"strings",
			args{
				[]string{"prefix", "key"},
				[]interface{}{"hoge", "fugadddddddd"},
			},
			"prefix_key_a31d03600d04dd35fc74f8489c9347d154074699ddb37ca893f3a0a9e20ac09d",
			nil,
		},
		{
			"README",
			args{
				[]string{"prefix", "any"},
				[]interface{}{1, 0.1, true, &[]string{"a", "b"}, time.Unix(0, 0).In(time.UTC)},
			},
			"prefix_any_c94a415eb6e20585f4fbc856b6edcf52007259522967c4bea548515e71531663",
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := cachefetcher.NewCacheFetcher(client, options)
			if err := f.SetHashKey(tt.args.prefixes, tt.args.elements...); !errors.Is(err, tt.err) {
				t.Errorf("%#v, %#v", tt.name, err)
			}

			key := f.Key()
			if key != tt.want {
				t.Errorf("%#v: %#v is not %#v", tt.name, key, tt.want)
			}
		})
	}
}

func TestFetch(t *testing.T) {
	before()

	f := cachefetcher.NewCacheFetcher(client, options)
	if err := f.SetKey([]string{"prefix", "key ptr"}, "hoge", "fuga"); err != nil {
		t.Errorf("%#v", err)
	}

	// first fetch read from fetcher.
	var dst string
	want := "piyo"
	if err := f.Fetch(10*time.Second, &dst, func() (string, error) {
		return want, nil
	}); err != nil {
		t.Errorf("%#v", err)
	}

	if f.IsCached() {
		t.Errorf("%#v", f.IsCached())
	}

	if dst != want {
		t.Errorf("%#v is not %#v", dst, want)
	}

	// second fetch read from cache.
	if err := f.Fetch(10*time.Second, &dst, func() (string, error) {
		return want, nil
	}); err != nil {
		t.Errorf("%#v", err)
	}

	if !f.IsCached() {
		t.Errorf("%#v", f.IsCached())
	}

	if dst != want {
		t.Errorf("%#v is not %#v", dst, want)
	}
}

func TestSet(t *testing.T) {
	before()

	f := cachefetcher.NewCacheFetcher(client, options)
	if err := f.SetKey([]string{"prefix", "key"}, "hoge", "fuga"); err != nil {
		t.Errorf("%#v", err)
	}

	if err := f.Set("value", 10*time.Second); err != nil {
		t.Errorf("%#v", err)
	}

	if !f.IsCached() {
		t.Errorf("%#v", f.IsCached())
	}
}

func TestGetString(t *testing.T) {
	before()

	want := "value"
	f := cachefetcher.NewCacheFetcher(client, options)
	if err := f.SetHashKey([]string{"prefix", "key"}, want); err != nil {
		t.Errorf("%#v", err)
	}

	if err := f.Set(want, 10*time.Second); err != nil {
		t.Errorf("%#v", err)
	}

	dst, err := f.GetString()
	if err != nil {
		t.Errorf("%#v", err)
	}

	if !f.IsCached() {
		t.Errorf("%#v", f.IsCached())
	}

	if dst != want {
		t.Errorf("%#v, is not %#v", dst, want)
	}
}

func TestGetInt(t *testing.T) {
	before()

	e := 100
	var dst int

	f := cachefetcher.NewCacheFetcher(client, options)
	if err := f.SetKey([]string{"prefix", "key"}, e); err != nil {
		t.Errorf("%#v", err)
	}

	if err := f.Set(e, 10*time.Second); err != nil {
		t.Errorf("%#v", err)
	}

	err := f.Get(&dst)
	if err != nil {
		t.Errorf("%#v", err)
	}

	if !f.IsCached() {
		t.Errorf("%#v", f.IsCached())
	}

	if dst != e {
		t.Errorf("%#v is not %#v", dst, e)
	}
}

func TestGetSlice(t *testing.T) {
	before()

	e := []string{"a", "b", "c"}
	var dst []string

	f := cachefetcher.NewCacheFetcher(client, options)
	if err := f.SetKey([]string{"prefix", "key"}, e); err != nil {
		t.Errorf("%#v", err)
	}

	if err := f.Set(e, 10*time.Second); err != nil {
		t.Errorf("%#v", err)
	}

	err := f.Get(&dst)
	if err != nil {
		t.Errorf("%#v", err)
	}

	if !f.IsCached() {
		t.Errorf("%#v", f.IsCached())
	}

	if !reflect.DeepEqual(dst, e) {
		t.Errorf("%#v is not %#v", dst, e)
	}
}

func TestGetMap(t *testing.T) {
	before()

	e := map[int]string{1: "a", 2: "b", 3: "c"}
	var dst map[int]string

	f := cachefetcher.NewCacheFetcher(client, options)
	if err := f.SetKey([]string{"prefix", "key"}, "map"); err != nil {
		t.Errorf("%#v", err)
	}

	if err := f.Set(e, 10*time.Second); err != nil {
		t.Errorf("%#v", err)
	}

	err := f.Get(&dst)
	if err != nil {
		t.Errorf("%#v", err)
	}

	if !f.IsCached() {
		t.Errorf("%#v", f.IsCached())
	}

	if !reflect.DeepEqual(dst, e) {
		t.Errorf("%#v is not %#v", dst, e)
	}
}

func TestGetStruct(t *testing.T) {
	before()

	e := &testStruct2{A: 10, B: "abc", C: []bool{true, false}}
	var dst testStruct2

	f := cachefetcher.NewCacheFetcher(client, options)
	if err := f.SetKey([]string{"prefix", "key"}, "struct"); err != nil {
		t.Errorf("%#v", err)
	}

	if err := f.Set(e, 10*time.Second); err != nil {
		t.Errorf("%#v", err)
	}

	err := f.Get(&dst)
	if err != nil {
		t.Errorf("%#v", err)
	}

	if !f.IsCached() {
		t.Errorf("%#v", f.IsCached())
	}

	if !reflect.DeepEqual(&dst, e) {
		t.Errorf("%#v is not %#v", dst, e)
	}
}

func TestDel(t *testing.T) {
	before()

	f := cachefetcher.NewCacheFetcher(client, options)
	if err := f.SetKey([]string{"prefix", "key"}, "hoge", "fuga"); err != nil {
		t.Errorf("%#v", err)
	}

	if err := f.Set("value", 10*time.Second); err != nil {
		t.Errorf("%#v", err)
	}

	if err := f.Del(); err != nil {
		t.Errorf("%#v", err)
	}
	if !f.IsCached() {
		t.Errorf("%#v", f.IsCached())
	}

	var dst string
	err := f.Get(&dst)
	if err != nil && !errors.Is(err, redis.Nil) {
		t.Errorf("%#v", err)
	}
	if dst != "" {
		t.Errorf("%#v is not %#v", dst, "")
	}
}
