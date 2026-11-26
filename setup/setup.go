package main

import (
	"os"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"github.com/subosito/gotenv"

	contractspackages "github.com/goravel/framework/contracts/packages"
	contractsmodify "github.com/goravel/framework/contracts/packages/modify"
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

	setup.Install(installActions(setup)...).Uninstall(uninstallActions(setup)...).Execute()
}

func installActions(setup contractspackages.Setup) []contractsmodify.Apply {
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
        "retry_after": 60,
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

	return []contractsmodify.Apply{
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
				Find(match.Config("cache")).Modify(modify.AddConfig("default", `config.Env("CACHE_STORE", "redis")`)),

			// Record the selector in the .env and .env.example files
			modify.WhenFileExists(envPath, modify.Env(envPath, "CACHE_STORE", "redis")),
			modify.WhenFileExists(envExamplePath, modify.Env(envExamplePath, "CACHE_STORE", "redis")),
		),

		// Add redis queue configuration to queue.go if queue config file exists
		modify.WhenFileExists(queueConfigPath,
			modify.GoFile(queueConfigPath).
				Find(match.Imports()).Modify(modify.AddImport(queueContract), modify.AddImport(redisFacades, "redisfacades")).
				Find(match.Config("queue.connections")).Modify(modify.AddConfig("redis", queueConfig)).
				Find(match.Config("queue")).Modify(modify.AddConfig("default", `config.Env("QUEUE_CONNECTION", "redis")`)),

			// Record the selector in the .env and .env.example files
			modify.WhenFileExists(envPath, modify.Env(envPath, "QUEUE_CONNECTION", "redis")),
			modify.WhenFileExists(envExamplePath, modify.Env(envExamplePath, "QUEUE_CONNECTION", "redis")),
		),

		// Add redis session configuration to session.go if session config file exists
		modify.WhenFileExists(sessionConfigPath,
			modify.GoFile(sessionConfigPath).
				Find(match.Imports()).Modify(modify.AddImport(sessionContract), modify.AddImport(redisFacades, "redisfacades")).
				Find(match.Config("session.drivers")).Modify(modify.AddConfig("redis", sessionConfig)).
				Find(match.Config("session")).Modify(modify.AddConfig("default", `config.Env("SESSION_DRIVER", "redis")`)),

			// Record the selector in the .env and .env.example files
			modify.WhenFileExists(envPath, modify.Env(envPath, "SESSION_DRIVER", "redis")),
			modify.WhenFileExists(envExamplePath, modify.Env(envExamplePath, "SESSION_DRIVER", "redis")),
		),

		// Add the missing REDIS_* keys to the .env and .env.example files
		modify.WhenFileExists(envPath, modify.EnvWhenMissing(envPath, "REDIS_HOST", "127.0.0.1")),
		modify.WhenFileExists(envPath, modify.EnvWhenMissing(envPath, "REDIS_PASSWORD", "")),
		modify.WhenFileExists(envPath, modify.EnvWhenMissing(envPath, "REDIS_PORT", "6379")),
		modify.WhenFileExists(envExamplePath, modify.EnvWhenMissing(envExamplePath, "REDIS_HOST", "127.0.0.1")),
		modify.WhenFileExists(envExamplePath, modify.EnvWhenMissing(envExamplePath, "REDIS_PASSWORD", "")),
		modify.WhenFileExists(envExamplePath, modify.EnvWhenMissing(envExamplePath, "REDIS_PORT", "6379")),
	}
}

