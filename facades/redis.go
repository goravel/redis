package facades

import (
	"log"

	"github.com/goravel/framework/contracts/cache"

	"github.com/goravel/redis"
)

func Redis(store string) cache.Driver {
	if redis.App == nil {
		log.Fatalln("please register redis service provider")
		return nil
	}
	if store == "" {
		log.Fatalln("store is required")
		return nil
	}

	instance, err := redis.App.MakeWith(redis.Binding, map[string]any{"store": store})
	if err != nil {
		log.Fatalln(err)
		return nil
	}

	return instance.(*redis.Redis)
}
