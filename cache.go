package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/goravel/framework/cache"
	contractscache "github.com/goravel/framework/contracts/cache"
	"github.com/goravel/framework/contracts/config"
	contractshttp "github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/contracts/process"
	contractsdocker "github.com/goravel/framework/contracts/testing/docker"
	"github.com/goravel/framework/errors"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cast"
)

var _ contractscache.Driver = &Cache{}

type Cache struct {
	ctx      context.Context
	config   config.Config
	process  process.Process
	prefix   string
	instance redis.UniversalClient
	store    string
}

func NewCache(ctx context.Context, config config.Config, process process.Process, store string) (*Cache, error) {
	connection := config.GetString(fmt.Sprintf("cache.stores.%s.connection", store), "default")

	client, err := GetClient(config, connection)
	if err != nil {
		return nil, fmt.Errorf("failed to init redis client: %w", err)
	}

	return &Cache{
		ctx:      ctx,
		prefix:   fmt.Sprintf("%s:", config.GetString("cache.prefix")),
		instance: client,
		store:    store,
		config:   config,
		process:  process,
	}, nil
}

// Add Driver an item in the cache if the key does not exist.
func (r *Cache) Add(key string, value any, t time.Duration) bool {
	val, err := r.instance.SetNX(r.ctx, r.key(key), value, t).Result()
	if err != nil {
		return false
	}

	return val
}

func (r *Cache) Decrement(key string, value ...int64) (int64, error) {
	if len(value) == 0 {
		value = append(value, 1)
	}

	return r.instance.DecrBy(r.ctx, r.key(key), value[0]).Result()
}

func (r *Cache) Docker() (contractsdocker.CacheDriver, error) {
	if r.process == nil {
		return nil, errors.ProcessFacadeNotSet
	}

	return NewDocker(r.config, r.process, r.store)
}

// Forever Driver an item in the cache indefinitely.
func (r *Cache) Forever(key string, value any) bool {
	if err := r.Put(key, value, 0); err != nil {
		return false
	}

	return true
}

// Forget Remove an item from the cache.
func (r *Cache) Forget(key string) bool {
	_, err := r.instance.Del(r.ctx, r.key(key)).Result()

	return err == nil
}

// Flush Remove all items from the cache.
func (r *Cache) Flush() bool {
	res, err := r.instance.FlushAll(r.ctx).Result()

	if err != nil || res != "OK" {
		return false
	}

	return true
}

// Get Retrieve an item from the cache by key.
func (r *Cache) Get(key string, def ...any) any {
	val, err := r.instance.Get(r.ctx, r.key(key)).Result()
	if err != nil {
		if len(def) == 0 {
			return nil
		}

		switch s := def[0].(type) {
		case func() any:
			return s()
		default:
			return s
		}
	}

	return val
}

func (r *Cache) GetBool(key string, def ...bool) bool {
	if len(def) == 0 {
		def = append(def, false)
	}
	res := r.Get(key, def[0])
	if val, ok := res.(string); ok {
		return val == "1"
	}

	return cast.ToBool(res)
}

func (r *Cache) GetInt(key string, def ...int) int {
	if len(def) == 0 {
		def = append(def, 1)
	}
	res := r.Get(key, def[0])
	if val, ok := res.(string); ok {
		i, err := strconv.Atoi(val)
		if err != nil {
			return def[0]
		}

		return i
	}

	return cast.ToInt(res)
}

func (r *Cache) GetInt64(key string, def ...int64) int64 {
	if len(def) == 0 {
		def = append(def, 1)
	}
	res := r.Get(key, def[0])
	if val, ok := res.(string); ok {
		i, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return def[0]
		}

		return i
	}

	return cast.ToInt64(res)
}

func (r *Cache) GetString(key string, def ...string) string {
	if len(def) == 0 {
		def = append(def, "")
	}
	return cast.ToString(r.Get(key, def[0]))
}

// Has Check an item exists in the cache.
func (r *Cache) Has(key string) bool {
	value, err := r.instance.Exists(r.ctx, r.key(key)).Result()

	if err != nil || value == 0 {
		return false
	}

	return true
}

func (r *Cache) Increment(key string, value ...int64) (int64, error) {
	if len(value) == 0 {
		value = append(value, 1)
	}

	return r.instance.IncrBy(r.ctx, r.key(key), value[0]).Result()
}

func (r *Cache) Lock(key string, t ...time.Duration) contractscache.Lock {
	return cache.NewLock(r, key, t...)
}

// Put Driver an item in the cache for a given time.
func (r *Cache) Put(key string, value any, t time.Duration) error {
	err := r.instance.Set(r.ctx, r.key(key), value, t).Err()
	if err != nil {
		return err
	}

	return nil
}

// Pull Retrieve an item from the cache and delete it.
func (r *Cache) Pull(key string, def ...any) any {
	var res any
	if len(def) == 0 {
		res = r.Get(key)
	} else {
		res = r.Get(key, def[0])
	}
	r.Forget(key)

	return res
}

// Remember Get an item from the cache, or execute the given Closure and store the result.
func (r *Cache) Remember(key string, seconds time.Duration, callback func() (any, error)) (any, error) {
	val := r.Get(key, nil)

	if val != nil {
		return val, nil
	}

	var err error
	val, err = callback()
	if err != nil {
		return nil, err
	}

	if err := r.Put(key, val, seconds); err != nil {
		return nil, err
	}

	return val, nil
}

// RememberForever Get an item from the cache, or execute the given Closure and store the result forever.
func (r *Cache) RememberForever(key string, callback func() (any, error)) (any, error) {
	val := r.Get(key, nil)

	if val != nil {
		return val, nil
	}

	var err error
	val, err = callback()
	if err != nil {
		return nil, err
	}

	if err := r.Put(key, val, 0); err != nil {
		return nil, err
	}

	return val, nil
}

func (r *Cache) WithContext(ctx context.Context) contractscache.Driver {
	if http, ok := ctx.(contractshttp.Context); ok {
		ctx = http.Context()
	}

	store, _ := NewCache(ctx, r.config, r.process, r.store)

	return store
}

func (r *Cache) key(key string) string {
	return r.prefix + key
}
