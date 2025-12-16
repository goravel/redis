package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/goravel/framework/contracts/config"
	"github.com/goravel/framework/contracts/session"
	"github.com/redis/go-redis/v9"
)

// Ensure Redis driver satisfies the session.Driver interface at compile time.
var _ session.Driver = (*Session)(nil)

type Session struct {
	ctx             context.Context
	config          config.Config
	prefix          string
	instance        redis.UniversalClient
	lifetimeSeconds int
}

// NewSession creates a new Redis session driver using Goravel's configuration.
func NewSession(ctx context.Context, config config.Config, driver string) (*Session, error) {

	connection := config.GetString(fmt.Sprintf("session.drivers.%s.connection", driver), "default")

	client, err := GetClient(config, connection)
	if err != nil {
		return nil, fmt.Errorf("failed to init redis client: %w", err)
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
	// This can affect other parts of the application using the same Redis client.
	// If you want to close the client, we need to be sure that no other part of the application is using it.
	// if r.instance != nil {
	// 	return r.instance.Close()
	// }
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
