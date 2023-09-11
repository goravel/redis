# Redis

A redis disk driver for `facades.Cache()` of Goravel.

## Version

| goravel/redis  | goravel/framework    |
| ----------     | --------------       |
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
    // Need register redis service provider before cache service provider
    &redis.ServiceProvider{},
    &cache.ServiceProvider{},
    ...
}
```

3. Add your redis configuration to `config/cache.go` file

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
            return redisfacades.Redis("redis"), nil // The `redis` value is the key of `stores`
        },
    },
},
```

4. Fill redis configuration to `config/database.go` file

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

## Testing

Run command below to run test:

```
go test ./...
```
