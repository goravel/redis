package redis

import (
	"context"

	configmocks "github.com/goravel/framework/mocks/config"
	queuemocks "github.com/goravel/framework/mocks/queue"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
)

func getCacheDocker() (*dockertest.Pool, *dockertest.Resource, *Cache, error) {
	mockConfig := &configmocks.Config{}
	pool, resource, err := initRedisDocker()
	if err != nil {
		return nil, nil, nil, err
	}

	var store *Cache
	if err := pool.Retry(func() error {
		var err error
		mockConfig.On("GetString", "cache.stores.redis.connection", "default").Return("default").Once()
		mockConfig.On("GetString", "database.redis.default.host").Return("localhost").Once()
		mockConfig.On("GetString", "database.redis.default.port").Return(resource.GetPort("6379/tcp")).Once()
		mockConfig.On("GetString", "database.redis.default.password").Return(resource.GetPort("")).Once()
		mockConfig.On("GetInt", "database.redis.default.database").Return(0).Once()
		mockConfig.On("GetString", "cache.prefix").Return("goravel_cache").Once()
		store, err = NewCache(context.Background(), mockConfig, "redis")

		return err
	}); err != nil {
		return nil, nil, nil, err
	}

	return pool, resource, store, nil
}

func getQueueDocker() (*dockertest.Pool, *dockertest.Resource, *Queue, error) {
	mockConfig := &configmocks.Config{}
	mockQueue := &queuemocks.Queue{}
	pool, resource, err := initRedisDocker()
	if err != nil {
		return nil, nil, nil, err
	}

	var queue *Queue
	if err := pool.Retry(func() error {
		var err error
		mockConfig.On("GetString", "queue.connections.redis.connection", "default").Return("default").Once()
		mockConfig.On("GetString", "database.redis.default.host").Return("localhost").Once()
		mockConfig.On("GetString", "database.redis.default.port").Return(resource.GetPort("6379/tcp")).Once()
		mockConfig.On("GetString", "database.redis.default.password").Return(resource.GetPort("")).Once()
		mockConfig.On("GetInt", "database.redis.default.database").Return(0).Once()
		queue, err = NewQueue(context.Background(), mockConfig, mockQueue, "redis")

		return err
	}); err != nil {
		return nil, nil, nil, err
	}

	return pool, resource, queue, nil
}

func pool() (*dockertest.Pool, error) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		return nil, errors.WithMessage(err, "Could not construct pool")
	}

	if err := pool.Client.Ping(); err != nil {
		return nil, errors.WithMessage(err, "Could not connect to Docker")
	}

	return pool, nil
}

func resource(pool *dockertest.Pool, opts *dockertest.RunOptions) (*dockertest.Resource, error) {
	return pool.RunWithOptions(opts, func(config *docker.HostConfig) {
		// set AutoRemove to true so that stopped container goes away by itself
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{
			Name: "no",
		}
	})
}

func initRedisDocker() (*dockertest.Pool, *dockertest.Resource, error) {
	pool, err := pool()
	if err != nil {
		return nil, nil, err
	}
	resource, err := resource(pool, &dockertest.RunOptions{
		Repository: "redis",
		Tag:        "latest",
		Env:        []string{},
	})
	if err != nil {
		return nil, nil, err
	}
	_ = resource.Expire(600)

	if err := pool.Retry(func() error {
		client := redis.NewClient(&redis.Options{
			Addr:     "localhost:" + resource.GetPort("6379/tcp"),
			Password: "",
			DB:       0,
		})

		if _, err := client.Ping(context.Background()).Result(); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return nil, nil, err
	}

	return pool, resource, nil
}
