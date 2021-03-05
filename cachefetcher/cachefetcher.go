package cachefetcher

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/k0kubun/pp"
	"golang.org/x/sync/singleflight"
)

type (
	CacheFetcher interface {
		SetKey(prefixes []string, elements ...string)
		SetHashKey(prefixes []string, elements ...string)
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
		IsErrCacheMiss(err error) bool
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

const (
	defaultGroupTimeout = 30 * time.Second
	skip                = 1
)

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

func (f *cacheFetcherImpl) SetKey(prefixes []string, elements ...string) {
	f.key = strings.Join(append(prefixes, elements...), "_")
}

func (f *cacheFetcherImpl) SetHashKey(prefixes []string, elements ...string) {
	s := sha256.Sum256([]byte(strings.Join(elements, "_")))
	e := []string{hex.EncodeToString(s[:])}
	f.key = strings.Join(append(prefixes, e...), "_")
}

func (f *cacheFetcherImpl) Fetch(expiration time.Duration, dst interface{}, fetcher interface{}) (interface{}, error) {
	ch := f.group.DoChan(f.key, f.fetch(expiration, dst, fetcher))

	select {
	case res := <-ch:
		if res.Err != nil {
			return nil, res.Err
		}

		if err := f.debugPrint(); err != nil {
			return nil, err
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
		if f.isErrOtherThanCacheMiss(err) {
			return nil, err
		}

		if f.isCached {
			return cRes, nil
		}

		// fetch function
		v := reflect.ValueOf(fetcher).Call(nil)
		if !v[1].IsNil() {
			return nil, v[1].Interface().(error)
		}

		fRes := v[0].Interface()
		if reflect.TypeOf(fRes).Kind() == reflect.Ptr {
			fRes = reflect.ValueOf(fRes).Elem().Interface()
		}
		if err := f.set(fRes, expiration); err != nil {
			return nil, err
		}

		return fRes, nil
	}
}

func (f *cacheFetcherImpl) Set(value interface{}, expiration time.Duration) error {
	f.isCached = false
	if err := f.set(value, expiration); err != nil {
		return err
	}
	f.isCached = true

	if err := f.debugPrint(); err != nil {
		return err
	}
	return nil
}

func (f *cacheFetcherImpl) set(value interface{}, expiration time.Duration) error {
	return f.client.Set(f.key, value, expiration)
}

func (f *cacheFetcherImpl) GetString() (string, error) {
	ch := f.group.DoChan(f.key, f.getString())

	select {
	case res := <-ch:
		if res.Err != nil {
			return "", res.Err
		}

		if err := f.debugPrint(); err != nil {
			return "", err
		}
		return res.Val.(string), nil

	case <-time.After(f.groupTimeout):
		return "", errTimeout
	}
}

func (f *cacheFetcherImpl) getString() func() (interface{}, error) {
	return func() (interface{}, error) {
		f.isCached = false

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

		if err := f.debugPrint(); err != nil {
			return nil, err
		}
		return res.Val, nil

	case <-time.After(f.groupTimeout):
		return nil, errTimeout
	}
}

func (f *cacheFetcherImpl) get(dst interface{}) func() (interface{}, error) {
	return func() (interface{}, error) {
		f.isCached = false

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
	err := f.client.Del(f.key)
	f.isCached = true
	if f.client.IsErrCacheMiss(err) {
		f.isCached = false
	}
	if err != nil {
		return err
	}

	if err := f.debugPrint(); err != nil {
		return err
	}
	return nil
}

func (f *cacheFetcherImpl) Key() string {
	return f.key
}

func (f *cacheFetcherImpl) IsCached() bool {
	return f.isCached
}

func (f *cacheFetcherImpl) isErrOtherThanCacheMiss(err error) bool {
	return err != nil && !f.client.IsErrCacheMiss(err)
}

func (f *cacheFetcherImpl) debugPrint() error {
	if f.debugPrintMode {
		pc, _, _, _ := runtime.Caller(skip)
		names := strings.Split(runtime.FuncForPC(pc).Name(), "/")
		_, err := pp.Printf("%+v: key: %+v, cache: %+v\n", names[len(names)-1], f.key, f.isCached)
		return err
	}
	return nil
}
