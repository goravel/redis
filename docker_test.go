package redis

import (
	"context"
	"fmt"
	"testing"

	contractsdocker "github.com/goravel/framework/contracts/testing/docker"
	mocksconfig "github.com/goravel/framework/mocks/config"
	testingdocker "github.com/goravel/framework/testing/docker"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

const (
	testStore      = "redis"
	testConnection = "default"
	testUsername   = "user"
	testPassword   = "pass"
	testPort       = 6379
	testDatabase   = 0
	testHost       = "localhost"
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
	tests := []struct {
		name                string
		host                string
		password            string
		port                int
		expectedImageDriver *testingdocker.ImageDriver
		expectedError       bool
	}{
		{
			name:     "success",
			host:     "localhost",
			password: testPassword,
			expectedImageDriver: testingdocker.NewImageDriver(contractsdocker.Image{
				Repository:   "redis",
				Tag:          "latest",
				ExposedPorts: []string{"6379"},
				Args:         []string{fmt.Sprintf("--requirepass %s", testPassword)},
			}),
			expectedError: false,
		},
		{
			name:     "missing host",
			host:     "",
			password: testPassword,
			expectedImageDriver: testingdocker.NewImageDriver(contractsdocker.Image{
				Repository:   "redis",
				Tag:          "latest",
				ExposedPorts: []string{"6379"},
				Args:         []string{fmt.Sprintf("--requirepass %s", testPassword)},
			}),
			expectedError: true,
		},
		{
			name:     "missing password",
			host:     "localhost",
			password: "",
			expectedImageDriver: testingdocker.NewImageDriver(contractsdocker.Image{
				Repository:   "redis",
				Tag:          "latest",
				ExposedPorts: []string{"6379"},
			}),
			expectedError: false,
		},
	}

	for _, test := range tests {
		s.Run(test.name, func() {
			s.SetupTest()
			s.mockConfig.On("GetString", fmt.Sprintf("cache.stores.%s.connection", testStore)).Return(testConnection).Once()
			s.mockConfig.On("GetString", fmt.Sprintf("database.redis.%s.host", testConnection)).Return(test.host).Once()
			s.mockConfig.On("GetInt", fmt.Sprintf("database.redis.%s.port", testConnection), 6379).Return(testPort).Once()
			s.mockConfig.On("GetString", fmt.Sprintf("database.redis.%s.username", testConnection)).Return(testUsername).Once()
			s.mockConfig.On("GetString", fmt.Sprintf("database.redis.%s.password", testConnection)).Return(test.password).Once()
			s.mockConfig.On("GetInt", fmt.Sprintf("database.redis.%s.database", testConnection), 0).Return(testDatabase).Once()

			docker, err := NewDocker(s.mockConfig, testStore)

			if test.expectedError {
				assert.Error(s.T(), err)
				assert.Nil(s.T(), docker)
			} else {
				s.NoError(err)
				s.NotNil(docker)
				s.Equal(test.host, docker.config.Host)
				s.Equal(testPort, docker.config.Port)
				s.Equal(testUsername, docker.config.Username)
				s.Equal(test.password, docker.config.Password)
				s.Equal(fmt.Sprintf("%d", testDatabase), docker.config.Database)
				s.Equal(test.expectedImageDriver, docker.imageDriver)
			}
		})
	}
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
		imageDriver: testingdocker.NewImageDriver(contractsdocker.Image{
			Repository:   "redis",
			Tag:          "latest",
			ExposedPorts: []string{"6379"},
			Args:         []string{"--requirepass 123123"},
		}),
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

func initDocker(mockConfig *mocksconfig.Config) *Docker {
	mockConfig.On("GetString", fmt.Sprintf("cache.stores.%s.connection", testStore)).Return(testConnection).Once()
	mockConfig.On("GetString", fmt.Sprintf("database.redis.%s.host", testConnection)).Return(testHost).Once()
	mockConfig.On("GetString", fmt.Sprintf("database.redis.%s.username", testConnection)).Return("").Once()
	mockConfig.On("GetString", fmt.Sprintf("database.redis.%s.password", testConnection)).Return(testPassword).Once()
	mockConfig.On("GetInt", fmt.Sprintf("database.redis.%s.database", testConnection), 0).Return(testDatabase).Once()

	docker, err := NewDocker(mockConfig, testStore)
	if err != nil {
		panic(err)
	}

	if err := docker.Build(); err != nil {
		panic(err)
	}

	if err := docker.Ready(); err != nil {
		panic(err)
	}

	return docker
}
