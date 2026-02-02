package main

import (
	"os"

	"github.com/goravel/framework/packages"
	"github.com/goravel/framework/packages/match"
	"github.com/goravel/framework/packages/modify"
	"github.com/goravel/framework/support/env"
	"github.com/goravel/framework/support/file"
	"github.com/goravel/framework/support/path"
	supportstubs "github.com/goravel/framework/support/stubs"
)

func main() {
	setup := packages.Setup(os.Args)
	cacheConfig := `map[string]any{
        "driver": "custom",
        "connection": "default",
        "via": func() (cache.Driver, error) {
            return redisfacades.Cache("redis") // The ` + "`redis`" + ` value is the key of ` + "`stores`" + `
        },
    }`
	databaseConfig := `map[string]any{
        "default": map[string]any{
            "host":     config.Env("REDIS_HOST", ""),
            "password": config.Env("REDIS_PASSWORD", ""),
            "port":     config.Env("REDIS_PORT", 6379),
            "database": config.Env("REDIS_DB", 0),
        },
    }`
	queueConfig := `map[string]any{
        "driver": "custom",
        "connection": "default",
        "queue": "default",
        "via": func() (queue.Driver, error) {
            return redisfacades.Queue("redis") // The ` + "`redis`" + ` value is the key of ` + "`connections`" + `
        },
    }`
	sessionConfig := `map[string]any{
        "driver": "custom",
        "connection": "default",
        "via": func() (session.Driver, error) {
            return redisfacades.Session("redis")
        },
    }`

	appConfigPath := path.Config("app.go")
	databaseConfigPath := path.Config("database.go")
	cacheConfigPath := path.Config("cache.go")
	queueConfigPath := path.Config("queue.go")
	sessionConfigPath := path.Config("session.go")
	redisServiceProvider := "&redis.ServiceProvider{}"
	moduleImport := setup.Paths().Module().Import()
	configPackage := setup.Paths().Config().Package()
	facadesImport := setup.Paths().Facades().Import()
	facadesPackage := setup.Paths().Facades().Package()
	envPath := path.Base(".env")
	envExamplePath := path.Base(".env.example")
	cacheContract := "github.com/goravel/framework/contracts/cache"
	queueContract := "github.com/goravel/framework/contracts/queue"
	sessionContract := "github.com/goravel/framework/contracts/session"
	redisFacades := "github.com/goravel/redis/facades"
	envContent := `
REDIS_HOST=127.0.0.1
REDIS_PASSWORD=
REDIS_PORT=6379
`

	setup.Install(
		// Add redis service provider to app.go if not using bootstrap setup
		modify.When(func(_ map[string]any) bool {
			return !env.IsBootstrapSetup()
		}, modify.GoFile(appConfigPath).
			Find(match.Imports()).Modify(modify.AddImport(moduleImport)).
			Find(match.Providers()).Modify(modify.Register(redisServiceProvider))),

		// Add redis service provider to providers.go if using bootstrap setup
		modify.When(func(_ map[string]any) bool {
			return env.IsBootstrapSetup()
		}, modify.RegisterProvider(moduleImport, redisServiceProvider)),

		// Create config/database.go
		modify.WhenFileNotExists(databaseConfigPath, modify.File(databaseConfigPath).Overwrite(supportstubs.DatabaseConfig(configPackage, facadesImport, facadesPackage))),

		// Add redis configuration to database.go
		modify.GoFile(databaseConfigPath).
			Find(match.Config("database")).Modify(modify.AddConfig("redis", databaseConfig, "// Redis connections")),

		// Add redis cache configuration to cache.go if cache config file exists
		modify.WhenFileExists(cacheConfigPath,
			modify.GoFile(cacheConfigPath).
				Find(match.Imports()).Modify(modify.AddImport(cacheContract), modify.AddImport(redisFacades, "redisfacades")).
				Find(match.Config("cache.stores")).Modify(modify.AddConfig("redis", cacheConfig)).
				Find(match.Config("cache")).Modify(modify.AddConfig("default", `"redis"`)),
		),

		// Add redis queue configuration to queue.go if queue config file exists
		modify.WhenFileExists(queueConfigPath,
			modify.GoFile(queueConfigPath).
				Find(match.Imports()).Modify(modify.AddImport(queueContract), modify.AddImport(redisFacades, "redisfacades")).
				Find(match.Config("queue.connections")).Modify(modify.AddConfig("redis", queueConfig)).
				Find(match.Config("queue")).Modify(modify.AddConfig("default", `"redis"`)),
		),

		// Add redis session configuration to session.go if session config file exists
		modify.WhenFileExists(sessionConfigPath,
			modify.GoFile(sessionConfigPath).
				Find(match.Imports()).Modify(modify.AddImport(sessionContract), modify.AddImport(redisFacades, "redisfacades")).
				Find(match.Config("session.drivers")).Modify(modify.AddConfig("redis", sessionConfig)).
				Find(match.Config("session")).Modify(modify.AddConfig("default", `"redis"`)),
		),

		// Add configurations to the .env and .env.example files
		modify.WhenFileNotContains(envPath, "REDIS_HOST", modify.File(envPath).Append(envContent)),
		modify.WhenFileNotContains(envExamplePath, "REDIS_HOST", modify.File(envExamplePath).Append(envContent)),
	).Uninstall(
		// Remove redis service provider from app.go if not using bootstrap setup
		modify.When(func(_ map[string]any) bool {
			return !env.IsBootstrapSetup()
		}, modify.GoFile(appConfigPath).
			Find(match.Providers()).Modify(modify.Unregister(redisServiceProvider)).
			Find(match.Imports()).Modify(modify.RemoveImport(moduleImport))),

		// Remove redis service provider from providers.go if using bootstrap setup
		modify.When(func(_ map[string]any) bool {
			return env.IsBootstrapSetup()
		}, modify.UnregisterProvider(moduleImport, redisServiceProvider)),

		// Remove redis configuration from cache.go if cache config file exists
		modify.WhenFileExists(cacheConfigPath,
			modify.GoFile(cacheConfigPath).
				Find(match.Config("cache")).Modify(modify.AddConfig("default", `"memory"`)).
				Find(match.Config("cache.stores")).Modify(modify.RemoveConfig("redis")).
				Find(match.Imports()).Modify(modify.RemoveImport(cacheContract), modify.RemoveImport(redisFacades, "redisfacades")),
		),

		// Remove redis configuration from queue.go if queue config file exists
		modify.WhenFileExists(queueConfigPath,
			modify.GoFile(queueConfigPath).
				Find(match.Config("queue")).Modify(modify.AddConfig("default", `"sync"`)).
				Find(match.Config("queue.connections")).Modify(modify.RemoveConfig("redis")).
				Find(match.Imports()).Modify(modify.RemoveImport(queueContract), modify.RemoveImport(redisFacades, "redisfacades")),
		),

		// Remove redis configuration from session.go if session config file exists
		modify.WhenFileExists(sessionConfigPath,
			modify.GoFile(sessionConfigPath).
				Find(match.Config("session")).Modify(modify.AddConfig("default", `"file"`)).
				Find(match.Config("session.drivers")).Modify(modify.RemoveConfig("redis")).
				Find(match.Imports()).Modify(modify.RemoveImport(sessionContract), modify.RemoveImport(redisFacades, "redisfacades")),
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
			return content == supportstubs.DatabaseConfig(configPackage, facadesImport, facadesPackage)
		}, modify.File(databaseConfigPath).Remove()),
	).Execute()
}
