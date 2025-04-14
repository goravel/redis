package redis

import (
	"context"
	"fmt"
	"testing"
	"time"

	redisclient "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	configmock "github.com/goravel/framework/mocks/config"
	testingdocker "github.com/goravel/framework/support/docker"
	"github.com/goravel/framework/support/env"
	"github.com/spf13/cast"
)

const (
	testSessionLifetime = 2
	testSessionCookie   = "goravel_session_test_redis"
	testRedisDriverName = "redis"
	testRedisConnection = "default"
	testPrefix          = testSessionCookie + ":"
	testSessionFiles    = ""
)

type SessionTestSuite struct {
	suite.Suite
	Session        *Session
	docker         *testingdocker.Redis
	mockConfig     *configmock.Config
	rawRedisClient *redisclient.Client
}

// TestSessionTestSuite runs the test suite
func TestSessionTestSuite(t *testing.T) {
	if env.IsWindows() {
		t.Skip("Skipping Redis session tests using Docker on Windows")
	}

	redisDocker := testingdocker.NewRedis()
	if err := redisDocker.Build(); err != nil {
		t.Fatalf("Failed to build Redis Docker container: %v", err)
	}
	t.Logf("Redis Docker container running on port %d", redisDocker.Config().Port)

	mockConfig := configmock.NewConfig(t)
	dockerPortStr := cast.ToString(redisDocker.Config().Port)

	mockConfig.On("GetString", fmt.Sprintf("database.redis.%s.host", testRedisConnection)).Return("localhost").Once()
	mockConfig.On("GetString", fmt.Sprintf("database.redis.%s.port", testRedisConnection), "6379").Return(dockerPortStr).Once()
	mockConfig.On("GetString", fmt.Sprintf("database.redis.%s.password", testRedisConnection)).Return("").Once()
	mockConfig.On("GetString", fmt.Sprintf("database.redis.%s.username", testRedisConnection)).Return("").Once()
	mockConfig.On("GetInt", fmt.Sprintf("database.redis.%s.database", testRedisConnection), 0).Return(0).Once()
	mockConfig.On("Get", fmt.Sprintf("database.redis.%s.tls", testRedisConnection)).Return(nil).Once()
	mockConfig.On("GetInt", "session.lifetime", 120).Return(testSessionLifetime).Once()
	mockConfig.On("GetString", "session.cookie", "goravel_session").Return(testSessionCookie).Once()

	SessionDriver, err := NewSession(context.Background(), mockConfig, testRedisConnection)
	require.NoError(t, err, "NewRedis should succeed")
	require.NotNil(t, SessionDriver, "NewRedis result should not be nil")

	rawClient := redisclient.NewClient(&redisclient.Options{
		Addr: fmt.Sprintf("localhost:%s", dockerPortStr),
		DB:   0,
	})
	_, pingErr := rawClient.Ping(context.Background()).Result()
	require.NoError(t, pingErr, "Failed to ping Redis with raw client")

	suite.Run(t, &SessionTestSuite{
		Session:        SessionDriver,
		docker:         redisDocker,
		mockConfig:     mockConfig,
		rawRedisClient: rawClient,
	})

	require.NoError(t, SessionDriver.Close(), "Failed to close session driver")
	require.NoError(t, rawClient.Close(), "Failed to close raw redis client")
	require.NoError(t, redisDocker.Shutdown(), "Failed to shutdown Redis Docker container")
}

// BeforeTest runs before each test in the suite.
func (s *SessionTestSuite) BeforeTest(suiteName, testName string) {
	err := s.rawRedisClient.FlushDB(context.Background()).Err()
	s.Require().NoError(err, "Failed to flush Redis DB before test %s/%s", suiteName, testName)
}

func (s *SessionTestSuite) TestWrite() {
	testID := "write_session_id"
	testData := "session_data_to_write"
	expectedKey := testPrefix + testID
	expectedTTL := time.Duration(s.Session.lifetimeSeconds) * time.Second

	err := s.Session.Write(testID, testData)
	s.Nil(err, "Write should not return an error")

	ctx := context.Background()
	actualData, err := s.rawRedisClient.Get(ctx, expectedKey).Result()
	s.Require().NoError(err, "Raw client failed to GET written key")
	s.Equal(testData, actualData, "Data written to Redis does not match")

	actualTTL, err := s.rawRedisClient.TTL(ctx, expectedKey).Result()
	s.Require().NoError(err, "Raw client failed to get TTL")
	s.Greater(actualTTL, time.Duration(0), "TTL should be positive")
	s.InDelta(expectedTTL, actualTTL, float64(3*time.Second), "TTL is not close to the configured lifetime")
}

func (s *SessionTestSuite) TestRead_Exists() {
	testID := "read_existing_session_id"
	testData := "existing_data"
	key := testPrefix + testID

	ttl := time.Duration(s.Session.lifetimeSeconds+10) * time.Second
	errSetup := s.rawRedisClient.Set(context.Background(), key, testData, ttl).Err()
	s.Require().NoError(errSetup, "Setup failed: Raw client could not SET data")

	data, err := s.Session.Read(testID)
	s.Nil(err, "Read should not return an error for existing key")
	s.Equal(testData, data, "Read returned incorrect data")
}

