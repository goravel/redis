# Redis

A redis driver for `facades.Cache()` and `facades.Queue()` of Goravel.

## Version

| goravel/redis  | goravel/framework    |
| ----------     | --------------       |
| v1.2.*         | v1.14.*              |
| v1.1.*         | v1.13.*              |
| v1.0.*         | v1.12.*              |

## Install

1. Add package

```
go get -u github.com/goravel/redis
```

2. Register service provider

```
// config/app.go
import "github.com/goravel/redis"

"providers": []foundation.ServiceProvider{
    ...
    // Need register redis service provider before cache and queue service provider
    &redis.ServiceProvider{},
   // Exists in the config/app.go file, DO NOT copy this line
    &cache.ServiceProvider{},
    &queue.ServiceProvider{},
    ...
}
```

3. Add your redis configuration to `config/cache.go` file if you want to use redis as cache driver

```
import (
    "github.com/goravel/framework/contracts/cache"
    redisfacades "github.com/goravel/redis/facades"
)

"stores": map[string]any{
    ...
    "redis": map[string]any{
        "driver": "custom",
        "connection": "default",
        "via": func() (cache.Driver, error) {
            return redisfacades.Cache("redis") // The `redis` value is the key of `stores`
        },
    },
},
```

4. Add your redis configuration to `config/queue.go` file if you want to use redis as queue driver

```
import (
    "github.com/goravel/framework/contracts/queue"
    redisfacades "github.com/goravel/redis/facades"
)

"connections": map[string]any{
    ...
    "redis": map[string]any{
        "driver": "custom",
        "connection": "default",
        "via": func() (queue.Driver, error) {
            return redisfacades.Queue("redis"), nil // The `redis` value is the key of `connections`
        },
    },
},
```

5. Fill redis configuration to `config/database.go` file

```
// config/database.go
"redis": map[string]any{
    "default": map[string]any{
        "host":     config.Env("REDIS_HOST", ""),
        "password": config.Env("REDIS_PASSWORD", ""),
        "port":     config.Env("REDIS_PORT", 6379),
        "database": config.Env("REDIS_DB", 0),
    },
},
```

To enable TLS/SSL, you need to add the following configuration:

```
import "crypto/tls"

// config/database.go
"redis": map[string]any{
    "default": map[string]any{
        "host":     config.Env("REDIS_HOST", ""),
        "password": config.Env("REDIS_PASSWORD", ""),
        "port":     config.Env("REDIS_PORT", 6379),
        "database": config.Env("REDIS_DB", 0),
        "tls":      &tls.Config{
            // Add your tls configuration here
        },
    },
},
```

## Testing

Run command below to run test:

```
go test ./...
```
