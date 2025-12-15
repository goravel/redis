package redis

import (
	"context"
	"fmt"
	"testing"
	"time"

	mocksconfig "github.com/goravel/framework/mocks/config"
	"github.com/goravel/framework/support/env"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"
)

const (
	testSessionLifetime        = 2
	testSessionCookie          = "goravel_session_test_redis"
	testRedisDriverName        = "redis"
	testSessionRedisConnection = "session-default"
	testPrefix                 = testSessionCookie + ":"
	testSessionFiles           = ""
)

type SessionTestSuite struct {
	suite.Suite
	mockConfig *mocksconfig.Config

	client  redis.UniversalClient
	docker  *Docker
	session *Session
}

func TestSessionTestSuite(t *testing.T) {
	if env.IsWindows() {
		t.Skip("Skipping Redis session tests using Docker on Windows")
	}

	suite.Run(t, &SessionTestSuite{})
}

func (s *SessionTestSuite) SetupSuite() {
	mockConfig := mocksconfig.NewConfig(s.T())
	docker := initDocker(mockConfig)

	mockGetClient(mockConfig, docker)

	mockConfig.EXPECT().GetString(fmt.Sprintf("session.drivers.%s.connection", testConnection), "default").Return(testConnection).Once()
	mockConfig.EXPECT().GetInt("session.lifetime", 120).Return(testSessionLifetime).Once()
	mockConfig.EXPECT().GetString("session.cookie", "goravel_session").Return(testSessionCookie).Once()

	SessionDriver, err := NewSession(context.Background(), mockConfig, testConnection)
	s.Require().NoError(err, "NewRedis should succeed")
	s.Require().NotNil(SessionDriver, "NewRedis result should not be nil")

	client, err := docker.connect()
	s.Require().NoError(err)

	s.mockConfig = mockConfig
	s.client = client
	s.docker = docker
	s.session = SessionDriver
}

func (s *SessionTestSuite) TearDownSuite() {
	s.NoError(s.docker.Shutdown())
}

func (s *SessionTestSuite) SetupTest() {
	clients.Clear()
}

func (s *SessionTestSuite) TestWrite() {
	testID := "write_session_id"
	testData := "session_data_to_write"
	expectedKey := testPrefix + testID
	expectedTTL := time.Duration(s.session.lifetimeSeconds) * time.Second

	err := s.session.Write(testID, testData)
	s.Nil(err, "Write should not return an error")

	ctx := context.Background()
	actualData, err := s.client.Get(ctx, expectedKey).Result()
	s.Require().NoError(err, "Raw client failed to GET written key")
	s.Equal(testData, actualData, "Data written to Redis does not match")

	actualTTL, err := s.client.TTL(ctx, expectedKey).Result()
	s.Require().NoError(err, "Raw client failed to get TTL")
	s.Greater(actualTTL, time.Duration(0), "TTL should be positive")
	s.InDelta(expectedTTL, actualTTL, float64(3*time.Second), "TTL is not close to the configured lifetime")
}

func (s *SessionTestSuite) TestRead_Exists() {
	testID := "read_existing_session_id"
	testData := "existing_data"
	key := testPrefix + testID

	ttl := time.Duration(s.session.lifetimeSeconds+10) * time.Second
	errSetup := s.client.Set(context.Background(), key, testData, ttl).Err()
	s.Require().NoError(errSetup, "Setup failed: Raw client could not SET data")

	data, err := s.session.Read(testID)
	s.Nil(err, "Read should not return an error for existing key")
	s.Equal(testData, data, "Read returned incorrect data")
}

func (s *SessionTestSuite) TestRead_NotExist() {
	testID := "read_non_existent_session_id"

	exists, errExists := s.client.Exists(context.Background(), testPrefix+testID).Result()
	s.Require().NoError(errExists)
	s.Require().EqualValues(0, exists, "Test setup failed: Key should not exist")

	data, err := s.session.Read(testID)
	s.Nil(err, "Read should not return an error for non-existent key")
	s.Equal("", data, "Read should return empty string for non-existent key")
}

func (s *SessionTestSuite) TestRead_Expired() {
	testID := "read_expired_session_id"
	testData := "expired_data"
	key := testPrefix + testID
	shortTTL := 1 * time.Second

	errSetup := s.client.Set(context.Background(), key, testData, shortTTL).Err()
	s.Require().NoError(errSetup, "Setup failed: Raw client could not SET data")

	time.Sleep(shortTTL + 500*time.Millisecond)

	data, err := s.session.Read(testID)
	s.Nil(err, "Read should not return an error for expired key")
	s.Equal("", data, "Read should return empty string for expired key")
}

func (s *SessionTestSuite) TestDestroy_Exists() {
	testID := "destroy_existing_session_id"
	testData := "data_to_destroy"
	key := testPrefix + testID

	errSetup := s.session.Write(testID, testData)
	s.Require().NoError(errSetup, "Setup failed: Could not write data to destroy")

	dataBefore, errReadBefore := s.session.Read(testID)
	s.Require().Nil(errReadBefore)
	s.Require().Equal(testData, dataBefore)

	err := s.session.Destroy(testID)
	s.Nil(err, "Destroy should not return an error")

	exists, errExists := s.client.Exists(context.Background(), key).Result()
	s.Require().NoError(errExists)
	s.EqualValues(0, exists, "Key should not exist in Redis after Destroy")

	dataAfter, errReadAfter := s.session.Read(testID)
	s.Nil(errReadAfter)
	s.Equal("", dataAfter, "Read should return empty after Destroy")
}

func (s *SessionTestSuite) TestDestroy_NotExist() {
	testID := "destroy_non_existent_session_id"
	key := testPrefix + testID

	exists, errExists := s.client.Exists(context.Background(), key).Result()
	s.Require().NoError(errExists)
	s.Require().EqualValues(0, exists, "Test setup failed: Key should not exist before destroy")

	err := s.session.Destroy(testID)
	s.Nil(err, "Destroy should not return an error for non-existent key")
}

func (s *SessionTestSuite) TestGc() {
	err := s.session.Gc(300)
	s.Nil(err, "Gc should be a no-op and return nil error")
}

func (s *SessionTestSuite) TestOpen() {
	err := s.session.Open("", "")
	s.Nil(err, "Open should be a no-op and return nil error")
}

func (s *SessionTestSuite) TestPrefix() {
	testID := "prefix_test_id"
	testData := "prefix_data"
	expectedKey := testPrefix + testID

	err := s.session.Write(testID, testData)
	s.Require().Nil(err, "Write failed during prefix test")

	actualData, err := s.client.Get(context.Background(), expectedKey).Result()
	s.Require().NoError(err, "Raw client GET failed using expected prefix")
	s.Equal(testData, actualData, "Data retrieved with prefixed key does not match")

	readData, err := s.session.Read(testID)
	s.Require().Nil(err)
	s.Equal(testData, readData)

	err = s.session.Destroy(testID)
	s.Require().Nil(err)
	exists, _ := s.client.Exists(context.Background(), expectedKey).Result()
	s.EqualValues(0, exists, "Destroy did not remove prefixed key")
}
