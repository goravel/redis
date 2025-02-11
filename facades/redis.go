package facades

import (
	"log"

	"github.com/goravel/framework/contracts/cache"
	"github.com/goravel/framework/contracts/queue"

	"github.com/goravel/redis"
)

func Cache(store string) cache.Driver {
	if redis.App == nil {
		log.Fatalln("please register redis service provider")
		return nil
	}
	if store == "" {
		log.Fatalln("store is required")
		return nil
	}

	instance, err := redis.App.MakeWith(redis.CacheBinding, map[string]any{"store": store})
	if err != nil {
		log.Fatalln(err)
		return nil
	}

	return instance.(*redis.Cache)
}

func Queue(connection string) queue.Driver {
	if redis.App == nil {
		log.Fatalln("please register redis service provider")
		return nil
	}
	if connection == "" {
		log.Fatalln("connection is required")
		return nil
	}

	instance, err := redis.App.MakeWith(redis.QueueBinding, map[string]any{"connection": connection})
	if err != nil {
		log.Fatalln(err)
		return nil
	}

	return instance.(*redis.Queue)
}
