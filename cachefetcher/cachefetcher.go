package cachefetcher

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/k0kubun/pp"
	"golang.org/x/sync/singleflight"
)

type (
	CacheFetcher interface {
		SetKey(prefixes []string, useHash bool, elements ...string)
		Fetch(expiration time.Duration, dst interface{}, fetcher interface{}) (interface{}, error)
		Set(value interface{}, expiration time.Duration) error
		GetString() (string, error)
		Get(dst interface{}) (interface{}, error)
		Del() error
		Key() string
		IsCached() bool
	}

	Client interface {
		Set(key string, value interface{}, expiration time.Duration) error
		Get(key string, dst interface{}) error
		Del(key string) error
		IsFoundKey(err error) bool
	}

	Options struct {
		Group          *singleflight.Group
		GroupTimeout   time.Duration
		DebugPrintMode bool
	}

	cacheFetcherImpl struct {
		client         Client
		group          *singleflight.Group
		groupTimeout   time.Duration
		debugPrintMode bool

		key      string
		isCached bool // is used cache?
	}
)

var (
	defaultGroup     = singleflight.Group{}
	errTimeout       = errors.New("timeout")
	errNoPointerType = errors.New("no pointer type")
)

const defaultGroupTimeout = 30 * time.Second

func NewCacheFetcher(client Client, options *Options) CacheFetcher {
	// default
	if options == nil {
		options = &Options{}
	}
	if options.Group == nil {
		options.Group = &defaultGroup
	}
	if options.GroupTimeout == 0 {
		options.GroupTimeout = defaultGroupTimeout
	}

	return &cacheFetcherImpl{
		client:         client,
		group:          options.Group,
		groupTimeout:   options.GroupTimeout,
		debugPrintMode: options.DebugPrintMode,
	}
}

func (f *cacheFetcherImpl) SetKey(prefixes []string, useHash bool, elements ...string) {
	e := elements
	if useHash {
		s := sha256.Sum256([]byte(strings.Join(elements, "_")))
		e = []string{hex.EncodeToString(s[:])}
	}
	f.key = strings.Join(append(prefixes, e...), "_")
}

func (f *cacheFetcherImpl) Fetch(expiration time.Duration, dst interface{}, fetcher interface{}) (interface{}, error) {
	ch := f.group.DoChan(f.key, f.fetch(expiration, dst, fetcher))

	select {
	case res := <-ch:
		if res.Err != nil {
			return nil, res.Err
		}

		if f.debugPrintMode {
			// nolint
			pp.Printf("cache: %+v is %+v\n", f.key, f.isCached)
		}

		reflect.ValueOf(dst).Elem().Set(reflect.ValueOf(res.Val))
		return res.Val, nil

	case <-time.After(f.groupTimeout):
		return nil, errTimeout
	}
}

func (f *cacheFetcherImpl) fetch(expiration time.Duration, dst interface{}, fetcher interface{}) func() (interface{}, error) {
	return func() (interface{}, error) {
		cRes, err := f.get(dst)()
		if err != nil && f.client.IsFoundKey(err) {
			return nil, err // no add error stack.
		}

		if f.isCached {
			return cRes, nil
		}

		// fetch function
		v := reflect.ValueOf(fetcher).Call(nil)
		if !v[1].IsNil() {
			return nil, v[1].Interface().(error) // no add error stack.
		}

		fRes := v[0].Interface()
		if reflect.TypeOf(fRes).Kind() == reflect.Ptr {
			fRes = reflect.ValueOf(fRes).Elem().Interface()
		}
		if err := f.Set(fRes, expiration); err != nil {
			return nil, err // no add error stack.
		}

		return fRes, nil
	}
}

func (f *cacheFetcherImpl) Set(value interface{}, expiration time.Duration) error {
	err := f.client.Set(f.key, value, expiration)
	if err != nil {
		return err
	}

	if f.debugPrintMode {
		// nolint
		pp.Printf("set: %+v\n", f.key)
	}
	return nil
}

func (f *cacheFetcherImpl) GetString() (string, error) {
	ch := f.group.DoChan(f.key, f.getString())

	select {
	case res := <-ch:
		if res.Err != nil {
			return "", res.Err
		}

		if f.debugPrintMode {
			// nolint
			pp.Printf("get: %+v is %+v\n", f.key, f.isCached)
		}
		return res.Val.(string), nil

	case <-time.After(f.groupTimeout):
		return "", errTimeout
	}
}

func (f *cacheFetcherImpl) getString() func() (interface{}, error) {
	return func() (interface{}, error) {
		var dst string
		err := f.client.Get(f.key, &dst)
		if err != nil {
			return nil, err
		}

		f.isCached = true
		return dst, nil
	}
}

func (f *cacheFetcherImpl) Get(dst interface{}) (interface{}, error) {
	ch := f.group.DoChan(f.key, f.get(dst))

	select {
	case res := <-ch:
		if res.Err != nil {
			return nil, res.Err
		}

		if f.debugPrintMode {
			// nolint
			pp.Printf("get: %+v is %+v\n", f.key, f.isCached)
		}
		return res.Val, nil

	case <-time.After(f.groupTimeout):
		return nil, errTimeout
	}
}

func (f *cacheFetcherImpl) get(dst interface{}) func() (interface{}, error) {
	return func() (interface{}, error) {
		if reflect.TypeOf(dst).Kind() != reflect.Ptr {
			return nil, fmt.Errorf("%w: dst", errNoPointerType)
		}

		if err := f.client.Get(f.key, dst); err != nil {
			return nil, err
		}

		f.isCached = true
		return reflect.ValueOf(dst).Elem().Interface(), nil
	}
}

func (f *cacheFetcherImpl) Del() error {
	return f.client.Del(f.key)
}

func (f *cacheFetcherImpl) Key() string {
	return f.key
}

func (f *cacheFetcherImpl) IsCached() bool {
	return f.isCached
}
