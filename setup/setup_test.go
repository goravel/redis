package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goravel/framework/packages"
	"github.com/goravel/framework/support"
)

const appStub = `package bootstrap

import "github.com/goravel/framework/foundation"

func Boot() {
	foundation.Setup().WithProviders(Providers).Create()
}
`

const providersStub = `package bootstrap

import "github.com/goravel/framework/contracts/foundation"

func Providers() []foundation.ServiceProvider {
	return []foundation.ServiceProvider{}
}
`

const cacheConfigStub = `package config

import (
	"github.com/goravel/framework/facades"
)

func init() {
	config := facades.Config()
	config.Add("cache", map[string]any{
		// Default Cache Store
		"default": config.Env("CACHE_STORE", "memory"),

		// Cache Stores
		"stores": map[string]any{
			"memory": map[string]any{
				"driver": "memory",
			},
		},
	})
}
`

const queueConfigStub = `package config

import (
	"github.com/goravel/framework/facades"
)

func init() {
	config := facades.Config()
	config.Add("queue", map[string]any{
		// Default Queue Connection Name
		"default": config.Env("QUEUE_CONNECTION", "sync"),

		// Queue Connections
		"connections": map[string]any{
			"sync": map[string]any{
				"driver": "sync",
			},
		},
	})
}
`

const sessionConfigStub = `package config

import (
	"github.com/goravel/framework/facades"
)

func init() {
	config := facades.Config()
	config.Add("session", map[string]any{
		// Default Session Driver
		"default": config.Env("SESSION_DRIVER", "file"),

		// Session Drivers
		"drivers": map[string]any{
			"file": map[string]any{
				"driver": "file",
			},
		},
	})
}
`

const databaseConfigStub = `package config

import (
	"github.com/goravel/framework/facades"
)

func init() {
	config := facades.Config()
	config.Add("database", map[string]any{
		// Default Database Connection Name
		"default": "",
	})
}
`

type component struct {
	name       string
	configPath string
	key        string
	fallback   string
	comment    string
	stub       string
}

func components() []component {
	return []component{
		{
			name:       "cache",
			configPath: filepath.Join("config", "cache.go"),
			key:        "CACHE_STORE",
			fallback:   "memory",
			comment:    "// Default Cache Store",
			stub:       cacheConfigStub,
		},
		{
			name:       "queue",
			configPath: filepath.Join("config", "queue.go"),
			key:        "QUEUE_CONNECTION",
			fallback:   "sync",
			comment:    "// Default Queue Connection Name",
			stub:       queueConfigStub,
		},
		{
			name:       "session",
			configPath: filepath.Join("config", "session.go"),
			key:        "SESSION_DRIVER",
			fallback:   "file",
			comment:    "// Default Session Driver",
			stub:       sessionConfigStub,
		},
	}
}

