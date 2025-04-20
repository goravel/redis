package redis

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	"github.com/goravel/framework/contracts/config"

	"github.com/redis/go-redis/v9"
)

// Global map to store Redis client connections.
// Keyed by the connection name defined in the config.
var (
	clients = make(map[string]*redis.Client)
	mu      sync.RWMutex
)

// createClient initializes a new Redis client based on configuration.
// It performs a PING check to ensure connectivity.
func createClient(config config.Config, connection string) (*redis.Client, error) {

	redisConfigPath := fmt.Sprintf("database.redis.%s", connection)

	host := config.GetString(fmt.Sprintf("%s.host", redisConfigPath))
	if host == "" {
		return nil, fmt.Errorf("redis host is not configured for connection [%s] at path '%s.host'", connection, redisConfigPath)
	}
	port := config.GetString(fmt.Sprintf("%s.port", redisConfigPath), "6379") // Default port
	username := config.GetString(fmt.Sprintf("%s.username", redisConfigPath)) // Optional
	password := config.GetString(fmt.Sprintf("%s.password", redisConfigPath)) // Optional
	db := config.GetInt(fmt.Sprintf("%s.database", redisConfigPath), 0)       // Default DB 0

	options := &redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Username: username,
		Password: password,
		DB:       db,
	}

	tlsConfigRaw := config.Get(fmt.Sprintf("%s.tls", redisConfigPath))
	if tlsConfig, ok := tlsConfigRaw.(*tls.Config); ok && tlsConfig != nil {
		options.TLSConfig = tlsConfig
	}

	client := redis.NewClient(options)

	// Verify the connection using PING with a timeout
	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if status := client.Ping(pingCtx); status.Err() != nil {
		// Close the client if ping fails to release resources
		_ = client.Close() // Ignore close error as the connection likely failed anyway
		return nil, fmt.Errorf("failed to connect to redis connection [%s] (addr: %s): %w", connection, options.Addr, status.Err())
	}

	return client, nil
}

// GetClient returns a Redis client for the specified connection name.
// It uses a cached instance if one already exists for the name, otherwise,
// it creates, caches, and returns a new one. It is thread-safe.
// Returns an error if the client cannot be created or configured correctly.
func GetClient(config config.Config, connection string) (*redis.Client, error) {
	// 1. Fast path: Check if client exists with read lock (allows concurrent reads)
	mu.RLock()
	client, exists := clients[connection]
	mu.RUnlock()

	if exists {
		return client, nil
	}

	// 2. Slow path: Acquire write lock to create the client
	mu.Lock()
	defer mu.Unlock()

	// 3. Double check: Another goroutine might have created the client
	//    while we were waiting for the write lock.
	client, exists = clients[connection]
	if exists {
		return client, nil
	}

	// 4. Create the new client
	newClient, err := createClient(config, connection)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize redis client for connection [%s]: %w", connection, err)
	}

	clients[connection] = newClient

	return newClient, nil
}