func uninstallActions(setup contractspackages.Setup) []contractsmodify.Apply {
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

	return []contractsmodify.Apply{
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
				Find(match.Config("cache")).Modify(resetDefault("CACHE_STORE", "memory")).
				Find(match.Config("cache.stores")).Modify(modify.RemoveConfig("redis")).
				Find(match.Imports()).Modify(modify.RemoveImport(cacheContract), modify.RemoveImport(redisFacades, "redisfacades")),

			// Restore the framework default only when the selector names redis
			restoreEnvDefault(envPath, "CACHE_STORE", "memory"),
			restoreEnvDefault(envExamplePath, "CACHE_STORE", "memory"),
		),

		// Remove redis configuration from queue.go if queue config file exists
		modify.WhenFileExists(queueConfigPath,
			modify.GoFile(queueConfigPath).
				Find(match.Config("queue")).Modify(resetDefault("QUEUE_CONNECTION", "sync")).
				Find(match.Config("queue.connections")).Modify(modify.RemoveConfig("redis")).
				Find(match.Imports()).Modify(modify.RemoveImport(queueContract), modify.RemoveImport(redisFacades, "redisfacades")),

			// Restore the framework default only when the selector names redis
			restoreEnvDefault(envPath, "QUEUE_CONNECTION", "sync"),
			restoreEnvDefault(envExamplePath, "QUEUE_CONNECTION", "sync"),
		),

		// Remove redis configuration from session.go if session config file exists
		modify.WhenFileExists(sessionConfigPath,
			modify.GoFile(sessionConfigPath).
				Find(match.Config("session")).Modify(resetDefault("SESSION_DRIVER", "file")).
				Find(match.Config("session.drivers")).Modify(modify.RemoveConfig("redis")).
				Find(match.Imports()).Modify(modify.RemoveImport(sessionContract), modify.RemoveImport(redisFacades, "redisfacades")),

			// Restore the framework default only when the selector names redis
			restoreEnvDefault(envPath, "SESSION_DRIVER", "file"),
			restoreEnvDefault(envExamplePath, "SESSION_DRIVER", "file"),
		),

		// Remove redis configuration from database.go
		modify.GoFile(databaseConfigPath).
			Find(match.Config("database")).Modify(modify.RemoveConfig("redis")),

		// Remove config/database.go
		modify.When(func(_ map[string]any) bool {
			content, err := file.GetContent(databaseConfigPath)
			return err == nil && content == supportstubs.DatabaseConfig(configPackage, facadesImport, facadesPackage)
		}, modify.File(databaseConfigPath).Remove()),
	}
}

// resetDefault rewrites the "default" key of a component configuration to the
// framework default when the key still names redis. It keeps the decorations of
// the key, so the comment above the selector survives.
func resetDefault(envKey, frameworkDefault string) contractsmodify.Action {
	return func(cursor *dstutil.Cursor) {
		node := cursor.Node().(*dst.CallExpr)
		value := node.Args[1].(*dst.CompositeLit)

		index := modify.KeyIndex(value.Elts, modify.MustParseExpr(`"default"`).(dst.Expr))
		if index < 0 {
			return
		}

		kv := value.Elts[index].(*dst.KeyValueExpr)
		if !isRedisValue(kv.Value, envKey) {
			return
		}

		kv.Value = modify.MustParseExpr(`config.Env("` + envKey + `", "` + frameworkDefault + `")`).(dst.Expr)
	}
}

// isRedisValue reports whether the expression is the redis driver value, either
// hardcoded or as the fallback of an environment expression.
func isRedisValue(expr dst.Expr, envKey string) bool {
	return match.EqualNode(modify.MustParseExpr(`"redis"`)).MatchNode(expr) ||
		match.EqualNode(modify.MustParseExpr(`config.Env("`+envKey+`", "redis")`)).MatchNode(expr)
}

// restoreEnvDefault restores the framework default in the environment file when
// the selector names redis. A missing file is skipped and a failed read is
// returned.
func restoreEnvDefault(envFile, key, frameworkDefault string) contractsmodify.Apply {
	return modify.WhenFileExists(
		envFile,
		modify.Call(func(_ []contractsmodify.Option) error {
			content, err := file.GetContent(envFile)
			if err != nil {
				return err
			}

			if !envValueIs(content, key, "redis") {
				return nil
			}

			return modify.Env(envFile, key, frameworkDefault).Apply()
		}),
	)
}

// envValueIs reports whether the environment file assigns key the value. It
// reads the file with the parser the application uses, so quotes, comments,
// variable references and duplicate assignments behave the same way.
func envValueIs(content, key, value string) bool {
	env, err := gotenv.StrictParse(strings.NewReader(content))
	if err != nil {
		return false
	}

	return env[key] == value
}