// setupProject points the project root at a temporary application skeleton.
func setupProject(t *testing.T) string {
	t.Helper()

	relativePath := support.RelativePath
	t.Cleanup(func() { support.RelativePath = relativePath })
	support.RelativePath = t.TempDir()

	writeFile(t, filepath.Join(support.RelativePath, "bootstrap", "app.go"), appStub)
	writeFile(t, filepath.Join(support.RelativePath, "bootstrap", "providers.go"), providersStub)
	writeFile(t, filepath.Join(support.RelativePath, "config", "cache.go"), cacheConfigStub)
	writeFile(t, filepath.Join(support.RelativePath, "config", "queue.go"), queueConfigStub)
	writeFile(t, filepath.Join(support.RelativePath, "config", "session.go"), sessionConfigStub)
	writeFile(t, filepath.Join(support.RelativePath, "config", "database.go"), databaseConfigStub)

	return support.RelativePath
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func writeEnv(t *testing.T, root, content string) {
	t.Helper()

	writeFile(t, filepath.Join(root, ".env"), content)
	writeFile(t, filepath.Join(root, ".env.example"), content)
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	return string(content)
}

// install applies the install actions with the installer options. An external
// driver install passes no facade and no driver.
func install(t *testing.T) error {
	t.Helper()

	setup := packages.Setup([]string{"install", "--main-path=goravel"})
	for _, action := range installActions(setup) {
		if err := action.Apply(); err != nil {
			return err
		}
	}

	return nil
}

// uninstall applies the uninstall actions with the installer options.
func uninstall(t *testing.T) error {
	t.Helper()

	setup := packages.Setup([]string{"uninstall", "--main-path=goravel"})
	for _, action := range uninstallActions(setup) {
		if err := action.Apply(); err != nil {
			return err
		}
	}

	return nil
}

func TestInstallActions_Selector(t *testing.T) {
	root := setupProject(t)
	writeEnv(t, root, "APP_ENV=local")

	require.NoError(t, install(t))

	for _, c := range components() {
		t.Run(c.name, func(t *testing.T) {
			content := readFile(t, filepath.Join(root, c.configPath))
			assert.Contains(t, content, `"default": config.Env("`+c.key+`", "redis"),`)
			assert.Contains(t, content, c.comment)
			assert.NotContains(t, content, `"default": "redis",`)

			env := readFile(t, filepath.Join(root, ".env"))
			assert.Contains(t, env, c.key+"=redis")
			assert.Equal(t, 1, strings.Count(env, c.key+"="))
			assert.Equal(t, env, readFile(t, filepath.Join(root, ".env.example")))
		})
	}
}

func TestInstallActions_LegacyDefault(t *testing.T) {
	for _, c := range components() {
		t.Run(c.name, func(t *testing.T) {
			root := setupProject(t)
			writeEnv(t, root, "APP_ENV=local")

			configPath := filepath.Join(root, c.configPath)
			writeFile(t, configPath, strings.Replace(c.stub, `config.Env("`+c.key+`", "`+c.fallback+`")`, `"redis"`, 1))

			require.NoError(t, install(t))

			content := readFile(t, configPath)
			assert.Contains(t, content, `"default": config.Env("`+c.key+`", "redis"),`)
			assert.Contains(t, content, c.comment)
			assert.NotContains(t, content, `"default": "redis",`)
		})
	}
}

func TestInstallActions_EnvAbsent(t *testing.T) {
	root := setupProject(t)

	require.NoError(t, install(t))

	assert.NoFileExists(t, filepath.Join(root, ".env"))
	assert.NoFileExists(t, filepath.Join(root, ".env.example"))
}

func TestInstallActions_RedisEnv(t *testing.T) {
	tests := []struct {
		name     string
		env      string
		expected []string
		counts   map[string]int
	}{
		{
			name:     "appends the missing keys",
			env:      "APP_ENV=local",
			expected: []string{"REDIS_HOST=127.0.0.1", "REDIS_PASSWORD=", "REDIS_PORT=6379"},
			counts:   map[string]int{"REDIS_HOST=": 1, "REDIS_PASSWORD=": 1, "REDIS_PORT=": 1},
		},
		{
			name:     "a commented key does not block the append",
			env:      "# REDIS_HOST=127.0.0.1",
			expected: []string{"# REDIS_HOST=127.0.0.1", "REDIS_HOST=127.0.0.1", "REDIS_PASSWORD=", "REDIS_PORT=6379"},
			counts:   map[string]int{"REDIS_HOST=": 2, "REDIS_PASSWORD=": 1, "REDIS_PORT=": 1},
		},
		{
			name:     "does not duplicate an existing key",
			env:      "REDIS_PORT=6380",
			expected: []string{"REDIS_HOST=127.0.0.1", "REDIS_PASSWORD=", "REDIS_PORT=6380"},
			counts:   map[string]int{"REDIS_HOST=": 1, "REDIS_PASSWORD=": 1, "REDIS_PORT=": 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := setupProject(t)
			writeEnv(t, root, tt.env)

			require.NoError(t, install(t))

			env := readFile(t, filepath.Join(root, ".env"))
			for _, line := range tt.expected {
				assert.Contains(t, env, line)
			}
			for key, count := range tt.counts {
				assert.Equal(t, count, strings.Count(env, key))
			}
			assert.Equal(t, env, readFile(t, filepath.Join(root, ".env.example")))
		})
	}
}

func TestUninstallActions_EnvBackedDefault(t *testing.T) {
	root := setupProject(t)
	writeEnv(t, root, "APP_ENV=local")
	require.NoError(t, install(t))

	require.NoError(t, uninstall(t))

	for _, c := range components() {
		t.Run(c.name, func(t *testing.T) {
			content := readFile(t, filepath.Join(root, c.configPath))
			assert.Contains(t, content, `"default": config.Env("`+c.key+`", "`+c.fallback+`"),`)
			assert.NotContains(t, content, `config.Env("`+c.key+`", "redis")`)
			assert.Contains(t, content, c.comment)
			assert.NotContains(t, content, "redisfacades")

			env := readFile(t, filepath.Join(root, ".env"))
			assert.Contains(t, env, c.key+"="+c.fallback)
			assert.NotContains(t, env, c.key+"=redis")
		})
	}
}

func TestUninstallActions_HardcodedDefault(t *testing.T) {
	for _, c := range components() {
		t.Run(c.name, func(t *testing.T) {
			root := setupProject(t)
			writeEnv(t, root, c.key+"=redis")

			configPath := filepath.Join(root, c.configPath)
			writeFile(t, configPath, strings.Replace(c.stub, `config.Env("`+c.key+`", "`+c.fallback+`")`, `"redis"`, 1))

			require.NoError(t, uninstall(t))

			content := readFile(t, configPath)
			assert.Contains(t, content, `"default": config.Env("`+c.key+`", "`+c.fallback+`"),`)
			assert.NotContains(t, content, `"default": "redis",`)

			env := readFile(t, filepath.Join(root, ".env"))
			assert.Contains(t, env, c.key+"="+c.fallback)
		})
	}
}

func TestUninstallActions_EnvReadError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read a file without permission bits")
	}

	root := setupProject(t)
	writeEnv(t, root, "CACHE_STORE=redis")

	envPath := filepath.Join(root, ".env")
	require.NoError(t, os.Chmod(envPath, 0o000))
	t.Cleanup(func() {
		require.NoError(t, os.Chmod(envPath, 0o644))
	})

	assert.Error(t, uninstall(t))
}

