package redisfetcher

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/k0kubun/pp"
	perrors "github.com/pkg/errors"
	"golang.org/x/sync/singleflight"
)

type (
	RedisFetcher interface {
		SetKey(prefixes []string, usedUUID bool, elements ...string)
		Fetch(expiration time.Duration, dst interface{}, fetcher interface{}) (interface{}, error)
		SetVal(value interface{}, expiration time.Duration) error
		GetVal(dst interface{}) (interface{}, error)
		DelVal() error
		Key() string
		IsCached() bool
	}

	Client interface {
		Set(key string, value interface{}, expiration time.Duration) error
		// GetSimple(key string) (string, error)
		Get(key string, dst interface{}) error
		Del(key string) error
	}

	Options struct {
		Group          *singleflight.Group
		GroupTimeout   time.Duration
		WithStackTrace bool
		DebugPrintMode bool
	}

	redisFetcherImpl struct {
		client         Client
		group          *singleflight.Group
		groupTimeout   time.Duration
		withStackTrace bool
		debugPrintMode bool

		key      string
		isCached bool // is used redis cache?
	}
)

var defaultGroup = singleflight.Group{}

const defaultGroupTimeout = 30 * time.Second

func NewRedisFetcher(client Client, options *Options) RedisFetcher {
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

	return &redisFetcherImpl{
		client:         client,
		group:          options.Group,
		groupTimeout:   options.GroupTimeout,
		withStackTrace: options.WithStackTrace,
		debugPrintMode: options.DebugPrintMode,
	}
}

func (f *redisFetcherImpl) SetKey(prefixes []string, usedUUID bool, elements ...string) {
	e := elements
	if usedUUID {
		s := sha256.Sum256([]byte(strings.Join(elements, "_")))
		e = []string{hex.EncodeToString(s[:])}
	}
	f.key = strings.Join(append(prefixes, e...), "_")
}

func (f *redisFetcherImpl) Fetch(expiration time.Duration, dst interface{}, fetcher interface{}) (interface{}, error) {
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
		return res.Val, nil

	case <-time.After(f.groupTimeout):
		return nil, f.newError("fetch timeout: %+v", f.groupTimeout)
	}
}

func (f *redisFetcherImpl) fetch(expiration time.Duration, dst interface{}, fetcher interface{}) func() (interface{}, error) {
	return func() (interface{}, error) {
		cRes, err := f.get(dst)()
		if err != nil && !errors.Is(err, redis.Nil) {
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
		if err := f.SetVal(fRes, expiration); err != nil {
			return nil, err // no add error stack.
		}

		return fRes, nil
	}
}

func (f *redisFetcherImpl) SetVal(value interface{}, expiration time.Duration) error {
	err := f.withStack(f.client.Set(f.key, value, expiration))
	if err != nil {
		return err
	}

	if f.debugPrintMode {
		// nolint
		pp.Printf("set: %+v\n", f.key)
	}
	return nil
}

// adapt single flight
func (f *redisFetcherImpl) GetVal(dst interface{}) (interface{}, error) {
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
		return nil, f.newError("get timeout: %+v", f.groupTimeout)
	}
}

func (f *redisFetcherImpl) get(dst interface{}) func() (interface{}, error) {
	return func() (interface{}, error) {
		if reflect.TypeOf(dst).Kind() != reflect.Ptr {
			return nil, f.newError("dst requires pointer type")
		}

		if err := f.client.Get(f.key, dst); err != nil {
			return nil, f.withStack(err)
		}

		f.isCached = true
		return reflect.ValueOf(dst).Elem().Interface(), nil
	}
}

func (f *redisFetcherImpl) DelVal() error {
	return f.withStack(f.client.Del(f.key))
}

func (f *redisFetcherImpl) Key() string {
	return f.key
}

func (f *redisFetcherImpl) IsCached() bool {
	return f.isCached
}

func (f *redisFetcherImpl) withStack(err error) error {
	if f.withStackTrace {
		return perrors.WithStack(err)
	}
	return err
}

func (f *redisFetcherImpl) newError(format string, args ...interface{}) error {
	if f.withStackTrace {
		return perrors.Errorf(format, args...)
	}
	// nolint:goerr113
	return fmt.Errorf(format, args...)
}
