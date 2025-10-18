package main

import (
	"os"

	"github.com/goravel/framework/contracts/facades"
	"github.com/goravel/framework/packages"
	"github.com/goravel/framework/packages/match"
	"github.com/goravel/framework/packages/modify"
	"github.com/goravel/framework/support/path"
)

var (
	cacheConfig = `map[string]any{
        "driver": "custom",
        "connection": "default",
        "via": func() (cache.Driver, error) {
            return redisfacades.Cache("redis") // The ` + "`redis`" + ` value is the key of ` + "`stores`" + `
        },
    }`
	databaseConfig = `map[string]any{
        "default": map[string]any{
            "host":     config.Env("REDIS_HOST", ""),
            "password": config.Env("REDIS_PASSWORD", ""),
            "port":     config.Env("REDIS_PORT", 6379),
            "database": config.Env("REDIS_DB", 0),
        },
    }`
	queueConfig = `map[string]any{
        "driver": "custom",
        "connection": "default",
        "queue": "default",
        "via": func() (queue.Driver, error) {
            return redisfacades.Queue("redis") // The ` + "`redis`" + ` value is the key of ` + "`connections`" + `
        },
    }`
	sessionConfig = `map[string]any{
        "driver": "custom",
        "connection": "default",
        "via": func() (session.Driver, error) {
            return redisfacades.Session("redis")
        },
    }`
)

func main() {
	packages.Setup(os.Args).
		Install(
			// Add redis service provider to app.go
			modify.GoFile(path.Config("app.go")).
				Find(match.Imports()).Modify(modify.AddImport(packages.GetModulePath())).
				Find(match.Providers()).Modify(modify.Register("&redis.ServiceProvider{}")),

			// Add redis configuration to database.go
			modify.GoFile(path.Config("database.go")).
				Find(match.Config("database")).Modify(modify.AddConfig("redis", databaseConfig)),

			// Add redis cache configuration to cache.go if
			modify.GoFile(path.Config("cache.go")).
				Find(match.Imports()).Modify(modify.AddImport("github.com/goravel/framework/contracts/cache"), modify.AddImport("github.com/goravel/redis/facades", "redisfacades")).
				Find(match.Config("cache.stores")).Modify(modify.AddConfig("redis", cacheConfig)),

			modify.WhenFacade(facades.Queue,
				modify.GoFile(path.Config("queue.go")).
					Find(match.Imports()).Modify(modify.AddImport("github.com/goravel/framework/contracts/queue"), modify.AddImport("github.com/goravel/redis/facades", "redisfacades")).
					Find(match.Config("queue.connections")).Modify(modify.AddConfig("redis", queueConfig)),
			),
			modify.WhenFacade(facades.Session,
				modify.GoFile(path.Config("session.go")).
					Find(match.Imports()).Modify(modify.AddImport("github.com/goravel/framework/contracts/session"), modify.AddImport("github.com/goravel/redis/facades", "redisfacades")).
					Find(match.Config("session.drivers")).Modify(modify.AddConfig("redis", sessionConfig)),
			),
		).
		Uninstall(
			modify.GoFile(path.Config("app.go")).
				Find(match.Providers()).Modify(modify.Unregister("&redis.ServiceProvider{}")).
				Find(match.Imports()).Modify(modify.RemoveImport(packages.GetModulePath())),
			modify.WhenFacade(facades.Cache,
				modify.GoFile(path.Config("cache.go")).
					Find(match.Config("cache.stores")).Modify(modify.RemoveConfig("redis")).
					Find(match.Imports()).Modify(modify.RemoveImport("github.com/goravel/framework/contracts/cache"), modify.RemoveImport("github.com/goravel/redis/facades", "redisfacades")),
			),
			modify.GoFile(path.Config("queue.go")).
				Find(match.Config("queue.connections")).Modify(modify.RemoveConfig("redis")).
				Find(match.Imports()).Modify(modify.RemoveImport("github.com/goravel/framework/contracts/queue"), modify.RemoveImport("github.com/goravel/redis/facades", "redisfacades")),
			modify.GoFile(path.Config("session.go")).
				Find(match.Config("session.drivers")).Modify(modify.RemoveConfig("redis")).
				Find(match.Imports()).Modify(modify.RemoveImport("github.com/goravel/framework/contracts/session"), modify.RemoveImport("github.com/goravel/redis/facades", "redisfacades")),
			modify.GoFile(path.Config("database.go")).
				Find(match.Config("database")).Modify(modify.RemoveConfig("redis")),
		).
		Execute()
}
