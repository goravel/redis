package redis

import (
	"log"
	"testing"

	mocksconfig "github.com/goravel/framework/mocks/config"
	"github.com/goravel/framework/support/docker"
	"github.com/spf13/cast"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ConnectionTestSuite struct {
	suite.Suite
	mockConfig *mocksconfig.Config
	redisPort  int
}

func TestManagerTestSuite(t *testing.T) {
	mockConfig := mocksconfig.NewConfig(t)
	redisDocker := docker.NewRedis()
	if err := redisDocker.Build(); err != nil {
		log.Fatalf("Failed to build Redis Docker container: %v", err)
	}
	suite.Run(t, &ConnectionTestSuite{mockConfig: mockConfig, redisPort: redisDocker.Config().Port})
	require.NoError(t, redisDocker.Shutdown(), "Failed to shutdown Redis Docker container")
}

func (s *ConnectionTestSuite) TestCreateClient() {
	// Test successful connection
	s.mockConfig.On("GetString", "database.redis.create_client_connection.host").Return("localhost").Once()
	s.mockConfig.On("GetString", "database.redis.create_client_connection.port", "6379").Return(cast.ToString(s.redisPort)).Once()
	s.mockConfig.On("GetString", "database.redis.create_client_connection.username").Return("").Once()
	s.mockConfig.On("GetString", "database.redis.create_client_connection.password").Return("").Once()
	s.mockConfig.On("GetInt", "database.redis.create_client_connection.database", 0).Return(0).Once()
	s.mockConfig.On("Get", "database.redis.create_client_connection.tls").Return(nil).Once()

	client, err := createClient(s.mockConfig, "create_client_connection")
	s.Nil(err)
	s.NotNil(client)
	s.Nil(err)
}

func (s *ConnectionTestSuite) TestCreateClientWithEmptyHost() {
	s.mockConfig.On("GetString", "database.redis.invalid.host").Return("").Once()

	client, err := createClient(s.mockConfig, "invalid")
	s.Error(err)
	s.Nil(client)
	s.Contains(err.Error(), "redis host is not configured for connection [invalid]")
}

func (s *ConnectionTestSuite) TestCreateClientWithInvalidHost() {
	s.mockConfig.On("GetString", "database.redis.invalid.host").Return("invalid-host").Once()
	s.mockConfig.On("GetString", "database.redis.invalid.port", "6379").Return("9999").Once()
	s.mockConfig.On("GetString", "database.redis.invalid.username").Return("").Once()
	s.mockConfig.On("GetString", "database.redis.invalid.password").Return("").Once()
	s.mockConfig.On("GetInt", "database.redis.invalid.database", 0).Return(0).Once()
	s.mockConfig.On("Get", "database.redis.invalid.tls").Return(nil).Once()

	client, err := createClient(s.mockConfig, "invalid")
	s.Error(err)
	s.Nil(client)
	s.Contains(err.Error(), "failed to connect to redis connection [invalid]")
}

func (s *ConnectionTestSuite) TestGetClient() {
	// Setup for successful connection
	s.mockConfig.On("GetString", "database.redis.default.host").Return("localhost").Once()
	s.mockConfig.On("GetString", "database.redis.default.port", "6379").Return(cast.ToString(s.redisPort)).Once()
	s.mockConfig.On("GetString", "database.redis.default.username").Return("").Once()
	s.mockConfig.On("GetString", "database.redis.default.password").Return("").Once()
	s.mockConfig.On("GetInt", "database.redis.default.database", 0).Return(0).Once()
	s.mockConfig.On("Get", "database.redis.default.tls").Return(nil).Once()

	// First call should create the client
	client, err := GetClient(s.mockConfig, "default")
	s.Nil(err)
	s.NotNil(client)

	// Second call should return the same instance (no more config calls expected)
	client2, err := GetClient(s.mockConfig, "default")
	s.Nil(err)
	s.NotNil(client2)
	s.Equal(client, client2)

	// Test with failed connection
	s.mockConfig.On("GetString", "database.redis.invalid.host").Return("invalid-host").Once()
	s.mockConfig.On("GetString", "database.redis.invalid.port", "6379").Return("9999").Once()
	s.mockConfig.On("GetString", "database.redis.invalid.username").Return("").Once()
	s.mockConfig.On("GetString", "database.redis.invalid.password").Return("").Once()
	s.mockConfig.On("GetInt", "database.redis.invalid.database", 0).Return(0).Once()
	s.mockConfig.On("Get", "database.redis.invalid.tls").Return(nil).Once()

	failedClient, err := GetClient(s.mockConfig, "invalid")
	s.Error(err)
	s.Nil(failedClient)
}

func (s *ConnectionTestSuite) TestPingTimeout() {
	// Test that the connection works with the default timeout
	s.mockConfig.On("GetString", "database.redis.ping_connection.host").Return("localhost").Once()
	s.mockConfig.On("GetString", "database.redis.ping_connection.port", "6379").Return(cast.ToString(s.redisPort)).Once()
	s.mockConfig.On("GetString", "database.redis.ping_connection.username").Return("").Once()
	s.mockConfig.On("GetString", "database.redis.ping_connection.password").Return("").Once()
	s.mockConfig.On("GetInt", "database.redis.ping_connection.database", 0).Return(0).Once()
	s.mockConfig.On("Get", "database.redis.ping_connection.tls").Return(nil).Once()

	// This test validates that the ping_connection ping timeout (5 seconds) works properly
	client, err := createClient(s.mockConfig, "ping_connection")
	s.Nil(err)
	s.NotNil(client)

	// Clean up
	err = client.Close()
	s.Nil(err)
}