func TestUninstallActions_OtherDriver(t *testing.T) {
	root := setupProject(t)
	writeEnv(t, root, "APP_ENV=local")
	require.NoError(t, install(t))

	cachePath := filepath.Join(root, "config", "cache.go")
	content := readFile(t, cachePath)
	writeFile(t, cachePath, strings.Replace(content, `config.Env("CACHE_STORE", "redis")`, `config.Env("CACHE_STORE", "memory")`, 1))
	writeEnv(t, root, "APP_ENV=local\nCACHE_STORE=memory")

	require.NoError(t, uninstall(t))

	content = readFile(t, cachePath)
	assert.Contains(t, content, `"default": config.Env("CACHE_STORE", "memory"),`)
	assert.NotContains(t, content, `config.Env("CACHE_STORE", "sync")`)
	assert.NotContains(t, content, `"redis": map[string]any{`)
	assert.NotContains(t, content, "redisfacades")
	assert.NotContains(t, content, "contracts/cache")

	env := readFile(t, filepath.Join(root, ".env"))
	assert.Contains(t, env, "CACHE_STORE=memory")
	assert.NotContains(t, env, "CACHE_STORE=sync")
}

func TestUninstallActions_CustomStoreKeepsImport(t *testing.T) {
	root := setupProject(t)
	writeEnv(t, root, "APP_ENV=local")
	require.NoError(t, install(t))

	cachePath := filepath.Join(root, "config", "cache.go")
	customStore := "\n\t\t\t\"custom\": map[string]any{\n" +
		"\t\t\t\t\"driver\": \"custom\",\n" +
		"\t\t\t\t\"via\": func() (cache.Driver, error) {\n" +
		"\t\t\t\t\treturn nil, nil\n" +
		"\t\t\t\t},\n" +
		"\t\t\t},"
	content := strings.Replace(readFile(t, cachePath), `"stores": map[string]any{`, `"stores": map[string]any{`+customStore, 1)
	writeFile(t, cachePath, content)

	require.NoError(t, uninstall(t))

	content = readFile(t, cachePath)
	assert.Contains(t, content, `"custom": map[string]any{`)
	assert.Contains(t, content, "cache.Driver")
	assert.Contains(t, content, `"github.com/goravel/framework/contracts/cache"`)
	assert.NotContains(t, content, `"redis": map[string]any{`)
}

func TestInstallActions_Idempotent(t *testing.T) {
	root := setupProject(t)
	writeEnv(t, root, "APP_ENV=local")

	require.NoError(t, install(t))

	files := []string{".env", ".env.example", "config/cache.go", "config/queue.go", "config/session.go", "config/database.go"}
	first := map[string]string{}
	for _, name := range files {
		first[name] = readFile(t, filepath.Join(root, name))
	}

	require.NoError(t, install(t))

	for name, content := range first {
		assert.Equal(t, content, readFile(t, filepath.Join(root, name)), name)
	}
	assert.Equal(t, 1, strings.Count(readFile(t, filepath.Join(root, ".env")), "REDIS_HOST="))
}

func TestUninstallActions_Idempotent(t *testing.T) {
	root := setupProject(t)
	writeEnv(t, root, "APP_ENV=local")
	require.NoError(t, install(t))
	require.NoError(t, uninstall(t))

	files := []string{".env", ".env.example", "config/cache.go", "config/queue.go", "config/session.go", "config/database.go"}
	first := map[string]string{}
	for _, name := range files {
		first[name] = readFile(t, filepath.Join(root, name))
	}

	require.NoError(t, uninstall(t))

	for name, content := range first {
		assert.Equal(t, content, readFile(t, filepath.Join(root, name)), name)
	}
}

