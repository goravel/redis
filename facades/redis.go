package facades

import (
	"github.com/goravel/framework/contracts/cache"
	"github.com/goravel/framework/contracts/queue"
	"github.com/goravel/framework/contracts/session"

	goredis "github.com/redis/go-redis/v9"

	"github.com/goravel/redis"
)

func Cache(store string) (cache.Driver, error) {
	if redis.App == nil {
		return nil, redis.ErrRedisServiceProviderNotRegistered
	}
	if store == "" {
		return nil, redis.ErrRedisStoreIsRequired
	}

	instance, err := redis.App.MakeWith(redis.BindingCache, map[string]any{"store": store})
	if err != nil {
		return nil, err
	}

	return instance.(*redis.Cache), nil
}

func Queue(connection string) (queue.Driver, error) {
	if redis.App == nil {
		return nil, redis.ErrRedisServiceProviderNotRegistered
	}
	if connection == "" {
		return nil, redis.ErrRedisConnectionIsRequired
	}

	instance, err := redis.App.MakeWith(redis.BindingQueue, map[string]any{"connection": connection})
	if err != nil {
		return nil, err
	}

	return instance.(*redis.Queue), nil
}

func Session(driver string) (session.Driver, error) {
	if redis.App == nil {
		return nil, redis.ErrRedisServiceProviderNotRegistered
	}
	if driver == "" {
		return nil, redis.ErrRedisConnectionIsRequired
	}

	instance, err := redis.App.MakeWith(redis.BindingSession, map[string]any{"driver": driver})
	if err != nil {
		return nil, err
	}

	return instance.(*redis.Session), nil
}

// Instance returns a Redis client instance for the specified connection name.
// This might be useful for some advanced usages.
func Instance(connection string) (goredis.UniversalClient, error) {
	config := redis.App.MakeConfig()
	return redis.GetClient(config, connection)
}
