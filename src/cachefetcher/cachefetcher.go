// Package cachefetcher is the function cache fetcher for golang.
package cachefetcher

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
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
	Factory interface {
		NewFetcher() CacheFetcher
	}

	// CacheFetcher have main module functions.
	CacheFetcher interface {
		SetKey(prefixes []string, elements ...interface{}) error
		SetHashKey(prefixes []string, elements ...interface{}) error
		Key() string

		Fetch(expiration time.Duration, dst interface{}, fetcher interface{}) error
		Set(value interface{}, expiration time.Duration) error
		Get(dst interface{}) error
		SetString(value string, expiration time.Duration) error
		GetString() (string, error)
		Del() error

		GobRegister(value interface{})
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
		Group           *singleflight.Group
		GroupTimeout    time.Duration
		DebugPrintMode  bool
		IsNotSerialized bool // serialize default with using gob serializer.
	}

	factoryImpl struct {
		client  Client
		options *Options
	}

	cacheFetcherImpl struct {
		client  Client
		options *Options

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

	// ErrGobSerialized failed to encode or decode of gob.
	ErrGobSerialized = errors.New("cachefetcher: gob serialized failed")
)

const (
	defaultGroupTimeout = 5 * time.Minute
	skip                = 1
	sep                 = "_"
)

// NewCacheFetcher is new method for CacheFetcher.
func NewFactory(client Client, options *Options) Factory {
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

	return &factoryImpl{client: client, options: options}
}

func (b *factoryImpl) NewFetcher() CacheFetcher {
	return &cacheFetcherImpl{
		client:  b.client,
		options: b.options,
	}
}

// Set key.
func (f *cacheFetcherImpl) SetKey(prefixes []string, elements ...interface{}) error {
	return f.setKey(prefixes, elements, false)
}

// Set key with hash.
func (f *cacheFetcherImpl) SetHashKey(prefixes []string, elements ...interface{}) error {
	return f.setKey(prefixes, elements, true)
}

func (f *cacheFetcherImpl) setKey(prefixes []string, elements []interface{}, useHash bool) error {
	s := prefixes
	if len(elements) > 0 {
		e, err := f.toStringsForElements(elements...)
		if err != nil {
			return err
		}

		h := e
		if useHash {
			b := sha256.Sum256([]byte(e))
			h = hex.EncodeToString(b[:])
		}
		s = append(s, h)
	}

	f.key = strings.ReplaceAll(strings.Join(s, sep), " ", sep)
	return nil
}

// Get key.
func (f *cacheFetcherImpl) Key() string {
	return f.key
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
	select {
	case res := <-f.options.Group.DoChan(f.key, f.fetch(expiration, dst, fetcher)):
		if res.Err != nil {
			return res.Err
		}

		if err := f.debugPrint(); err != nil {
			return err
		}

		return nil

	case <-time.After(f.options.GroupTimeout):
		return ErrTimeout
	}
}

func (f *cacheFetcherImpl) fetch(expiration time.Duration, dst interface{}, fetcher interface{}) func() (interface{}, error) {
	return func() (interface{}, error) {
		_, err := f.get(dst, false)()
		if f.isErrOtherThanCacheMiss(err) {
			return nil, err
		}

		if f.isCached {
			return nil, nil
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

		isCached := f.isCached
		if err := f.set(fRes, expiration, false); err != nil {
			return nil, err
		}
		f.isCached = isCached // replace get's isCached

		reflect.ValueOf(dst).Elem().Set(reflect.ValueOf(fRes))
		return nil, nil
	}
}

// Set cache.
func (f *cacheFetcherImpl) Set(value interface{}, expiration time.Duration) error {
	if err := f.set(value, expiration, false); err != nil {
		return err
	}

	if err := f.debugPrint(); err != nil {
		return err
	}
	return nil
}

// Set cache.
func (f *cacheFetcherImpl) SetString(value string, expiration time.Duration) error {
	if err := f.set(value, expiration, true); err != nil {
		return err
	}

	if err := f.debugPrint(); err != nil {
		return err
	}
	return nil
}

func (f *cacheFetcherImpl) set(value interface{}, expiration time.Duration, isStringMode bool) error {
	f.isCached = false
	v := value
	if !(isStringMode || f.options.IsNotSerialized) {
		buf := new(bytes.Buffer)
		if err := gob.NewEncoder(buf).Encode(value); err != nil {
			return fmt.Errorf("%w: %+v", ErrGobSerialized, err)
		}

		v = buf.String()
	}

	if err := f.client.Set(f.key, v, expiration); err != nil {
		return err
	}

	f.isCached = true
	return nil
}

// Get cache as any interface.
func (f *cacheFetcherImpl) Get(dst interface{}) error {
	select {
	case res := <-f.options.Group.DoChan(f.key, f.get(dst, false)):
		if res.Err != nil {
			return res.Err
		}

		if err := f.debugPrint(); err != nil {
			return err
		}
		return nil

	case <-time.After(f.options.GroupTimeout):
		return ErrTimeout
	}
}

// Get cache as string.
func (f *cacheFetcherImpl) GetString() (string, error) {
	var dst string

	select {
	case res := <-f.options.Group.DoChan(f.key, f.get(&dst, true)):
		if res.Err != nil {
			return "", res.Err
		}

		if err := f.debugPrint(); err != nil {
			return "", err
		}
		return dst, nil

	case <-time.After(f.options.GroupTimeout):
		return "", ErrTimeout
	}
}

func (f *cacheFetcherImpl) get(dst interface{}, isStringMode bool) func() (interface{}, error) {
	return func() (interface{}, error) {
		f.isCached = false

		if reflect.TypeOf(dst).Kind() != reflect.Ptr {
			return nil, fmt.Errorf("dst: %w", ErrNoPointerType)
		}

		var s string
		if err := f.client.Get(f.key, &s); err != nil {
			return nil, err
		}

		if isStringMode || f.options.IsNotSerialized {
			reflect.ValueOf(dst).Elem().SetString(s)
		} else {
			buf := bytes.NewBufferString(s)
			if err := gob.NewDecoder(buf).Decode(dst); err != nil {
				return nil, fmt.Errorf("%w: %+v", ErrGobSerialized, err)
			}
		}

		f.isCached = true
		return nil, nil
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

// GobRegister is register gob.
func (f *cacheFetcherImpl) GobRegister(value interface{}) {
	gob.Register(value)
}

// Get cached.
func (f *cacheFetcherImpl) IsCached() bool {
	return f.isCached
}

func (f *cacheFetcherImpl) isErrOtherThanCacheMiss(err error) bool {
	return err != nil && !f.client.IsErrCacheMiss(err)
}

func (f *cacheFetcherImpl) debugPrint() error {
	if f.options.DebugPrintMode {
		pc, _, _, _ := runtime.Caller(skip)
		names := strings.Split(runtime.FuncForPC(pc).Name(), "/")
		_, err := pp.Printf("%+v: key: %+v, cache: %+v\n", names[len(names)-1], f.key, f.isCached)
		return err
	}
	return nil
}