func TestUninstallActions_DuplicateEnvKeys(t *testing.T) {
	tests := []struct {
		name     string
		env      string
		expected string
	}{
		{
			name:     "redis is the last assignment",
			env:      "CACHE_STORE=memory\nCACHE_STORE=redis",
			expected: "CACHE_STORE=memory\nCACHE_STORE=memory",
		},
		{
			name:     "memory is the last assignment",
			env:      "CACHE_STORE=redis\nCACHE_STORE=memory",
			expected: "CACHE_STORE=redis\nCACHE_STORE=memory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := setupProject(t)
			writeEnv(t, root, "APP_ENV=local")
			require.NoError(t, install(t))

			// The dotenv parser keeps the last assignment of a key.
			writeEnv(t, root, tt.env)

			require.NoError(t, uninstall(t))

			env := readFile(t, filepath.Join(root, ".env"))
			assert.Equal(t, tt.expected, env)
			assert.Equal(t, env, readFile(t, filepath.Join(root, ".env.example")))
		})
	}
}

func TestUninstallActions_EnvValueForms(t *testing.T) {
	// An empty expected value means the file is left unchanged.
	tests := []struct {
		name     string
		env      string
		expected string
	}{
		{name: "double quotes", env: `CACHE_STORE="redis"`, expected: "CACHE_STORE=memory"},
		{name: "single quotes", env: `CACHE_STORE='redis'`, expected: "CACHE_STORE=memory"},
		{name: "trailing comment", env: "CACHE_STORE=redis # cache", expected: "CACHE_STORE=memory"},
		{name: "export prefix", env: "export CACHE_STORE=redis", expected: "export CACHE_STORE=memory"},
		{name: "colon separator", env: "CACHE_STORE: redis", expected: "CACHE_STORE=memory"},
		{name: "variable reference", env: "DRIVER=redis\nCACHE_STORE=$DRIVER", expected: "DRIVER=redis\nCACHE_STORE=memory"},
		{name: "unterminated quote", env: `CACHE_STORE="redis`},
		{name: "quoted value with a suffix", env: `CACHE_STORE="redis"suffix`},
		{name: "colon with no value", env: "CACHE_STORE:"},
		{name: "colon with a blank value", env: "CACHE_STORE: "},
		{name: "line without a delimiter", env: "CACHE_STORE=redis\nINVALID LINE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := setupProject(t)
			writeEnv(t, root, "APP_ENV=local")
			require.NoError(t, install(t))

			writeEnv(t, root, tt.env)

			require.NoError(t, uninstall(t))

			expected := tt.expected
			if expected == "" {
				expected = tt.env
			}
			assert.Equal(t, expected, readFile(t, filepath.Join(root, ".env")))
		})
	}
}

func TestMain_Install(t *testing.T) {
	root := setupProject(t)
	writeEnv(t, root, "APP_ENV=local")

	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	os.Args = []string{"setup", "install", "--main-path=goravel"}

	main()

	assert.Contains(t, readFile(t, filepath.Join(root, ".env")), "REDIS_HOST=127.0.0.1")
	assert.Contains(t, readFile(t, filepath.Join(root, "config", "cache.go")), `config.Env("CACHE_STORE", "redis")`)
}

func TestUninstallActions_EnvCommentsAndBlankLines(t *testing.T) {
	root := setupProject(t)
	writeEnv(t, root, "APP_ENV=local")
	require.NoError(t, install(t))

	writeEnv(t, root, "# cache store\n\nCACHE_STORE=redis\n")

	require.NoError(t, uninstall(t))

	env := readFile(t, filepath.Join(root, ".env"))
	assert.Contains(t, env, "# cache store")
	assert.Contains(t, env, "CACHE_STORE=memory")
}

func TestUninstallActions_MissingDefault(t *testing.T) {
	root := setupProject(t)
	writeEnv(t, root, "CACHE_STORE=redis")

	writeFile(t, filepath.Join(root, "config", "cache.go"), `package config

import (
	"github.com/goravel/framework/facades"
)

func init() {
	config := facades.Config()
	config.Add("cache", map[string]any{
		"stores": map[string]any{},
	})
}
`)

	require.NoError(t, uninstall(t))

	assert.NotContains(t, readFile(t, filepath.Join(root, "config", "cache.go")), `"default"`)
}
