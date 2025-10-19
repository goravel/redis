package main

import (
	"os"

	"github.com/goravel/framework/packages"
	"github.com/goravel/framework/packages/match"
	"github.com/goravel/framework/packages/modify"
	"github.com/goravel/framework/support/file"
	"github.com/goravel/framework/support/path"
	supportstubs "github.com/goravel/framework/support/stubs"
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
	appConfigPath := path.Config("app.go")
	databaseConfigPath := path.Config("database.go")
	cacheConfigPath := path.Config("cache.go")
	queueConfigPath := path.Config("queue.go")
	sessionConfigPath := path.Config("session.go")
	redisServiceProvider := "&redis.ServiceProvider{}"
	modulePath := packages.GetModulePath()
	moduleName := packages.GetModuleNameFromArgs(os.Args)

	packages.Setup(os.Args).
		Install(
			// Add redis service provider to app.go
			modify.GoFile(appConfigPath).
				Find(match.Imports()).Modify(modify.AddImport(modulePath)).
				Find(match.Providers()).Modify(modify.Register(redisServiceProvider)),

			// Create config/database.go
			modify.WhenFileNotExists(databaseConfigPath, modify.File(databaseConfigPath).Overwrite(supportstubs.DatabaseConfig(moduleName))),

			// Add redis configuration to database.go
			modify.GoFile(databaseConfigPath).
				Find(match.Config("database")).Modify(modify.AddConfig("redis", databaseConfig, "// Redis connections")),

			// Add redis cache configuration to cache.go if cache config file exists
			modify.WhenFileExists(cacheConfigPath,
				modify.GoFile(cacheConfigPath).
					Find(match.Imports()).Modify(modify.AddImport("github.com/goravel/framework/contracts/cache"), modify.AddImport("github.com/goravel/redis/facades", "redisfacades")).
					Find(match.Config("cache.stores")).Modify(modify.AddConfig("redis", cacheConfig)),
			),

			// Add redis queue configuration to queue.go if queue config file exists
			modify.WhenFileExists(queueConfigPath,
				modify.GoFile(queueConfigPath).
					Find(match.Imports()).Modify(modify.AddImport("github.com/goravel/framework/contracts/queue"), modify.AddImport("github.com/goravel/redis/facades", "redisfacades")).
					Find(match.Config("queue.connections")).Modify(modify.AddConfig("redis", queueConfig)),
			),

			// Add redis session configuration to session.go if session config file exists
			modify.WhenFileExists(sessionConfigPath,
				modify.GoFile(sessionConfigPath).
					Find(match.Imports()).Modify(modify.AddImport("github.com/goravel/framework/contracts/session"), modify.AddImport("github.com/goravel/redis/facades", "redisfacades")).
					Find(match.Config("session.drivers")).Modify(modify.AddConfig("redis", sessionConfig)),
			),
		).
		Uninstall(
			// Remove redis service provider from app.go
			modify.GoFile(appConfigPath).
				Find(match.Providers()).Modify(modify.Unregister(redisServiceProvider)).
				Find(match.Imports()).Modify(modify.RemoveImport(modulePath)),

			// Remove redis configuration from cache.go if cache config file exists
			modify.WhenFileExists(cacheConfigPath,
				modify.GoFile(cacheConfigPath).
					Find(match.Config("cache.stores")).Modify(modify.RemoveConfig("redis")).
					Find(match.Imports()).Modify(modify.RemoveImport("github.com/goravel/framework/contracts/cache"), modify.RemoveImport("github.com/goravel/redis/facades", "redisfacades")),
			),

			// Remove redis configuration from queue.go if queue config file exists
			modify.WhenFileExists(queueConfigPath,
				modify.GoFile(queueConfigPath).
					Find(match.Config("queue.connections")).Modify(modify.RemoveConfig("redis")).
					Find(match.Imports()).Modify(modify.RemoveImport("github.com/goravel/framework/contracts/queue"), modify.RemoveImport("github.com/goravel/redis/facades", "redisfacades")),
			),

			// Remove redis configuration from session.go if session config file exists
			modify.WhenFileExists(sessionConfigPath,
				modify.GoFile(sessionConfigPath).
					Find(match.Config("session.drivers")).Modify(modify.RemoveConfig("redis")).
					Find(match.Imports()).Modify(modify.RemoveImport("github.com/goravel/framework/contracts/session"), modify.RemoveImport("github.com/goravel/redis/facades", "redisfacades")),
			),

			// Remove redis configuration from database.go
			modify.GoFile(databaseConfigPath).
				Find(match.Config("database")).Modify(modify.RemoveConfig("redis")),

			// Remove config/database.go
			modify.When(func(_ map[string]any) bool {
				content, err := file.GetContent(databaseConfigPath)
				if err != nil {
					return false
				}
				return content == supportstubs.DatabaseConfig(moduleName)
			}, modify.File(databaseConfigPath).Remove()),
		).
		Execute()
}
