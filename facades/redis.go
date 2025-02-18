package facades

import (
	"github.com/goravel/framework/contracts/cache"
	"github.com/goravel/framework/contracts/queue"
	"github.com/goravel/framework/support/color"

	"github.com/goravel/redis"
)

func Cache(store string) cache.Driver {
	if redis.App == nil {
		color.Warningln(redis.ErrRedisServiceProviderNotRegistered.Error())
		return nil
	}
	if store == "" {
		color.Warningln(redis.ErrRedisStoreIsRequired.Error())
		return nil
	}

	instance, err := redis.App.MakeWith(redis.CacheBinding, map[string]any{"store": store})
	if err != nil {
		color.Warningln(err.Error())
		return nil
	}

	return instance.(*redis.Cache)
}

func Queue(connection string) queue.Driver {
	if redis.App == nil {
		color.Warningln(redis.ErrRedisServiceProviderNotRegistered.Error())
		return nil
	}
	if connection == "" {
		color.Warningln(redis.ErrRedisConnectionIsRequired.Error())
		return nil
	}

	instance, err := redis.App.MakeWith(redis.QueueBinding, map[string]any{"connection": connection})
	if err != nil {
		color.Warningln(err.Error())
		return nil
	}

	return instance.(*redis.Queue)
}
