package facades

import (
	"context"
	"testing"

	configmock "github.com/goravel/framework/mocks/config"
	foundationmock "github.com/goravel/framework/mocks/foundation"
	queuemock "github.com/goravel/framework/mocks/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	redisdriver "github.com/goravel/redis"
)

func TestMakingSessionWithRealImplementation(t *testing.T) {

	mockApp := foundationmock.NewApplication(t)
	mockConfig := configmock.NewConfig(t)

	mockConfig.On("GetString", "database.redis.default.host").Return("localhost").Once()
	mockConfig.On("GetString", "database.redis.default.port", "6379").Return("6379").Once()
	mockConfig.On("GetString", "database.redis.default.password").Return("").Once()
	mockConfig.On("GetString", "database.redis.default.username").Return("").Once()
	mockConfig.On("GetInt", "database.redis.default.database", 0).Return(0).Once()
	mockConfig.On("Get", "database.redis.default.tls").Return(nil).Once()

	mockConfig.On("GetInt", "session.lifetime", 120).Return(120).Once()
	mockConfig.On("GetString", "session.cookie", "goravel_session").Return("goravel_session_test_redis").Once()

	originalApp := redisdriver.App
	defer func() {
		redisdriver.App = originalApp
	}()
	redisdriver.App = mockApp

	realSessionsInstance, err := redisdriver.NewSession(context.Background(), mockConfig, "default")
	if err != nil {
		t.Skip("Skipping test as real driver creation failed:", err)
		return
	}

	mockApp.On("MakeWith", redisdriver.SessionBinding, map[string]any{"connection": "default"}).
		Return(realSessionsInstance, nil)

	driver, err := Session("default")
	require.NoError(t, err)

	require.NotNil(t, driver)
	assert.Equal(t, realSessionsInstance, driver)

	if err := realSessionsInstance.Close(); err != nil {
		t.Logf("Warning: Failed to close Redis driver: %v", err)
	}
}

func TestMakingCacheWithRealImplementation(t *testing.T) {

	mockApp := foundationmock.NewApplication(t)
	mockConfig := configmock.NewConfig(t)

	mockConfig.On("GetString", "cache.stores.redis.connection", "default").Return("default").Once()

	mockConfig.On("GetString", "database.redis.default.host").Return("localhost").Once()
	mockConfig.On("GetString", "database.redis.default.port").Return("6379").Once()
	mockConfig.On("GetString", "database.redis.default.username").Return("").Once()
	mockConfig.On("GetString", "database.redis.default.password").Return("").Once()
	mockConfig.On("GetInt", "database.redis.default.database").Return(0).Once()
	mockConfig.On("Get", "database.redis.default.tls").Return(nil).Once()

	mockConfig.On("GetString", "cache.prefix").Return("goravel_cache").Once()

	originalApp := redisdriver.App
	defer func() {
		redisdriver.App = originalApp
	}()
	redisdriver.App = mockApp

	realCacheInstance, err := redisdriver.NewCache(context.Background(), mockConfig, "redis")
	if err != nil {
		t.Skip("Skipping test as real driver creation failed:", err)
		return
	}

	mockApp.On("MakeWith", redisdriver.CacheBinding, map[string]any{"store": "redis"}).
		Return(realCacheInstance, nil)

	store, err := Cache("redis")
	require.NoError(t, err)

	require.NotNil(t, store)
	assert.Equal(t, realCacheInstance, store)
}

func TestMakingQueueWithRealImplementation(t *testing.T) {

	mockApp := foundationmock.NewApplication(t)
	mockConfig := configmock.NewConfig(t)

	mockConfig.On("GetString", "queue.connections.default.connection", "default").Return("default").Once()

	mockConfig.On("GetString", "database.redis.default.host").Return("localhost").Once()
	mockConfig.On("GetString", "database.redis.default.port").Return("6379").Once()
	mockConfig.On("GetString", "database.redis.default.username").Return("").Once()
	mockConfig.On("GetString", "database.redis.default.password").Return("").Once()
	mockConfig.On("GetInt", "database.redis.default.database").Return(0).Once()

	originalApp := redisdriver.App
	defer func() {
		redisdriver.App = originalApp
	}()
	redisdriver.App = mockApp

	queue := queuemock.NewQueue(t)

	realQueueInstance, err := redisdriver.NewQueue(context.Background(), mockConfig, queue, "default")
	if err != nil {
		t.Skip("Skipping test as real driver creation failed:", err)
		return
	}

	mockApp.On("MakeWith", redisdriver.QueueBinding, map[string]any{"connection": "default"}).
		Return(realQueueInstance, nil)

	store, err := Queue("default")
	require.NoError(t, err)

	require.NotNil(t, store)
	assert.Equal(t, realQueueInstance, store)
}
