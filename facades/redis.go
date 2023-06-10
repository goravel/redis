package facades

import (
	"log"

	"github.com/goravel/framework/contracts/cache"

	"github.com/goravel/redis"
)

func Redis(connection string) cache.Driver {
	if connection == "" {
		log.Fatalln("connection is required")
		return nil
	}

	instance, err := redis.App.MakeWith(redis.Binding, map[string]any{"connection": connection})
	if err != nil {
		log.Fatalln(err)
		return nil
	}

	return instance.(*redis.Redis)
}
