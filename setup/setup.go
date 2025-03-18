package main

import (
	"os"

	contractspackages "github.com/goravel/framework/contracts/packages"
	"github.com/goravel/framework/packages"
	"github.com/goravel/framework/support/path"
)

func main() {
	setup := packages.Setup(os.Args)
	setup.Install(
		packages.ModifyGoFile{
			File: path.Config("app.go"),
			Modifiers: []contractspackages.GoNodeModifier{
				packages.AddImportSpec(setup.Module),
				packages.AddProviderSpecBefore(
					"&redis.ServiceProvider{}",
					"&cache.ServiceProvider{}",
				),
			},
		},
		packages.ModifyGoFile{
			File: path.Config("cache.go"),
			Modifiers: []contractspackages.GoNodeModifier{
				packages.AddImportSpec("github.com/goravel/framework/contracts/cache"),
				packages.AddImportSpec("github.com/goravel/redis/facades", "redisfacades"),
				packages.AddConfigSpec("cache.stores", "redis", `
map[string]any{
        "driver": "custom",
        "connection": "default",
        "via": func() (cache.Driver, error) {
                return redisfacades.Cache("redis")
        },
}
`),
			},
		},
		packages.ModifyGoFile{
			File: path.Config("queue.go"),
			Modifiers: []contractspackages.GoNodeModifier{
				packages.AddImportSpec("github.com/goravel/framework/contracts/queue"),
				packages.AddImportSpec("github.com/goravel/redis/facades", "redisfacades"),
				packages.AddConfigSpec("queue.connections", "redis", `
map[string]any{
        "driver": "custom",
        "connection": "default",
        "via": func() (queue.Driver, error) {
                return redisfacades.Queue("redis")
        },
}`),
			},
		})

	setup.Uninstall(
		packages.ModifyGoFile{
			File: path.Config("app.go"),
			Modifiers: []contractspackages.GoNodeModifier{
				packages.RemoveImportSpec(setup.Module),
				packages.RemoveProviderSpec("&redis.ServiceProvider{}"),
			},
		},
		packages.ModifyGoFile{
			File: path.Config("cache.go"),
			Modifiers: []contractspackages.GoNodeModifier{
				packages.RemoveImportSpec("github.com/goravel/framework/contracts/cache"),
				packages.RemoveImportSpec("github.com/goravel/redis/facades", "redisfacades"),
				packages.RemoveConfigSpec("cache.stores.redis"),
			},
		},
		packages.ModifyGoFile{
			File: path.Config("queue.go"),
			Modifiers: []contractspackages.GoNodeModifier{
				packages.RemoveImportSpec("github.com/goravel/framework/contracts/queue"),
				packages.RemoveImportSpec("github.com/goravel/redis/facades", "redisfacades"),
				packages.RemoveConfigSpec("queue.connections.redis"),
			},
		})

	setup.Execute()
}
