// Package cachefetcher is the function cache fetcher for golang.
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
	// CacheFetcher have main module functions.
	CacheFetcher interface {
		SetKey(prefixes []string, elements ...interface{}) error
		SetHashKey(prefixes []string, elements ...interface{}) error
		Fetch(expiration time.Duration, dst interface{}, fetcher interface{}) error
		Set(value interface{}, expiration time.Duration) error
		Get(dst interface{}) error
		GetString() (string, error)
		Del() error
		Key() string
		IsCached() bool
	}

	// Client is needs implement.
	Client interface {
		Set(key string, value interface{}, expiration time.Duration) error
		Get(key string, dst interface{}) error
		Del(key string) error
		IsErrCacheMiss(err error) bool
	}

	// Options is extended settings.
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
	defaultGroup = singleflight.Group{}

	// ErrInvalidKeyElements is invalid for setting key.
	ErrInvalidKeyElements = errors.New("cachefetcher: key elements is invalid")

	// ErrTimeout is singleflight's chan timeout.
	ErrTimeout = errors.New("cachefetcher: timeout")

	// ErrNoPointerType is Get's dst type is no pointer.
	ErrNoPointerType = errors.New("cachefetcher: no pointer type")
)

const (
	defaultGroupTimeout = 30 * time.Second
	skip                = 1
	sep                 = "_"
)

// NewCacheFetcher is new method for CacheFetcher.
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

// Set key.
func (f *cacheFetcherImpl) SetKey(prefixes []string, elements ...interface{}) error {
	e, err := f.toStringsForElements(elements...)
	if err != nil {
		return err
	}

	f.key = strings.ReplaceAll(strings.Join(append(prefixes, e), sep), " ", sep)
	return nil
}

// Set key with hash.
func (f *cacheFetcherImpl) SetHashKey(prefixes []string, elements ...interface{}) error {
	e, err := f.toStringsForElements(elements...)
	if err != nil {
		return err
	}

	s := sha256.Sum256([]byte(e))
	h := []string{hex.EncodeToString(s[:])}
	f.key = strings.ReplaceAll(strings.Join(append(prefixes, h...), sep), " ", sep)
	return nil
}

func (f *cacheFetcherImpl) toStringsForElements(elements ...interface{}) (string, error) {
	if len(elements) == 0 {
		return "", nil // no elements.
	}

	var el []string
	var err error

	for _, e := range elements {
		if e == nil {
			return "", ErrInvalidKeyElements
		}

		switch v := reflect.ValueOf(e); reflect.TypeOf(e).Kind() {
		case reflect.String, reflect.Bool, reflect.Int, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int8, reflect.Uint, reflect.Uint16,
			reflect.Uint32, reflect.Uint64, reflect.Uint8, reflect.Uintptr, reflect.Float32, reflect.Float64, reflect.Complex128, reflect.Complex64:

		case reflect.Ptr:
			if e, err = f.toStringsForElements(v.Elem().Interface()); err != nil {
				return "", err
			}

		case reflect.Array, reflect.Slice:
			var il []interface{}
			for i := 0; i < v.Len(); i++ {
				il = append(il, v.Index(i).Interface())
			}

			if e, err = f.toStringsForElements(il...); err != nil {
				return "", err
			}

		case reflect.Struct:
			if _, ok := e.(interface{ String() string }); !ok {
				return "", ErrInvalidKeyElements
			}

		case reflect.Map, reflect.Chan, reflect.Func, reflect.UnsafePointer, reflect.Interface, reflect.Invalid:
			return "", ErrInvalidKeyElements
		}

		el = append(el, fmt.Sprintf("%+v", e))
	}

	return strings.Join(el, sep), nil
}

// Fetch function or cache.
func (f *cacheFetcherImpl) Fetch(expiration time.Duration, dst interface{}, fetcher interface{}) error {
	ch := f.group.DoChan(f.key, f.fetch(expiration, dst, fetcher))

	select {
	case res := <-ch:
		if res.Err != nil {
			return res.Err
		}

		if err := f.debugPrint(); err != nil {
			return err
		}

		reflect.ValueOf(dst).Elem().Set(reflect.ValueOf(res.Val))
		return nil

	case <-time.After(f.groupTimeout):
		return ErrTimeout
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

// Set cache.
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

// Get cache as any interface.
func (f *cacheFetcherImpl) Get(dst interface{}) error {
	ch := f.group.DoChan(f.key, f.get(dst))

	select {
	case res := <-ch:
		if res.Err != nil {
			return res.Err
		}

		if err := f.debugPrint(); err != nil {
			return err
		}
		return nil

	case <-time.After(f.groupTimeout):
		return ErrTimeout
	}
}

func (f *cacheFetcherImpl) get(dst interface{}) func() (interface{}, error) {
	return func() (interface{}, error) {
		f.isCached = false

		if reflect.TypeOf(dst).Kind() != reflect.Ptr {
			return nil, fmt.Errorf("dst: %w", ErrNoPointerType)
		}

		if err := f.client.Get(f.key, dst); err != nil {
			return nil, err
		}

		f.isCached = true
		return reflect.ValueOf(dst).Elem().Interface(), nil
	}
}

// Get cache as string.
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
		return "", ErrTimeout
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

// Delete cache.
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

// Get key.
func (f *cacheFetcherImpl) Key() string {
	return f.key
}

// Get cached.
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
