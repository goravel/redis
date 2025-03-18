package main

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	pkgcontracts "github.com/goravel/framework/contracts/packages"
	"github.com/goravel/framework/packages"
	"github.com/goravel/framework/support/color"
	"github.com/goravel/framework/support/path"
)

func main() {
	info, ok := debug.ReadBuildInfo()
	if !ok || !strings.HasSuffix(info.Path, "setup") {
		color.Errorln("Package module name is empty, please run command with module name.")
		return
	}
	module := filepath.Dir(info.Path)
	force := len(os.Args) == 3 && (os.Args[2] == "--force" || os.Args[2] == "-f")

	var pkg = &packages.Setup{
		Force:  force,
		Module: module,
		OnInstall: []pkgcontracts.FileModifier{
			packages.ModifyGoFile{
				File: path.Config("app.go"),
				Modifiers: []pkgcontracts.GoNodeModifier{
					packages.AddImportSpec(module),
					packages.AddProviderSpecBefore(
						"&redis.ServiceProvider{}",
						"&cache.ServiceProvider{}",
					),
				},
			},
			packages.ModifyGoFile{
				File: path.Config("cache.go"),
				Modifiers: []pkgcontracts.GoNodeModifier{
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
				Modifiers: []pkgcontracts.GoNodeModifier{
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
			},
		},
		OnUninstall: []pkgcontracts.FileModifier{
			packages.ModifyGoFile{
				File: path.Config("app.go"),
				Modifiers: []pkgcontracts.GoNodeModifier{
					packages.RemoveImportSpec(module),
					packages.RemoveProviderSpec("&redis.ServiceProvider{}"),
				},
			},
			packages.ModifyGoFile{
				File: path.Config("cache.go"),
				Modifiers: []pkgcontracts.GoNodeModifier{
					packages.RemoveImportSpec("github.com/goravel/framework/contracts/cache"),
					packages.RemoveImportSpec("github.com/goravel/redis/facades", "redisfacades"),
					packages.RemoveConfigSpec("cache.stores.redis"),
				},
			},
			packages.ModifyGoFile{
				File: path.Config("queue.go"),
				Modifiers: []pkgcontracts.GoNodeModifier{
					packages.RemoveImportSpec("github.com/goravel/framework/contracts/queue"),
					packages.RemoveImportSpec("github.com/goravel/redis/facades", "redisfacades"),
					packages.RemoveConfigSpec("queue.connections.redis"),
				},
			},
		},
	}

	if len(os.Args) > 1 {
		execute(pkg, os.Args[1])
	}
}

func execute(pkg pkgcontracts.Setup, command string) {
	var err error
	switch command {
	case "install":
		err = pkg.Install()
	case "uninstall":
		err = pkg.Uninstall()
	default:
		return
	}

	if err != nil {
		color.Errorln(err)
		return
	}

	color.Successf("Package %sed successfully\n", command)
}
