package redis

import (
	"fmt"
	"testing"

	mocksconfig "github.com/goravel/framework/mocks/config"
	"github.com/goravel/framework/support/env"
	"github.com/spf13/cast"
	"github.com/stretchr/testify/suite"
)

type ConnectionTestSuite struct {
	suite.Suite
	mockConfig *mocksconfig.Config

	docker    *Docker
	redisPort int
}

func TestManagerTestSuite(t *testing.T) {
	if env.IsWindows() {
		t.Skip("Skipping tests of using docker")
	}

	suite.Run(t, &ConnectionTestSuite{})
}

func (s *ConnectionTestSuite) SetupSuite() {
	s.mockConfig = mocksconfig.NewConfig(s.T())
	docker := initDocker(s.mockConfig)

	s.docker = docker
	s.redisPort = docker.Config().Port
}

func (s *ConnectionTestSuite) TearDownSuite() {
	s.NoError(s.docker.Shutdown())
}

func (s *ConnectionTestSuite) SetupTest() {
	clients.Clear()
}

func (s *ConnectionTestSuite) TestCreateClient() {
	s.Run("happy path", func() {
		s.mockConfig.EXPECT().GetString(fmt.Sprintf("database.redis.%s.host", testConnection)).Return(testHost).Once()
		s.mockConfig.EXPECT().GetString(fmt.Sprintf("database.redis.%s.port", testConnection), "6379").Return(cast.ToString(s.redisPort)).Once()
		s.mockConfig.EXPECT().GetString(fmt.Sprintf("database.redis.%s.username", testConnection)).Return("").Once()
		s.mockConfig.EXPECT().GetString(fmt.Sprintf("database.redis.%s.password", testConnection)).Return(testPassword).Once()
		s.mockConfig.EXPECT().GetInt(fmt.Sprintf("database.redis.%s.database", testConnection), 0).Return(0).Once()
		s.mockConfig.EXPECT().GetBool(fmt.Sprintf("database.redis.%s.cluster", testConnection), false).Return(false).Once()
		s.mockConfig.EXPECT().Get(fmt.Sprintf("database.redis.%s.tls", testConnection)).Return(nil).Once()

		client, err := createClient(s.mockConfig, testConnection)
		s.Nil(err)
		s.NotNil(client)
		s.Nil(err)
	})

	s.Run("empty host", func() {
		s.mockConfig.EXPECT().GetString(fmt.Sprintf("database.redis.%s.host", testConnection)).Return("").Once()

		client, err := createClient(s.mockConfig, testConnection)
		s.Equal(err, fmt.Errorf("redis host is not configured for connection [%s] at path 'database.redis.%s.host'", testConnection, testConnection))
		s.Nil(client)
	})

	s.Run("invalid host", func() {
		connection := "invalid"

		s.mockConfig.EXPECT().GetString(fmt.Sprintf("database.redis.%s.host", connection)).Return("invalid-host").Once()
		s.mockConfig.EXPECT().GetString(fmt.Sprintf("database.redis.%s.port", connection), "6379").Return("9999").Once()
		s.mockConfig.EXPECT().GetString(fmt.Sprintf("database.redis.%s.username", connection)).Return("").Once()
		s.mockConfig.EXPECT().GetString(fmt.Sprintf("database.redis.%s.password", connection)).Return("").Once()
		s.mockConfig.EXPECT().GetInt(fmt.Sprintf("database.redis.%s.database", connection), 0).Return(0).Once()
		s.mockConfig.EXPECT().GetBool(fmt.Sprintf("database.redis.%s.cluster", connection), false).Return(false).Once()
		s.mockConfig.EXPECT().Get(fmt.Sprintf("database.redis.%s.tls", connection)).Return(nil).Once()

		client, err := createClient(s.mockConfig, connection)
		s.NoError(err)
		s.Nil(client)
	})
}

func (s *ConnectionTestSuite) TestGetClient() {
	// Setup for successful connection
	s.mockConfig.EXPECT().GetString(fmt.Sprintf("database.redis.%s.host", testConnection)).Return(testHost).Once()
	s.mockConfig.EXPECT().GetString(fmt.Sprintf("database.redis.%s.port", testConnection), "6379").Return(cast.ToString(s.redisPort)).Once()
	s.mockConfig.EXPECT().GetString(fmt.Sprintf("database.redis.%s.username", testConnection)).Return("").Once()
	s.mockConfig.EXPECT().GetString(fmt.Sprintf("database.redis.%s.password", testConnection)).Return(testPassword).Once()
	s.mockConfig.EXPECT().GetInt(fmt.Sprintf("database.redis.%s.database", testConnection), 0).Return(0).Once()
	s.mockConfig.EXPECT().GetBool(fmt.Sprintf("database.redis.%s.cluster", testConnection), false).Return(false).Once()
	s.mockConfig.EXPECT().Get(fmt.Sprintf("database.redis.%s.tls", testConnection)).Return(nil).Once()

	// First call should create the client
	client, err := GetClient(s.mockConfig, testConnection)
	s.Nil(err)
	s.NotNil(client)

	// Second call should return the same instance (no more config calls expected)
	client2, err := GetClient(s.mockConfig, testConnection)
	s.Nil(err)
	s.NotNil(client2)
	s.Equal(client, client2)

	// Test with failed connection
	connection := "invalid"

	s.mockConfig.EXPECT().GetString(fmt.Sprintf("database.redis.%s.host", connection)).Return("invalid-host").Once()
	s.mockConfig.EXPECT().GetString(fmt.Sprintf("database.redis.%s.port", connection), "6379").Return("9999").Once()
	s.mockConfig.EXPECT().GetString(fmt.Sprintf("database.redis.%s.username", connection)).Return("").Once()
	s.mockConfig.EXPECT().GetString(fmt.Sprintf("database.redis.%s.password", connection)).Return("").Once()
	s.mockConfig.EXPECT().GetInt(fmt.Sprintf("database.redis.%s.database", connection), 0).Return(0).Once()
	s.mockConfig.EXPECT().GetBool(fmt.Sprintf("database.redis.%s.cluster", connection), false).Return(false).Once()
	s.mockConfig.EXPECT().Get(fmt.Sprintf("database.redis.%s.tls", connection)).Return(nil).Once()

	failedClient, err := GetClient(s.mockConfig, connection)
	s.NoError(err)
	s.Nil(failedClient)
}
