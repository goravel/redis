package redis

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/goravel/framework/contracts/config"
	"github.com/goravel/framework/contracts/session"
)

// Ensure Redis driver satisfies the session.Driver interface at compile time.
var _ session.Driver = (*Session)(nil)

type Session struct {
	ctx             context.Context
	config          config.Config
	prefix          string
	instance        *redis.Client
	lifetimeSeconds int
}

// NewRedis creates a new Redis session driver using Goravel's configuration.
func NewSession(ctx context.Context, config config.Config, connection string) (*Session, error) {

	if connection == "" {
		connection = "default"
	}

	redisConfigPath := fmt.Sprintf("database.redis.%s", connection)
	host := config.GetString(fmt.Sprintf("%s.host", redisConfigPath))
	port := config.GetString(fmt.Sprintf("%s.port", redisConfigPath), "6379")
	password := config.GetString(fmt.Sprintf("%s.password", redisConfigPath))
	db := config.GetInt(fmt.Sprintf("%s.database", redisConfigPath), 0)

	if host == "" {
		return nil, fmt.Errorf("redis host is not configured for connection [%s]", connection)
	}

	option := &redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Username: config.GetString(fmt.Sprintf("database.redis.%s.username", connection)),
		Password: password,
		DB:       db,
	}

	tlsConfig, ok := config.Get(fmt.Sprintf("%s.tls", redisConfigPath)).(*tls.Config)
	if ok && tlsConfig != nil {
		option.TLSConfig = tlsConfig
	}

	client := redis.NewClient(option)

	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := client.Ping(pingCtx).Result(); err != nil {

		_ = client.Close()
		return nil, fmt.Errorf("failed to connect to redis connection [%s] (%s): %w", connection, option.Addr, err)
	}

	lifetimeMinutes := config.GetInt("session.lifetime", 120)

	sessionPrefixBase := config.GetString("session.cookie", "goravel_session")
	sessionPrefix := fmt.Sprintf("%s:", sessionPrefixBase)

	return &Session{
		ctx:             ctx,
		config:          config,
		prefix:          sessionPrefix,
		instance:        client,
		lifetimeSeconds: lifetimeMinutes * 60,
	}, nil
}

// Close closes the Redis client connection.
func (r *Session) Close() error {
	if r.instance != nil {
		return r.instance.Close()
	}
	return nil
}

// Destroy removes a session from Redis.
func (r *Session) Destroy(id string) error {
	key := r.getKey(id)
	err := r.instance.Del(r.ctx, key).Err()
	if err != nil && !errors.Is(err, redis.Nil) { // Ignore error if key already gone
		return fmt.Errorf("failed to delete session '%s' from redis: %w", key, err)
	}
	return nil
}

// Gc performs garbage collection. (No-op for Redis TTL-based expiration)
func (r *Session) Gc(maxLifetime int) error {
	return nil
}

func (r *Session) Open(path string, name string) error {
	return nil
}

// Read retrieves session data from Redis.
func (r *Session) Read(id string) (string, error) {
	key := r.getKey(id)
	data, err := r.instance.Get(r.ctx, key).Result()

	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read session '%s' from redis: %w", key, err)
	}

	return data, nil
}

// Write saves session data to Redis with the configured lifetime.
func (r *Session) Write(id string, data string) error {
	key := r.getKey(id)
	expiration := time.Duration(r.lifetimeSeconds) * time.Second

	err := r.instance.Set(r.ctx, key, data, expiration).Err()
	if err != nil {
		return fmt.Errorf("failed to write session '%s' to redis: %w", key, err)
	}
	return nil
}

func (r *Session) getKey(id string) string {
	return r.prefix + id
}
