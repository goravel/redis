package redis

import (
	"context"
	"crypto/tls"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/pkg/errors"
	"github.com/spf13/cast"

	"github.com/goravel/framework/contracts/cache"
	"github.com/goravel/framework/contracts/config"
)

var _ cache.Driver = &Redis{}

type Redis struct {
	ctx      context.Context
	config   config.Config
	prefix   string
	instance *redis.Client
	store    string
}

func NewRedis(ctx context.Context, config config.Config, store string) (*Redis, error) {
	connection := config.GetString(fmt.Sprintf("cache.stores.%s.connection", store), "default")
	host := config.GetString(fmt.Sprintf("database.redis.%s.host", connection))
	if host == "" {
		return nil, nil
	}

	option := &redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, config.GetString(fmt.Sprintf("database.redis.%s.port", connection))),
		Password: config.GetString(fmt.Sprintf("database.redis.%s.password", connection)),
		DB:       config.GetInt(fmt.Sprintf("database.redis.%s.database", connection)),
	}

	tlsConfig, ok := config.Get(fmt.Sprintf("database.redis.%s.tls", connection)).(*tls.Config)
	if ok {
		option.TLSConfig = tlsConfig
	}

	client := redis.NewClient(option)

	if _, err := client.Ping(context.Background()).Result(); err != nil {
		return nil, errors.WithMessage(err, "init connection error")
	}

	return &Redis{
		ctx:      ctx,
		prefix:   fmt.Sprintf("%s:", config.GetString("cache.prefix")),
		instance: client,
		store:    store,
		config:   config,
	}, nil
}

// Add Driver an item in the cache if the key does not exist.
func (r *Redis) Add(key string, value any, t time.Duration) bool {
	val, err := r.instance.SetNX(r.ctx, r.key(key), value, t).Result()
	if err != nil {
		return false
	}

	return val
}

func (r *Redis) Decrement(key string, value ...int) (int, error) {
	if len(value) == 0 {
		value = append(value, 1)
	}

	res, err := r.instance.DecrBy(r.ctx, r.key(key), int64(value[0])).Result()

	return int(res), err
}

// Forever Driver an item in the cache indefinitely.
func (r *Redis) Forever(key string, value any) bool {
	if err := r.Put(key, value, 0); err != nil {
		return false
	}

	return true
}

// Forget Remove an item from the cache.
func (r *Redis) Forget(key string) bool {
	_, err := r.instance.Del(r.ctx, r.key(key)).Result()

	return err == nil
}

// Flush Remove all items from the cache.
func (r *Redis) Flush() bool {
	res, err := r.instance.FlushAll(r.ctx).Result()

	if err != nil || res != "OK" {
		return false
	}

	return true
}

// Get Retrieve an item from the cache by key.
func (r *Redis) Get(key string, def ...any) any {
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

func (r *Redis) GetBool(key string, def ...bool) bool {
	if len(def) == 0 {
		def = append(def, false)
	}
	res := r.Get(key, def[0])
	if val, ok := res.(string); ok {
		return val == "1"
	}

	return cast.ToBool(res)
}

func (r *Redis) GetInt(key string, def ...int) int {
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

func (r *Redis) GetInt64(key string, def ...int64) int64 {
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

func (r *Redis) GetString(key string, def ...string) string {
	if len(def) == 0 {
		def = append(def, "")
	}
	return cast.ToString(r.Get(key, def[0]))
}

// Has Check an item exists in the cache.
func (r *Redis) Has(key string) bool {
	value, err := r.instance.Exists(r.ctx, r.key(key)).Result()

	if err != nil || value == 0 {
		return false
	}

	return true
}

func (r *Redis) Increment(key string, value ...int) (int, error) {
	if len(value) == 0 {
		value = append(value, 1)
	}

	res, err := r.instance.IncrBy(r.ctx, r.key(key), int64(value[0])).Result()

	return int(res), err
}

func (r *Redis) Lock(key string, t ...time.Duration) cache.Lock {
	return NewLock(r, key, t...)
}

// Put Driver an item in the cache for a given time.
func (r *Redis) Put(key string, value any, t time.Duration) error {
	err := r.instance.Set(r.ctx, r.key(key), value, t).Err()
	if err != nil {
		return err
	}

	return nil
}

// Pull Retrieve an item from the cache and delete it.
func (r *Redis) Pull(key string, def ...any) any {
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
func (r *Redis) Remember(key string, seconds time.Duration, callback func() (any, error)) (any, error) {
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
func (r *Redis) RememberForever(key string, callback func() (any, error)) (any, error) {
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

func (r *Redis) WithContext(ctx context.Context) cache.Driver {
	store, _ := NewRedis(ctx, r.config, r.store)

	return store
}

func (r *Redis) key(key string) string {
	return r.prefix + key
}
