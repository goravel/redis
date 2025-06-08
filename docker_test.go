package redis

import (
	"context"
	"fmt"
	"testing"

	contractsdocker "github.com/goravel/framework/contracts/testing/docker"
	mocksconfig "github.com/goravel/framework/mocks/config"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type DockerTestSuite struct {
	suite.Suite
	mockConfig *mocksconfig.Config
}

func TestDockerTestSuite(t *testing.T) {
	suite.Run(t, &DockerTestSuite{})
}

func (s *DockerTestSuite) SetupTest() {
	s.mockConfig = &mocksconfig.Config{}
}

func (s *DockerTestSuite) TestNewDocker() {
	store := "redis"
	connection := "default"
	username := "user"
	password := "pass"
	port := 6379
	database := 0

	tests := []struct {
		name          string
		host          string
		port          int
		expectedError bool
	}{
		{
			name:          "success",
			host:          "localhost",
			expectedError: false,
		},
		{
			name:          "missing host",
			host:          "",
			expectedError: true,
		},
	}

	for _, test := range tests {
		s.Run(test.name, func() {
			s.mockConfig.On("GetString", fmt.Sprintf("cache.stores.%s.connection", store)).Return(connection).Once()
			s.mockConfig.On("GetString", fmt.Sprintf("database.redis.%s.host", connection)).Return(test.host).Once()
			s.mockConfig.On("GetInt", fmt.Sprintf("database.redis.%s.port", connection), 6379).Return(port).Once()
			s.mockConfig.On("GetString", fmt.Sprintf("database.redis.%s.username", connection)).Return(username).Once()
			s.mockConfig.On("GetString", fmt.Sprintf("database.redis.%s.password", connection)).Return(password).Once()
			s.mockConfig.On("GetInt", fmt.Sprintf("database.redis.%s.database", connection), 0).Return(database).Once()

			docker, err := NewDocker(s.mockConfig, store)

			if test.expectedError {
				assert.Error(s.T(), err)
				assert.Nil(s.T(), docker)
			} else {
				s.NoError(err)
				s.NotNil(docker)
				s.Equal(test.host, docker.config.Host)
				s.Equal(port, docker.config.Port)
				s.Equal(username, docker.config.Username)
				s.Equal(password, docker.config.Password)
				s.Equal(fmt.Sprintf("%d", database), docker.config.Database)
			}
		})
	}
}

func (s *DockerTestSuite) TestImage() {
	docker := &Docker{}

	newImage := contractsdocker.Image{
		Repository:   "custom-redis",
		Tag:          "7.0",
		ExposedPorts: []string{"6380"},
		Args:         []string{"--custom-arg"},
	}

	docker.Image(newImage)

	s.Equal(newImage.Repository, docker.image.Repository)
	s.Equal(newImage.Tag, docker.image.Tag)
	s.Equal(newImage.ExposedPorts, docker.image.ExposedPorts)
	s.Equal(newImage.Args, docker.image.Args)
}

func (s *DockerTestSuite) TestConfig() {
	docker := &Docker{}

	config := docker.Config()
	s.Equal(docker.config, config)
}

func (s *DockerTestSuite) TestReuse() {
	docker := &Docker{}

	containerID := "test-container"
	port := 6380

	err := docker.Reuse(containerID, port)
	s.NoError(err)
	s.Equal(containerID, docker.config.ContainerID)
	s.Equal(port, docker.config.Port)
}

func (s *DockerTestSuite) TestBuildReadyFreshShutdown() {
	docker := &Docker{
		config: contractsdocker.CacheConfig{
			Host:     "localhost",
			Port:     6379,
			Password: "123123",
			Database: "0",
		},
		image: &contractsdocker.Image{
			Repository:   "redis",
			Tag:          "latest",
			ExposedPorts: []string{"6379"},
			Args:         []string{"--requirepass 123123"},
		},
	}

	err := docker.Build()
	s.NoError(err)
	s.NotEmpty(docker.config.ContainerID)
	s.NotZero(docker.config.Port)

	err = docker.Ready()
	s.NoError(err)

	client, err := docker.connect()
	s.NoError(err)
	s.NotNil(client)

	err = client.Set(context.Background(), "test", "test", 0).Err()
	s.NoError(err)

	value, err := client.Get(context.Background(), "test").Result()
	s.NoError(err)
	s.Equal("test", value)

	err = docker.Fresh()
	s.NoError(err)

	value, err = client.Get(context.Background(), "test").Result()
	s.Equal(redis.Nil, err)
	s.Empty(value)

	err = docker.Shutdown()
	s.NoError(err)
}
