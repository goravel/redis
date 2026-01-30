package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	contractsconfig "github.com/goravel/framework/contracts/config"
	contractsprocess "github.com/goravel/framework/contracts/process"
	contractsdocker "github.com/goravel/framework/contracts/testing/docker"
	supportdocker "github.com/goravel/framework/support/docker"
	testingdocker "github.com/goravel/framework/testing/docker"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cast"
)

type Docker struct {
	config      contractsconfig.Config
	cacheConfig contractsdocker.CacheConfig
	imageDriver contractsdocker.ImageDriver
	process     contractsprocess.Process
	connection  string
}

func NewDocker(config contractsconfig.Config, process contractsprocess.Process, store string) (*Docker, error) {
	connection := config.GetString(fmt.Sprintf("cache.stores.%s.connection", store))
	configPrefix := fmt.Sprintf("database.redis.%s", connection)
	host := config.GetString(fmt.Sprintf("%s.host", configPrefix))
	if host == "" {
		return nil, fmt.Errorf("redis host is not configured for connection [%s] at path '%s.host'", connection, configPrefix)
	}

	username := config.GetString(fmt.Sprintf("%s.username", configPrefix))
	password := config.GetString(fmt.Sprintf("%s.password", configPrefix))
	database := config.GetInt(fmt.Sprintf("%s.database", configPrefix), 0)

	var args []string
	if password != "" {
		args = append(args, fmt.Sprintf("--requirepass %s", password))
	}

	return &Docker{
		connection: connection,
		config:     config,
		cacheConfig: contractsdocker.CacheConfig{
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
			Args:         args,
		}, process),
		process: process,
	}, nil
}

func (r *Docker) Build() error {
	if err := r.imageDriver.Build(); err != nil {
		return err
	}

	config := r.imageDriver.Config()
	r.cacheConfig.ContainerID = config.ContainerID
	r.cacheConfig.Port = cast.ToInt(supportdocker.ExposedPort(config.ExposedPorts, strconv.Itoa(r.cacheConfig.Port)))

	return nil
}

func (r *Docker) Config() contractsdocker.CacheConfig {
	return r.cacheConfig
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
	r.imageDriver = testingdocker.NewImageDriver(image, r.process)
}

func (r *Docker) Ready() error {
	client, err := r.connect()
	if err != nil {
		return fmt.Errorf("connect Redis docker error: %v", err)
	}

	r.resetConfigPort()

	return client.Close()
}

func (r *Docker) Reuse(containerID string, port int) error {
	r.cacheConfig.ContainerID = containerID
	r.cacheConfig.Port = port

	return nil
}

func (r *Docker) Shutdown() error {
	return r.imageDriver.Shutdown()
}

func (r *Docker) connect() (redis.UniversalClient, error) {
	var (
		client redis.UniversalClient
		err    error
	)

	for i := 0; i < 60; i++ {
		client = redis.NewUniversalClient(&redis.UniversalOptions{
			Addrs:    []string{fmt.Sprintf("%s:%d", r.cacheConfig.Host, r.cacheConfig.Port)},
			Password: r.cacheConfig.Password,
			DB:       cast.ToInt(r.cacheConfig.Database),
		})

		if _, err = client.Ping(context.Background()).Result(); err == nil {
			break
		}

		time.Sleep(2 * time.Second)
	}

	return client, err
}

func (r *Docker) resetConfigPort() {
	r.config.Add(fmt.Sprintf("database.redis.%s.port", r.connection), r.cacheConfig.Port)
}