func (s *SessionTestSuite) TestRead_NotExist() {
	testID := "read_non_existent_session_id"

	exists, errExists := s.rawRedisClient.Exists(context.Background(), testPrefix+testID).Result()
	s.Require().NoError(errExists)
	s.Require().EqualValues(0, exists, "Test setup failed: Key should not exist")

	data, err := s.Session.Read(testID)
	s.Nil(err, "Read should not return an error for non-existent key")
	s.Equal("", data, "Read should return empty string for non-existent key")
}

func (s *SessionTestSuite) TestRead_Expired() {
	testID := "read_expired_session_id"
	testData := "expired_data"
	key := testPrefix + testID
	shortTTL := 1 * time.Second

	errSetup := s.rawRedisClient.Set(context.Background(), key, testData, shortTTL).Err()
	s.Require().NoError(errSetup, "Setup failed: Raw client could not SET data")

	time.Sleep(shortTTL + 500*time.Millisecond)

	data, err := s.Session.Read(testID)
	s.Nil(err, "Read should not return an error for expired key")
	s.Equal("", data, "Read should return empty string for expired key")
}

func (s *SessionTestSuite) TestDestroy_Exists() {
	testID := "destroy_existing_session_id"
	testData := "data_to_destroy"
	key := testPrefix + testID

	errSetup := s.Session.Write(testID, testData)
	s.Require().NoError(errSetup, "Setup failed: Could not write data to destroy")

	dataBefore, errReadBefore := s.Session.Read(testID)
	s.Require().Nil(errReadBefore)
	s.Require().Equal(testData, dataBefore)

	err := s.Session.Destroy(testID)
	s.Nil(err, "Destroy should not return an error")

	exists, errExists := s.rawRedisClient.Exists(context.Background(), key).Result()
	s.Require().NoError(errExists)
	s.EqualValues(0, exists, "Key should not exist in Redis after Destroy")

	dataAfter, errReadAfter := s.Session.Read(testID)
	s.Nil(errReadAfter)
	s.Equal("", dataAfter, "Read should return empty after Destroy")
}

func (s *SessionTestSuite) TestDestroy_NotExist() {
	testID := "destroy_non_existent_session_id"
	key := testPrefix + testID

	exists, errExists := s.rawRedisClient.Exists(context.Background(), key).Result()
	s.Require().NoError(errExists)
	s.Require().EqualValues(0, exists, "Test setup failed: Key should not exist before destroy")

	err := s.Session.Destroy(testID)
	s.Nil(err, "Destroy should not return an error for non-existent key")
}

func (s *SessionTestSuite) TestGc() {
	err := s.Session.Gc(300)
	s.Nil(err, "Gc should be a no-op and return nil error")
}

func (s *SessionTestSuite) TestOpen() {
	err := s.Session.Open("", "")
	s.Nil(err, "Open should be a no-op and return nil error")
}

// TestClose is implicitly tested by the suite runner teardown which calls Close.
// We can add an explicit simple call check here too.
func (s *SessionTestSuite) TestClose() {
	mockConfig := configmock.NewConfig(s.T())
	dockerPortStr := cast.ToString(s.docker.Config().Port)
	mockConfig.On("GetString", fmt.Sprintf("database.redis.%s.host", testRedisConnection)).Return("localhost").Once()
	mockConfig.On("GetString", fmt.Sprintf("database.redis.%s.port", testRedisConnection), "6379").Return(dockerPortStr).Once()
	mockConfig.On("GetString", fmt.Sprintf("database.redis.%s.password", testRedisConnection)).Return("").Once()
	mockConfig.On("GetString", fmt.Sprintf("database.redis.%s.username", testRedisConnection)).Return("").Once()
	mockConfig.On("GetInt", fmt.Sprintf("database.redis.%s.database", testRedisConnection), 0).Return(0).Once()
	mockConfig.On("Get", fmt.Sprintf("database.redis.%s.tls", testRedisConnection)).Return(nil).Once()
	mockConfig.On("GetInt", "session.lifetime", 120).Return(testSessionLifetime).Once()
	mockConfig.On("GetString", "session.cookie", "goravel_session").Return(testSessionCookie).Once()

	tempDriver, err := NewSession(context.Background(), mockConfig, testRedisConnection)
	s.Require().NoError(err)
	s.Require().NotNil(tempDriver)

	closeErr := tempDriver.Close()
	s.Nil(closeErr, "Close should not return an error")
}

func (s *SessionTestSuite) TestPrefix() {
	testID := "prefix_test_id"
	testData := "prefix_data"
	expectedKey := testPrefix + testID

	err := s.Session.Write(testID, testData)
	s.Require().Nil(err, "Write failed during prefix test")

	actualData, err := s.rawRedisClient.Get(context.Background(), expectedKey).Result()
	s.Require().NoError(err, "Raw client GET failed using expected prefix")
	s.Equal(testData, actualData, "Data retrieved with prefixed key does not match")

	readData, err := s.Session.Read(testID)
	s.Require().Nil(err)
	s.Equal(testData, readData)

	err = s.Session.Destroy(testID)
	s.Require().Nil(err)
	exists, _ := s.rawRedisClient.Exists(context.Background(), expectedKey).Result()
	s.EqualValues(0, exists, "Destroy did not remove prefixed key")
}
