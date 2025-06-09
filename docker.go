package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	contractsconfig "github.com/goravel/framework/contracts/config"
	contractsdocker "github.com/goravel/framework/contracts/testing/docker"
	supportdocker "github.com/goravel/framework/support/docker"
	testingdocker "github.com/goravel/framework/testing/docker"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cast"
)

type Docker struct {
	config      contractsdocker.CacheConfig
	imageDriver contractsdocker.ImageDriver
}

func NewDocker(config contractsconfig.Config, store string) (*Docker, error) {
	connection := config.GetString(fmt.Sprintf("cache.stores.%s.connection", store))
	configPrefix := fmt.Sprintf("database.redis.%s", connection)
	host := config.GetString(fmt.Sprintf("%s.host", configPrefix))
	if host == "" {
		return nil, fmt.Errorf("redis host is not configured for connection [%s] at path '%s.host'", connection, configPrefix)
	}

	username := config.GetString(fmt.Sprintf("%s.username", configPrefix))
	password := config.GetString(fmt.Sprintf("%s.password", configPrefix))
	database := config.GetInt(fmt.Sprintf("%s.database", configPrefix), 0)

	return &Docker{
		config: contractsdocker.CacheConfig{
			Database: strconv.Itoa(database),
			Host:     host,
			Password: password,
			Port:     6379,
			Username: username,
		},
		imageDriver: testingdocker.NewImageDriver(contractsdocker.Image{
			Repository:   "redis",
			Tag:          "latest",
			ExposedPorts: []string{"6379"},
			Args:         []string{fmt.Sprintf("--requirepass %s", password)},
		}),
	}, nil
}

func (r *Docker) Build() error {
	if err := r.imageDriver.Build(); err != nil {
		return err
	}

	config := r.imageDriver.Config()
	r.config.ContainerID = config.ContainerID
	r.config.Port = cast.ToInt(supportdocker.ExposedPort(config.ExposedPorts, strconv.Itoa(r.config.Port)))

	return nil
}

func (r *Docker) Config() contractsdocker.CacheConfig {
	return r.config
}

func (r *Docker) Fresh() error {
	client, err := r.connect()
	if err != nil {
		return fmt.Errorf("connect Redis docker error: %v", err)
	}

	if err := client.FlushAll(context.Background()).Err(); err != nil {
		return fmt.Errorf("fresh Redis docker error: %v", err)
	}

	return client.Close()
}

func (r *Docker) Image(image contractsdocker.Image) {
	r.imageDriver = testingdocker.NewImageDriver(image)
}

func (r *Docker) Ready() error {
	client, err := r.connect()
	if err != nil {
		return fmt.Errorf("connect Redis docker error: %v", err)
	}

	return client.Close()
}

func (r *Docker) Reuse(containerID string, port int) error {
	r.config.ContainerID = containerID
	r.config.Port = port

	return nil
}

func (r *Docker) Shutdown() error {
	return r.imageDriver.Shutdown()
}

func (r *Docker) connect() (*redis.Client, error) {
	var (
		client *redis.Client
		err    error
	)
	for i := 0; i < 60; i++ {
		client = redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%d", r.config.Host, r.config.Port),
			Password: r.config.Password,
			DB:       cast.ToInt(r.config.Database),
		})

		if _, err = client.Ping(context.Background()).Result(); err == nil {
			break
		}

		time.Sleep(2 * time.Second)
	}

	return client, err
}
